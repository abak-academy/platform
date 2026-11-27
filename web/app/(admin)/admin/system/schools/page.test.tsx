import { describe, it, expect, vi, beforeEach, afterEach, beforeAll } from "vitest";
import userEvent from "@testing-library/user-event";
import { render, screen, waitFor, within, fireEvent, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { toast } from "sonner";
import SystemSchoolsPage from "./page";
import type { School } from "@/lib/types";

beforeAll(() => {
  globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

async function pick(field: string, name: string) {
  await userEvent.click(screen.getByRole("combobox", { name: field }));
  await userEvent.click(await screen.findByRole("option", { name }));
}

function renderPage(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

function schoolRecord(name: string) {
  const record = screen.getByText(name).closest("tr");
  expect(record).toBeTruthy();
  return record as HTMLElement;
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

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({
    data: [
      { id: "31", name: "DKI JAKARTA" },
      { id: "32", name: "JAWA BARAT" },
    ],
  }),
  useCitiesByProvince: (provinceId?: string) => ({
    data:
      provinceId === "31"
        ? [{ id: "3171", province_id: "31", name: "KOTA JAKARTA SELATAN" }]
        : provinceId === "32"
          ? [{ id: "3273", province_id: "32", name: "KOTA BANDUNG" }]
          : [],
  }),
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
    provinsi_id: "31",
    kota_id: "3171",
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
    provinsi_id: "31",
    kota_id: "3171",
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

  it("renders the list as one flush card with a toolbar, no detail side panel", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    const list = screen.getByRole("region", { name: "Daftar sekolah" });
    expect(list).toHaveClass("md-card-flush");
    expect(within(list).getByTestId("schools-table")).toBeInTheDocument();
    expect(screen.queryByRole("complementary", { name: "Detail sekolah" })).toBeNull();
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
  });

  it("gives the truncated address a title and shows it in full in the edit dialog", async () => {
    renderPage(<SystemSchoolsPage />);

    const address = await screen.findByTitle("Jl. Merdeka No.1");
    fireEvent.click(address);
    expect(await screen.findByDisplayValue("Jl. Merdeka No.1")).toBeInTheDocument();
  });

  it("opens the edit dialog when a row is clicked", async () => {
    renderPage(<SystemSchoolsPage />);

    fireEvent.click(await screen.findByText("SMAN 2 Jakarta"));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByDisplayValue("SMAN 2 Jakarta")).toBeInTheDocument();
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
      expect(screen.getByText("Memuat data…")).toBeInTheDocument();
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

  it("shows total, active, and student counts", async () => {
    renderPage(<SystemSchoolsPage />);

    expect((await screen.findAllByText(/2 sekolah/)).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/500/).length).toBeGreaterThanOrEqual(1);
  });

  it("puts Status and Provinsi dropdowns in the Filter popover instead of chips", async () => {
    renderPage(<SystemSchoolsPage />);
    await screen.findByText("SMAN 1 Jakarta");

    expect(screen.queryByRole("button", { name: /^aktif$/i })).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    expect(screen.getByRole("combobox", { name: "Status" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Provinsi" })).toBeInTheDocument();
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

    fireEvent.change(screen.getByTestId("create-school-province"), {
      target: { value: "31" },
    });
    fireEvent.change(screen.getByTestId("create-school-city"), {
      target: { value: "3171" },
    });
    fireEvent.change(screen.getByPlaceholderText("Jenis Sekolah"), {
      target: { value: "SMA" },
    });

    const saveButton = screen.getByRole("button", { name: /^buat$/i });
    fireEvent.click(saveButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "SMAN 3 Jakarta",
          code: "SMAN3JKT",
          school_types: ["SMA"],
          provinsi_id: "31",
          kota_id: "3171",
        }),
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

    const s1Row = schoolRecord("SMAN 1 Jakarta");
    fireEvent.pointerDown(
      within(s1Row).getByRole("button", { name: "" }),
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

  it("updates school location and school types", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "s1" });
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());
    const row = schoolRecord("SMAN 1 Jakarta");
    fireEvent.pointerDown(within(row).getByRole("button", { name: "" }), { button: 0 });
    fireEvent.click(await screen.findByText("Edit"));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByTestId("edit-school-province"), {
      target: { value: "32" },
    });
    fireEvent.change(within(dialog).getByTestId("edit-school-city"), {
      target: { value: "3273" },
    });
    fireEvent.change(within(dialog).getByPlaceholderText("Jenis Sekolah"), {
      target: { value: "SMK" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        id: "s1",
        school_types: ["SMK"],
        provinsi_id: "32",
        kota_id: "3273",
      });
    });
  });

  it("sends an explicit blank NPSN when the admin clears it", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "s1", npsn: null });
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());
    const row = schoolRecord("SMAN 1 Jakarta");
    fireEvent.pointerDown(within(row).getByRole("button", { name: "" }), { button: 0 });
    fireEvent.click(await screen.findByText("Edit"));

    const dialog = await screen.findByRole("dialog");
    fireEvent.input(within(dialog).getByDisplayValue("12345678"), { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "s1", npsn: "" });
    });
  });

  it("shows the NPSN format hint and limits create and edit inputs", async () => {
    renderPage(<SystemSchoolsPage />);
    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));
    let dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByPlaceholderText("8 karakter, mis. 20100001")).toHaveAttribute("maxlength", "8");
    fireEvent.click(within(dialog).getByRole("button", { name: /^batal$/i }));

    const row = schoolRecord("SMAN 1 Jakarta");
    fireEvent.pointerDown(within(row).getByRole("button", { name: "" }), { button: 0 });
    fireEvent.click(await screen.findByText("Edit"));
    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByPlaceholderText("8 karakter, mis. 20100001")).toHaveAttribute("maxlength", "8");
  });

  it("toggles a school's status from the row menu", async () => {
    mockMutateAsync.mockResolvedValueOnce({ status: "deactivated" });

    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    const s1Row = schoolRecord("SMAN 1 Jakarta");
    fireEvent.pointerDown(
      within(s1Row).getByRole("button", { name: "" }),
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

    const s1Row = schoolRecord("SMAN 1 Jakarta");
    const s2Row = schoolRecord("SMAN 2 Jakarta");

    fireEvent.pointerDown(
      within(s1Row).getByRole("button", { name: "" }),
      { button: 0 }
    );
    fireEvent.click(await screen.findByText("Edit"));

    let dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByDisplayValue("SMAN1JKT")).not.toBeDisabled();
    expect(screen.queryByText(/Kode tidak dapat diubah/)).not.toBeInTheDocument();

    fireEvent.click(within(dialog).getByRole("button", { name: /^batal$/i }));

    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());

    fireEvent.pointerDown(
      within(s2Row).getByRole("button", { name: "" }),
      { button: 0 }
    );
    fireEvent.click(await screen.findByText("Edit"));

    dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByDisplayValue("SMAN2JKT")).not.toBeDisabled();
  });

  it("sends the status filter to the server instead of filtering loaded rows client-side", async () => {
    renderPage(<SystemSchoolsPage />);

    await waitFor(() => expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument());

    await userEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    await pick("Status", "Aktif");

    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { status?: string } | undefined;
      expect(last?.status).toBe("active");
    });
    expect(within(screen.getByRole("group", { name: "Filter aktif" })).getByText("Status: Aktif")).toBeInTheDocument();

    await pick("Status", "Semua");
    await waitFor(() => {
      const last = useAdminSchoolsCalls.at(-1) as { status?: string } | undefined;
      expect(last?.status).toBeUndefined();
    });
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
  });

  it("filters the loaded rows by Provinsi and shows a chip", async () => {
    schoolsState = {
      ...schoolsState,
      data: paginatedResponse([...sampleSchools, { id: "s3", name: "SMAN 3 Bandung", provinsi_id: "32", status: "active" }]),
    };
    renderPage(<SystemSchoolsPage />);
    await screen.findByText("SMAN 3 Bandung");

    await userEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    await pick("Provinsi", "JAWA BARAT");

    expect(screen.getByText("SMAN 3 Bandung")).toBeInTheDocument();
    expect(screen.queryByText("SMAN 1 Jakarta")).not.toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "Filter aktif" })).getByText("Provinsi: JAWA BARAT")).toBeInTheDocument();

    await pick("Provinsi", "Semua");
    expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument();
  });

  it("sorts the loaded rows by most students", async () => {
    schoolsState = {
      ...schoolsState,
      data: paginatedResponse([
        { id: "a", name: "Kecil", status: "active", student_count: 1 },
        { id: "b", name: "Besar", status: "active", student_count: 99 },
      ]),
    };
    renderPage(<SystemSchoolsPage />);
    await screen.findByText("Kecil");

    await userEvent.click(screen.getByRole("combobox", { name: "Urutkan" }));
    await userEvent.click(await screen.findByRole("option", { name: "Siswa terbanyak" }));

    const names = screen.getAllByRole("row").slice(1).map((r) => r.textContent ?? "");
    expect(names[0]).toContain("Besar");
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

  // Regression: the loading state used to early-return a full-page screen,
  // unmounting the search input mid-search and dropping its focus — the user
  // had to click the field again after every result refresh. Loading now
  // renders inside the table while the toolbar (and the focused input) stay
  // mounted, same shape as ParticipantPicker.
  it("keeps the search input mounted and focused while a search reloads the list", () => {
    vi.useFakeTimers();
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

    const search = screen.getByPlaceholderText(/cari sekolah|search school/i);
    search.focus();

    // Type, then simulate the fresh (uncached) query key going pending while
    // the debounce fires — the exact moment the old early return unmounted it.
    fireEvent.change(search, { target: { value: "jakarta" } });
    act(() => {
      schoolsState = {
        data: null,
        isLoading: true,
        isError: false,
        error: null,
        refetch: vi.fn(),
      };
    });
    act(() => {
      vi.advanceTimersByTime(350);
    });

    expect(screen.getByText("Memuat data…")).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/cari sekolah|search school/i)).toBe(search);
    expect(search).toHaveFocus();

    // Results arrive: same input node, still focused.
    act(() => {
      schoolsState = {
        data: paginatedResponse(sampleSchools),
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn(),
      };
    });
    rerenderPage();

    expect(screen.getByText("SMAN 1 Jakarta")).toBeInTheDocument();
    expect(screen.getByPlaceholderText(/cari sekolah|search school/i)).toBe(search);
    expect(search).toHaveFocus();
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

    await userEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    await pick("Status", "Aktif");

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

  describe("loaded-rows-only province filter and sort", () => {
    it("shows a '+' loaded count, a loaded-only empty state with load more, and locks sort when more pages exist", async () => {
      schoolsState = { ...schoolsState, data: { ...paginatedResponse(sampleSchools, "c2"), total: 80 } };
      renderPage(<SystemSchoolsPage />);
      await screen.findByText("SMAN 1 Jakarta");
      expect(screen.getByText("80 sekolah")).toBeInTheDocument();

      await userEvent.click(screen.getByRole("button", { name: /^Filter/ }));
      await pick("Provinsi", "JAWA BARAT");

      expect(screen.getByText("0+ sekolah")).toBeInTheDocument();
      expect(screen.getByText(/Tidak ada yang cocok di data yang sudah dimuat/)).toBeInTheDocument();
      expect(screen.getAllByRole("button", { name: "Muat lebih banyak" }).length).toBeGreaterThanOrEqual(1);
      expect(screen.getByText(/Urutan lain aktif setelah semua data dimuat/)).toBeInTheDocument();

      await userEvent.keyboard("{Escape}");
      await userEvent.click(screen.getByRole("combobox", { name: "Urutkan" }));
      expect(await screen.findByRole("option", { name: "Nama A–Z" })).toHaveAttribute("aria-disabled", "true");
    });

    it("keeps the server total and an enabled sort when everything is loaded", async () => {
      renderPage(<SystemSchoolsPage />);
      await screen.findByText("SMAN 1 Jakarta");
      expect(screen.getByText("2 sekolah")).toBeInTheDocument();
      expect(screen.queryByText(/Urutan lain aktif/)).not.toBeInTheDocument();

      await userEvent.click(screen.getByRole("combobox", { name: "Urutkan" }));
      expect(await screen.findByRole("option", { name: "Nama A–Z" })).not.toHaveAttribute("aria-disabled", "true");
    });
  });
});
