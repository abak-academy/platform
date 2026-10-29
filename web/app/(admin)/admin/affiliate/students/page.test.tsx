import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import AffiliateStudentsPage from "./page";
import type { AdminStudent } from "@/lib/types";

let readOnly = false;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => true }));
vi.mock("@/components/admin/AffiliatorNotice", () => ({
  AffiliatorNotice: () => null,
  useAffiliatorReadOnly: () => readOnly,
}));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: { user: { id: string } }) => unknown) => selector({ user: { id: "aff-1" } }),
}));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(searchParams),
}));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("@/components/admin/AffiliatorRegisterStudentModal", () => ({
  AffiliatorRegisterStudentModal: ({ open }: { open: boolean }) => (open ? <div role="dialog">register</div> : null),
}));
vi.mock("@/components/admin/BulkImportModal", () => ({
  BulkImportModal: ({ open, schoolPerRow, allowExplicitPassword }: { open: boolean; schoolPerRow?: boolean; allowExplicitPassword: boolean }) =>
    open ? <div role="dialog">bulk {String(schoolPerRow)} {String(allowExplicitPassword)}</div> : null,
}));
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorMe: () => ({ data: { schools: [{ id: "s1", name: "SMAN 1" }] } }),
}));

let searchParams = "";
const studentCalls: Record<string, unknown>[] = [];
let studentsState: { data?: { data: AdminStudent[]; next_cursor?: string; total?: number }; isLoading: boolean; isError: boolean; refetch: () => void };
vi.mock("@/lib/hooks/admin-students", () => ({
  useAdminStudents: (opts: Record<string, unknown>) => {
    studentCalls.push(opts);
    return studentsState;
  },
}));

const rows: AdminStudent[] = [
  { id: "a", name: "Budi", jenjang: "SMA", status: "active", school_name: "SMAN 1", registered_by: "aff-1", created_at: "2026-10-01T00:00:00Z" },
  { id: "b", name: "Siti", jenjang: "SMP", status: "deactivated", school_name: "SMAN 1", registered_by: null, created_at: "2026-10-02T00:00:00Z" },
];

describe("AffiliateStudentsPage", () => {
  beforeEach(() => {
    readOnly = false;
    searchParams = "";
    studentCalls.length = 0;
    studentsState = { data: { data: rows, total: 2 }, isLoading: false, isError: false, refetch: vi.fn() };
  });

  it("lists students and marks the ones this affiliator registered", () => {
    render(<AffiliateStudentsPage />);

    expect(screen.getByText("Budi")).toBeInTheDocument();
    expect(screen.getByText("Siti")).toBeInTheDocument();
    expect(screen.getByText("Saya")).toBeInTheDocument();
    expect(screen.getByText("Total: 2")).toBeInTheDocument();
  });

  it("starts filtered to students without a purchase when opened from the dashboard link", () => {
    searchParams = "not_purchased=true";
    render(<AffiliateStudentsPage />);

    expect(studentCalls.at(-1)).toMatchObject({ notPurchased: true });
    expect(screen.getByRole("group", { name: "Filter aktif" })).toHaveTextContent("Belum dibelikan produk");
  });

  it("removes a filter chip and queries again without it", () => {
    searchParams = "not_purchased=true";
    render(<AffiliateStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: /Hapus filter: Belum dibelikan produk/ }));

    expect(studentCalls.at(-1)).toMatchObject({ notPurchased: false });
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
  });

  it("sends the search text to the query", () => {
    render(<AffiliateStudentsPage />);

    fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama siswa" }), { target: { value: "bud" } });

    expect(studentCalls.at(-1)).toMatchObject({ q: "bud" });
  });

  it("opens the registration dialog", () => {
    render(<AffiliateStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Daftarkan siswa" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("register");
  });

  it("opens bulk import in school-per-row mode with no password column", () => {
    render(<AffiliateStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Impor massal" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("bulk true false");
  });

  it("disables registration and import while the account is deactivated", () => {
    readOnly = true;
    render(<AffiliateStudentsPage />);

    expect(screen.getByRole("button", { name: "Daftarkan siswa" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Impor massal" })).toBeDisabled();
  });

  it("pages forward with the server cursor and back again", () => {
    studentsState = { ...studentsState, data: { data: rows, total: 40, next_cursor: "c2" } };
    render(<AffiliateStudentsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(studentCalls.at(-1)).toMatchObject({ cursor: "c2" });

    fireEvent.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(studentCalls.at(-1)?.cursor).toBeUndefined();
  });

  it("shows an empty state and a retry on failure", () => {
    studentsState = { data: { data: [], total: 0 }, isLoading: false, isError: false, refetch: vi.fn() };
    const { unmount } = render(<AffiliateStudentsPage />);
    expect(screen.getByText("Tidak ada siswa.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    studentsState = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AffiliateStudentsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });
});
