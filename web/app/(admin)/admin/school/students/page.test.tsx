import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import SchoolStudentsPage from "./page";
import type { AdminStudent, StudentRegistrationResult, StudentCredentials, School } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let studentsState = {
  data: null as { data: AdminStudent[]; next_cursor?: string } | null,
  isLoading: true,
  isFetching: false,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let registerState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let reissueState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

// Auth store mock
let authStore: {
  token: string | null;
  user: { role?: string; name?: string; school_id?: string } | null;
} = {
  token: "t",
  user: { role: "admin_school", school_id: "s1" },
};

// Schools mock
let schoolsState = {
  data: null as { data: School[]; next_cursor?: string } | null,
  isLoading: false,
  isError: false,
};

vi.mock("@/lib/hooks/admin-students", () => ({
  useAdminStudents: () => studentsState,
  useRegisterStudent: () => registerState,
  useChangeStudentStatus: () => changeStatusState,
  useReissueStudentCredentials: () => reissueState,
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

vi.mock("@/lib/hooks/admin-schools", () => ({
  useSchoolOptions: () => schoolsState,
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchools: () => ({
    data: [{ id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA", "SMK"] }],
    isLoading: false,
  }),
}));

vi.mock("@/lib/hooks/regions", () => ({
  useProvinces: () => ({ data: [], isLoading: false }),
  useCitiesByProvince: () => ({ data: [], isLoading: false }),
  useDistrictsByCity: () => ({ data: [], isLoading: false }),
}));

// BulkImportModal is always mounted (Dialog just stays closed) — its hooks
// need mocking here too, same as every other data hook on this page.
vi.mock("@/lib/hooks/admin-students-bulk", () => ({
  usePresignStudentBulkUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
  putFileToPresignedURL: vi.fn(),
  useEnqueueStudentBulkImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
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

const sampleStudents: AdminStudent[] = [
  {
    id: "st1",
    name: "Budi Santoso",
    username: "budi",
    jenjang: "SMA",
    email: "budi@test.com",
    status: "active",
    grade: 12,
    created_at: "2026-01-15T00:00:00Z",
  },
  {
    id: "st2",
    name: "Siti Aisyah",
    username: "siti",
    jenjang: "SMK",
    status: "deactivated",
    grade: 11,
    created_at: "2026-02-20T00:00:00Z",
  },
];

const paginatedResponse = (students: AdminStudent[]) => ({
  data: students,
  next_cursor: undefined,
});

describe("SchoolStudentsPage", () => {
  beforeEach(() => {
    authStore = {
      token: "t",
      user: { role: "admin_school", school_id: "s1" },
    };
    schoolsState = {
      data: { data: [{ id: "s1", name: "SMAN 1 Jakarta" }, { id: "s2", name: "SMAN 2 Bandung" }], next_cursor: undefined },
      isLoading: false,
      isError: false,
    };
    studentsState = {
      data: paginatedResponse(sampleStudents),
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    registerState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    reissueState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders loading state when loading and no accumulated data", async () => {
    studentsState = {
      data: null,
      isLoading: true,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Memuat…")).toBeInTheDocument();
    });
  });

  it("renders error state when error and no accumulated data", async () => {
    studentsState = {
      data: null,
      isLoading: false,
      isFetching: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat data/i)).toBeInTheDocument();
    });
  });

  it("renders the students table with student data", async () => {
    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
      expect(screen.getByText("Siti Aisyah")).toBeInTheDocument();
    });

    expect(screen.getByText(/@budi/)).toBeInTheDocument();
    expect(screen.getByText(/@siti/)).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("11")).toBeInTheDocument();
    expect(screen.getAllByText("Aktif").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Nonaktif").length).toBeGreaterThanOrEqual(1);
  });

  it("shows stat cards with total, active, and deactivated counts", async () => {
    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("2")).toBeInTheDocument();
    });
    expect(screen.getAllByText("1").length).toBeGreaterThanOrEqual(1);
  });

  it("opens the bulk import modal from the header button", async () => {
    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /pendaftaran siswa massal/i }));

    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(
      within(screen.getByRole("dialog")).getByText("Unduh Template"),
    ).toBeInTheDocument();
  });

  it("opens register dialog and shows credential panel on success", async () => {
    const registrationResult: StudentRegistrationResult = {
      id: "st3",
      name: "Dewi Lestari",
      username: "dewi",
      jenjang: "SMA",
      status: "active",
      created_at: "2026-03-01T00:00:00Z",
      temp_password: "tempPass321",
    };
    mockMutateAsync.mockResolvedValueOnce(registrationResult);

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));

    // The dialog title and submit button both say "Daftarkan Siswa"
    expect(screen.getAllByText("Daftarkan Siswa").length).toBeGreaterThanOrEqual(1);

    const nameInput = screen.getByPlaceholderText("Nama Lengkap");
    fireEvent.input(nameInput, { target: { value: "Dewi Lestari" } });

    // Select Jenjang from the required select (scoped to the dialog; Radix Select
    // triggers don't expose an accessible name in jsdom, so target by order —
    // Gender is the only combobox before Jenjang for a non-super_admin role).
    const dialogForJenjang = screen.getByRole("dialog");
    const jenjangTrigger = within(dialogForJenjang).getAllByRole("combobox")[1];
    fireEvent.click(jenjangTrigger);
    const smaOption = await screen.findByRole("option", { name: "SMA" });
    fireEvent.click(smaOption);

    // Click the submit button inside the dialog
    const dialog = screen.getByRole("dialog");
    const submitBtn = within(dialog).getByRole("button", { name: /daftarkan siswa/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({ name: "Dewi Lestari", jenjang: "SMA" }),
        }),
      );
      expect(toast.success).toHaveBeenCalledWith("Siswa berhasil didaftarkan.");
    });

    // Credential panel should appear
    await waitFor(() => {
      expect(screen.getByText("Kredensial Siswa")).toBeInTheDocument();
      expect(screen.getByText("dewi")).toBeInTheDocument();
      expect(screen.getByText("tempPass321")).toBeInTheDocument();
    });
  });

  it("validates required fields before register", async () => {
    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));

    // Click submit without filling required fields
    const dialog = screen.getByRole("dialog");
    const submitBtn = within(dialog).getByRole("button", { name: /daftarkan siswa/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Semua field harus diisi");
    });

    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("renders load more button when next_cursor exists", async () => {
    studentsState = {
      data: { data: sampleStudents, next_cursor: "cursor-next" },
      isLoading: false,
      isFetching: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Muat lebih")).toBeInTheDocument();
    });
  });

  it("opens reissue-credentials flow from the row menu and shows new credentials", async () => {
    const credentials: StudentCredentials = {
      username: "budi",
      temp_password: "newTempPass999",
    };
    mockMutateAsync.mockResolvedValueOnce(credentials);

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const budiRow = rows.find((r) => within(r).queryByText("Budi Santoso"));
    expect(budiRow).toBeTruthy();
    fireEvent.pointerDown(
      within(budiRow as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    const reissueItem = await screen.findByText("Terbitkan Ulang Kredensial");
    fireEvent.click(reissueItem);

    const dialog = await screen.findByRole("dialog");
    const confirmBtn = within(dialog).getByRole("button", { name: "Terbitkan Ulang Kredensial" });
    fireEvent.click(confirmBtn);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ id: "st1" }),
      );
      expect(toast.success).toHaveBeenCalledWith("Kredensial baru berhasil diterbitkan.");
    });

    await waitFor(() => {
      expect(screen.getByText("newTempPass999")).toBeInTheDocument();
    });
  });

  it("toggles a student's status from the row menu", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "status updated" });

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const budiRow = rows.find((r) => within(r).queryByText("Budi Santoso"));
    expect(budiRow).toBeTruthy();
    fireEvent.pointerDown(
      within(budiRow as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    const toggleItem = await screen.findByText("Nonaktifkan siswa");
    fireEvent.click(toggleItem);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "st1", status: "deactivated" });
      expect(toast.success).toHaveBeenCalledWith("Siswa dinonaktifkan.");
    });
  });

  it("surfaces an API error as error toast on register failure", async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error("gagal register"));

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));

    const nameInput = screen.getByPlaceholderText("Nama Lengkap");
    fireEvent.input(nameInput, { target: { value: "Gagal Student" } });

    // Select Jenjang (scoped to the dialog; Gender is the only combobox before it)
    const dialogForJenjang2 = screen.getByRole("dialog");
    const jenjangTrigger = within(dialogForJenjang2).getAllByRole("combobox")[1];
    fireEvent.click(jenjangTrigger);
    const smaOption = await screen.findByRole("option", { name: "SMA" });
    fireEvent.click(smaOption);

    const dialog = screen.getByRole("dialog");
    const submitBtn = within(dialog).getByRole("button", { name: /daftarkan siswa/i });
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("gagal register");
    });
  });

  // ── School dropdown (Bug B) ──

  it("shows school dropdown for super_admin role", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };

    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    // The school combobox should appear
    const schoolPicker = screen.getByRole("combobox", { name: /sekolah/i });
    expect(schoolPicker).toBeInTheDocument();
  });

  it("does not show school dropdown for admin_school role", async () => {
    authStore = { token: "t", user: { role: "admin_school" } };

    render(<SchoolStudentsPage />);

    await waitFor(() => {
      expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    });

    // No school combobox should exist
    expect(screen.queryByRole("combobox", { name: /sekolah/i })).not.toBeInTheDocument();
  });

  // ── School picker inside the Register Student dialog (super_admin) ──

  it("registers a super_admin's student with no school at all", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };
    schoolsState = {
      data: {
        data: [{ id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA"] }],
        next_cursor: undefined,
      },
      isLoading: false,
      isError: false,
    };

    render(<SchoolStudentsPage />);
    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));
    const dialog = screen.getByRole("dialog");

    // Leave the school picker untouched: not every registrant has a school
    // (mahasiswa and the general public sign up too). Jenjang must still be
    // choosable, from the fallback list rather than a school's types.
    const jenjangTrigger = within(dialog).getAllByRole("combobox")[2];
    expect(jenjangTrigger).not.toBeDisabled();
    fireEvent.click(jenjangTrigger);
    fireEvent.click(await screen.findByRole("option", { name: "SMP" }));

    fireEvent.input(screen.getByPlaceholderText("Nama Lengkap"), {
      target: { value: "Peserta Umum" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /daftarkan siswa/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({ name: "Peserta Umum", jenjang: "SMP" }),
          schoolId: "",
        }),
      );
    });
  });

  it("shows a school picker inside the register dialog for super_admin, independent of the page-level filter", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };
    schoolsState = {
      data: {
        data: [
          { id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA"] },
          { id: "s2", name: "SMAN 2 Bandung", school_types: ["SMP", "SMA"] },
        ],
        next_cursor: undefined,
      },
      isLoading: false,
      isError: false,
    };

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));

    const dialog = screen.getByRole("dialog");
    const jenjangTrigger = within(dialog).getAllByRole("combobox")[2];

    // Jenjang stays usable with no school picked — school is optional, and
    // gating jenjang on it made registering a school-less person impossible.
    expect(jenjangTrigger).not.toBeDisabled();

    const dialogSchoolPicker = within(dialog).getByRole("combobox", { name: /sekolah/i });
    fireEvent.click(dialogSchoolPicker);
    fireEvent.click(await screen.findByText("SMAN 2 Bandung"));

    expect(jenjangTrigger).not.toBeDisabled();
    fireEvent.click(jenjangTrigger);
    fireEvent.click(await screen.findByRole("option", { name: "SMP" }));

    const nameInput = screen.getByPlaceholderText("Nama Lengkap");
    fireEvent.input(nameInput, { target: { value: "Rina Wijaya" } });

    fireEvent.click(within(dialog).getByRole("button", { name: /daftarkan siswa/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({ name: "Rina Wijaya", jenjang: "SMP" }),
          schoolId: "s2",
        }),
      );
    });
  });

  it("requires picking a school inside the dialog for super_admin before submitting", async () => {
    authStore = { token: "t", user: { role: "super_admin" } };
    schoolsState = {
      data: { data: [{ id: "s1", name: "SMAN 1 Jakarta", school_types: ["SMA"] }], next_cursor: undefined },
      isLoading: false,
      isError: false,
    };

    render(<SchoolStudentsPage />);

    await waitFor(() => expect(screen.getByText("Budi Santoso")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /daftarkan siswa/i }));

    const nameInput = screen.getByPlaceholderText("Nama Lengkap");
    fireEvent.input(nameInput, { target: { value: "Tanpa Sekolah" } });

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /daftarkan siswa/i }));

    expect(mockMutateAsync).not.toHaveBeenCalled();
    expect(toast.error).toHaveBeenCalledWith("Semua field harus diisi");
  });
});
