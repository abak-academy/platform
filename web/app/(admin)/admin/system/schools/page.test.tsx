import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within, fireEvent, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import SystemSchoolsPage from "./page";
import type { School } from "@/lib/types";

function renderPage(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

interface SchoolsListResponse {
  data: School[];
  next_cursor?: string;
  total: number;
  active: number;
  students: number;
}

let schoolsState = {
  data: null as SchoolsListResponse | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

// Records every call so tests can assert q/status/cursor are actually sent
// to the server, not just filtered client-side.
const useAdminSchoolsCalls: unknown[] = [];

vi.mock("@/lib/hooks/admin-schools", () => ({
  useAdminSchools: (params: unknown) => {
    useAdminSchoolsCalls.push(params);
    return schoolsState;
  },
  useCreateSchool: () => createState,
  useUpdateSchool: () => updateState,
  useChangeSchoolStatus: () => changeStatusState,
  adminSchoolsKeys: { all: ["admin", "schools"] },
}));

// SchoolBulkImportModal is always mounted (Dialog just stays closed) — its
// hooks need mocking here too, same as BulkImportModal on the students page.
const bulkPresignMutateAsync = vi.fn();
const bulkEnqueueMutateAsync = vi.fn();

vi.mock("@/lib/hooks/admin-schools-bulk", () => ({
  usePresignSchoolBulkUpload: () => ({ mutateAsync: bulkPresignMutateAsync, isPending: false }),
  putFileToPresignedURL: vi.fn(),
  useEnqueueSchoolBulkImport: () => ({ mutateAsync: bulkEnqueueMutateAsync, isPending: false }),
}));

// Mutable like SchoolBulkImportModal.test.tsx, so a test can drive a job from
// enqueued to "succeeded" and assert the page resets its own pagination
// state (root cause #3 in docs/backlog/school-bulk-list-pagination.md) —
// not just that React Query was invalidated.
let jobStatusState: {
  data: { id: string; type: string; status: string; progress: number; result_url: string | null; error: string | null; created_at: string; updated_at: string } | null;
} = { data: null };

vi.mock("@/lib/hooks/jobs", () => ({
  useJobStatus: (jobId: string | null) =>
    jobId ? jobStatusState : { data: null, isLoading: false, isError: false, error: null },
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleSchools: School[] = [
  {
    id: "s1",
    name: "SMAN 1 Jakarta",
    code: "SMAN1JKT",
    npsn: "12345678",
    school_types: ["Negeri"],
    alamat: "Jl. Merdeka No.1",
    status: "active",
    student_count: 500,
  },
  {
    id: "s2",
    name: "SMAN 2 Jakarta",
    code: "SMAN2JKT",
    npsn: "87654321",
    school_types: ["Negeri", "SMA"],
    alamat: "Jl. Sudirman No.5",
    status: "deactivated",
  },
];

// total/active/students mirror what CountSchoolsAdmin would return for the
// full filtered set — deliberately not derived from schools.length, since
// that was exactly the "Total ≈ 20" bug (client only ever saw loaded rows).
const paginatedResponse = (schools: School[], next_cursor?: string): SchoolsListResponse => ({
  data: schools,
  next_cursor,
  total: schools.length,
  active: schools.filter((s) => s.status === "active").length,
  students: schools.reduce((sum, s) => sum + (s.student_count ?? 0), 0),
});

describe("SystemSchoolsPage", () => {
  beforeEach(() => {
    schoolsState = {
      data: paginatedResponse(sampleSchools),
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
    useAdminSchoolsCalls.length = 0;
    bulkPresignMutateAsync.mockReset();
    bulkEnqueueMutateAsync.mockReset();
    jobStatusState = { data: null };
  });

  // A prior test throwing before vi.useRealTimers() would otherwise leave
  // fake timers on and hang every later waitFor() in this file.
  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders loading state when data is loading and no schools exist", async () => {
    schoolsState = {
      data: null,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("Memuat…")).toBeInTheDocument();
    });
  });

  it("renders error state when error and no schools exist", async () => {
    schoolsState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat data/i)).toBeInTheDocument();
    });
  });

  it("renders the schools table with school data", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument();
      expect(screen.getByText("SMAN 2 Jakarta")).toBeInTheDocument();
    });

    expect(screen.getByText("SMAN1JKT")).toBeInTheDocument();
    expect(screen.getByText("12345678")).toBeInTheDocument();
    expect(screen.getByText("87654321")).toBeInTheDocument();
    expect(screen.getAllByText("Aktif").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Nonaktif").length).toBeGreaterThanOrEqual(1);
  });

  it("shows stat cards with total, active, and student counts", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("2")).toBeInTheDocument();
    });
    // 500 appears both in stat card and student count column
    expect(screen.getAllByText("500").length).toBeGreaterThanOrEqual(1);
  });

  it("renders filter chips", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: /^semua$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^aktif$/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^nonaktif$/i })).toBeInTheDocument();
  });

  it("opens create dialog and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({
      id: "s3",
      name: "SMAN 3 Jakarta",
      code: "SMAN3JKT",
    });

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));

    expect(screen.getByText("Buat sekolah")).toBeInTheDocument();

    const nameInput = screen.getByPlaceholderText("mis. SMAN 1 Jakarta");
    fireEvent.input(nameInput, { target: { value: "SMAN 3 Jakarta" } });

    const codeInput = screen.getByPlaceholderText("Kode Sekolah");
    fireEvent.input(codeInput, { target: { value: "SMAN3JKT" } });

    const saveButton = screen.getByRole("button", { name: /^buat$/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ name: "SMAN 3 Jakarta", code: "SMAN3JKT" }),
      );
      expect(toast.success).toHaveBeenCalledWith("Perubahan disimpan.");
    });
  });

  it("validates required fields before create", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));

    const saveButton = screen.getByRole("button", { name: /^buat$/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Semua field harus diisi");
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("surfaces an API error as error toast on create failure", async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error("gagal simpan"));

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));

    const nameInput = screen.getByPlaceholderText("mis. SMAN 1 Jakarta");
    fireEvent.input(nameInput, { target: { value: "SMAN Gagal" } });

    const codeInput = screen.getByPlaceholderText("Kode Sekolah");
    fireEvent.input(codeInput, { target: { value: "GAGAL" } });

    fireEvent.click(screen.getByRole("button", { name: /^buat$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("gagal simpan");
    });
  });

  it("renders load more button when next_cursor exists", async () => {
    schoolsState = {
      data: paginatedResponse(sampleSchools, "cursor-next"),
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => {
      expect(screen.getByText("Muat lebih banyak")).toBeInTheDocument();
    });
  });

  it("opens edit dialog and calls update mutation with only changed fields", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "s1", name: "SMAN 1 Jakarta Baru" });

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const s1Row = rows.find((r) => within(r).queryByText("SMAN 1 Jakarta"));
    expect(s1Row).toBeTruthy();
    fireEvent.pointerDown(
      within(s1Row as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    fireEvent.click(await screen.findByText("Edit"));

    const dialog = await screen.findByRole("dialog");
    const nameInput = within(dialog).getByDisplayValue("SMAN 1 Jakarta");
    fireEvent.input(nameInput, { target: { value: "SMAN 1 Jakarta Baru" } });

    fireEvent.click(within(dialog).getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "s1", name: "SMAN 1 Jakarta Baru" });
      expect(toast.success).toHaveBeenCalledWith("Perubahan disimpan.");
    });
  });

  it("toggles a school's status from the row menu", async () => {
    mockMutateAsync.mockResolvedValueOnce({ status: "deactivated" });

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const s1Row = rows.find((r) => within(r).queryByText("SMAN 1 Jakarta"));
    expect(s1Row).toBeTruthy();
    fireEvent.pointerDown(
      within(s1Row as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    fireEvent.click(await screen.findByText("Nonaktifkan"));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "s1", status: "deactivated" });
      expect(toast.success).toHaveBeenCalledWith("Nonaktifkan berhasil");
    });
  });

  it("always keeps the school code input editable (Task 32 removed the student-count lock)", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const s1Row = rows.find((r) => within(r).queryByText("SMAN 1 Jakarta"));
    const s2Row = rows.find((r) => within(r).queryByText("SMAN 2 Jakarta"));
    expect(s1Row).toBeTruthy();
    expect(s2Row).toBeTruthy();

    fireEvent.pointerDown(
      within(s1Row as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );
    fireEvent.click(await screen.findByText("Edit"));

    let dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByDisplayValue("SMAN1JKT")).not.toBeDisabled();
    expect(screen.queryByText(/Kode tidak dapat diubah/)).not.toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: /^batal$/i }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

    fireEvent.pointerDown(
      within(s2Row as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );
    fireEvent.click(await screen.findByText("Edit"));

    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByDisplayValue("SMAN2JKT")).not.toBeDisabled();
  });

  it("sends the status filter to the server instead of filtering loaded rows client-side", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^aktif$/i }));

    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { status?: string } | undefined;
      expect(last?.status).toBe("active");
    });
  });

  it("debounces search input before sending it as the q param", async () => {
    vi.useFakeTimers();
    renderPage(<SystemSchoolsPage />);

    const search = screen.getByPlaceholderText(/cari sekolah|search school/i);
    fireEvent.change(search, { target: { value: "jakarta" } });

    // Not yet — a keystroke must not immediately trigger a new query.
    let last = useAdminSchoolsCalls.at(-1) as { q?: string } | undefined;
    expect(last?.q).toBeUndefined();

    act(() => {
      vi.advanceTimersByTime(350);
    });

    last = useAdminSchoolsCalls.at(-1) as { q?: string } | undefined;
    expect(last?.q).toBe("jakarta");
  });

  it("resets the cursor to page 1 when the status filter changes after loading more", async () => {
    schoolsState = {
      data: paginatedResponse(sampleSchools, "cursor-next"),
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("Muat lebih banyak")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Muat lebih banyak"));

    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { cursor?: string } | undefined;
      expect(last?.cursor).toBe("cursor-next");
    });

    fireEvent.click(screen.getByRole("button", { name: /^aktif$/i }));

    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { cursor?: string; status?: string } | undefined;
      expect(last?.status).toBe("active");
      expect(last?.cursor).toBeUndefined();
    });
  });

  it("resets pagination back to page 1 when a bulk import succeeds, even after loading more", async () => {
    // Land the page on a "load more"'d page 2 first.
    schoolsState = {
      data: paginatedResponse(sampleSchools, "cursor-next"),
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { rerender } = render(
      <QueryClientProvider client={qc}>
        <SystemSchoolsPage />
      </QueryClientProvider>,
    );
    const rerenderPage = () =>
      rerender(
        <QueryClientProvider client={qc}>
          <SystemSchoolsPage />
        </QueryClientProvider>,
      );

    await waitFor(() => expect(screen.getByText("Muat lebih banyak")).toBeInTheDocument());
    fireEvent.click(screen.getByText("Muat lebih banyak"));
    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { cursor?: string } | undefined;
      expect(last?.cursor).toBe("cursor-next");
    });

    // Open the bulk import dialog and drive a job to success.
    bulkPresignMutateAsync.mockResolvedValueOnce({
      url: "http://minio.local/k?sig=xyz",
      method: "PUT",
      key: "school-bulk/k1.csv",
    });
    bulkEnqueueMutateAsync.mockResolvedValueOnce({ job_id: "job-1" });

    fireEvent.click(screen.getByRole("button", { name: /bulk_school_import_button|impor massal/i }));
    const fileInput = await screen.findByLabelText(/choose_file|file|pilih file/i);
    fireEvent.change(fileInput as HTMLInputElement, {
      target: { files: [new File(["x"], "schools.csv", { type: "text/csv" })] },
    });
    fireEvent.click(screen.getByRole("button", { name: /upload|import|submit|start|unggah/i }));

    await waitFor(() => expect(bulkEnqueueMutateAsync).toHaveBeenCalled());

    jobStatusState = {
      data: {
        id: "job-1",
        type: "school_bulk",
        status: "succeeded",
        progress: 100,
        result_url: null,
        error: null,
        created_at: "2026-08-19T00:00:00Z",
        updated_at: "2026-08-19T00:01:00Z",
      },
    };
    rerenderPage();

    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { cursor?: string } | undefined;
      expect(last?.cursor).toBeUndefined();
    });
  });
});
