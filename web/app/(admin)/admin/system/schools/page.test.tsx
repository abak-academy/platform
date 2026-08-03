import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
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

let schoolsState = {
  data: null as { data: School[]; next_cursor?: string } | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-schools", () => ({
  useAdminSchools: () => schoolsState,
  useCreateSchool: () => createState,
  useUpdateSchool: () => updateState,
  useChangeSchoolStatus: () => changeStatusState,
  adminSchoolsKeys: { all: ["admin", "schools"] },
}));

// SchoolBulkImportModal is always mounted (Dialog just stays closed) — its
// hooks need mocking here too, same as BulkImportModal on the students page.
vi.mock("@/lib/hooks/admin-schools-bulk", () => ({
  usePresignSchoolBulkUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
  putFileToPresignedURL: vi.fn(),
  useEnqueueSchoolBulkImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/hooks/jobs", () => ({
  useJobStatus: () => ({ data: null, isLoading: false, isError: false, error: null }),
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

const paginatedResponse = (schools: School[]) => ({
  data: schools,
  next_cursor: undefined,
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
      data: { data: sampleSchools, next_cursor: "cursor-next" },
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
});
