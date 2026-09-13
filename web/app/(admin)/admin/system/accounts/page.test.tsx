import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import SystemAccountsPage from "./page";
import type { AdminAccount, School } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let accountsState = {
  data: undefined as AdminAccount[] | undefined,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let changeRoleState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let resetPwdState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

let schoolsState = {
  data: null as School[] | null,
  isLoading: false,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

vi.mock("@/lib/hooks/admin-accounts", () => ({
  useAdminAccounts: () => accountsState,
  useCreateAdminAccount: () => createState,
  useChangeAccountRole: () => changeRoleState,
  useChangeAccountStatus: () => changeStatusState,
  useResetAccountPassword: () => resetPwdState,
}));

vi.mock("@/lib/hooks/students", () => ({
  useSchools: () => schoolsState,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleAccounts: AdminAccount[] = [
  {
    id: "a1",
    name: "Admin Store",
    email: "store@test.com",
    role: "admin_store",
    status: "active",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  {
    id: "a2",
    name: "Admin School",
    email: "school@test.com",
    role: "admin_school",
    status: "active",
    school_id: "sch-1",
    created_at: "2026-02-01T00:00:00Z",
    updated_at: "2026-02-01T00:00:00Z",
  },
  {
    id: "a3",
    name: "Super Admin",
    email: "super@test.com",
    role: "super_admin",
    status: "active",
    created_at: "2026-03-01T00:00:00Z",
    updated_at: "2026-03-01T00:00:00Z",
  },
];

const sampleSchools: School[] = [
  { id: "sch-1", name: "SMAN 1 Jakarta" },
  { id: "sch-2", name: "SMAN 2 Jakarta" },
];

describe("SystemAccountsPage", () => {
  beforeEach(() => {
    accountsState = {
      data: sampleAccounts,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    changeRoleState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    changeStatusState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    resetPwdState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    schoolsState = {
      data: sampleSchools,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders loading state", async () => {
    accountsState = {
      data: undefined,
      isLoading: true,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<SystemAccountsPage />);

    await waitFor(() => {
      expect(screen.getByText("Memuat…")).toBeInTheDocument();
    });
  });

  it("renders error state", async () => {
    accountsState = {
      data: undefined,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<SystemAccountsPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat data/i)).toBeInTheDocument();
    });
  });

  it("renders the accounts table with account data", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => {
      expect(screen.getByText("Admin Store")).toBeInTheDocument();
      expect(screen.getByText("Admin School")).toBeInTheDocument();
    });

    // "Super Admin" is both a filter chip and a row value
    expect(screen.getAllByText("Super Admin").length).toBeGreaterThanOrEqual(1);
    // "Store Manager" is both a filter chip and a role badge
    expect(screen.getAllByText("Store Manager").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("School Operator").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Aktif").length).toBeGreaterThanOrEqual(1);
  });

  it("shows stat cards with total, active, deactivated counts", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => {
      const threes = screen.getAllByText("3");
      expect(threes.length).toBeGreaterThanOrEqual(1);
    });
  });

  it("school picker is hidden in create dialog (default role is admin_store)", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => expect(screen.getByText("Admin Store")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));

    expect(screen.getByText("Buat akun admin")).toBeInTheDocument();
    expect(screen.queryByText(/sekolah/i)).not.toBeInTheDocument();
  });

  it("shows the school picker in create dialog when role=admin_school is selected", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => expect(screen.getByText("Admin Store")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));
    const dialog = screen.getByRole("dialog");

    fireEvent.click(within(dialog).getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "School Operator" }));

    expect(within(dialog).getByText("Sekolah")).toBeInTheDocument();
  });

  it("blocks create submission with a required-school toast when admin_school role has no school selected", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => expect(screen.getByText("Admin Store")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat/i }));
    const dialog = screen.getByRole("dialog");

    fireEvent.input(within(dialog).getByPlaceholderText("Nama lengkap"), {
      target: { value: "Operator Baru" },
    });
    fireEvent.input(within(dialog).getByPlaceholderText("email@example.com"), {
      target: { value: "operator@test.com" },
    });
    fireEvent.input(within(dialog).getByPlaceholderText("Minimal 8 karakter"), {
      target: { value: "password123" },
    });

    fireEvent.click(within(dialog).getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "School Operator" }));

    fireEvent.click(within(dialog).getByRole("button", { name: /^buat$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Sekolah wajib dipilih untuk peran School Operator");
    });
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("shows and pre-fills the school picker in the role-change dialog for an existing admin_school account", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => expect(screen.getByText("Admin School")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const targetRow = rows.find((r) => within(r).queryByText("Admin School"));
    expect(targetRow).toBeTruthy();
    fireEvent.pointerDown(
      within(targetRow as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    fireEvent.click(await screen.findByText("Ganti peran"));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Sekolah")).toBeInTheDocument();
    expect(within(dialog).getByText("SMAN 1 Jakarta")).toBeInTheDocument();
  });

  it("blocks role-change submission with a required-school toast when admin_school has no school selected", async () => {
    render(<SystemAccountsPage />);

    await waitFor(() => expect(screen.getByText("Admin Store")).toBeInTheDocument());

    const rows = screen.getAllByRole("row");
    const targetRow = rows.find((r) => within(r).queryByText("Admin Store"));
    expect(targetRow).toBeTruthy();
    fireEvent.pointerDown(
      within(targetRow as HTMLElement).getByRole("button", { name: "" }),
      { button: 0 }
    );

    fireEvent.click(await screen.findByText("Ganti peran"));

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("combobox"));
    fireEvent.click(await screen.findByRole("option", { name: "School Operator" }));

    fireEvent.click(within(dialog).getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("Sekolah wajib dipilih untuk peran School Operator");
    });
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });
});
