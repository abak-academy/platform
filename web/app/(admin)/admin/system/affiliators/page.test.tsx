import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import AdminAffiliatorsPage from "./page";
import type { Affiliator } from "@/lib/types";

let allowed = true;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => allowed }));
vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/admin/SchoolMultiPicker", () => ({
  SchoolMultiPicker: ({ value, onChange }: { value: { id: string; name: string }[]; onChange: (v: { id: string; name: string }[]) => void }) => (
    <div>
      <span data-testid="picked">{value.map((s) => s.id).join(",")}</span>
      <button type="button" onClick={() => onChange([...value, { id: "s1", name: "SMAN 1" }])}>add school</button>
    </div>
  ),
}));

const listCalls: (string | undefined)[] = [];
let listState: { data?: Affiliator[]; isLoading: boolean; isError: boolean; refetch: () => void };
const statusMutate = vi.fn();
const createMutate = vi.fn();
const bulkMutate = vi.fn();
const promoteMutate = vi.fn();
const schoolsMutate = vi.fn();
let detailState: { data?: unknown; isLoading: boolean };
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  useAdminAffiliators: (status?: string) => {
    listCalls.push(status);
    return listState;
  },
  useAdminAffiliator: (id: string | null) => (id ? detailState : { data: undefined, isLoading: false }),
  useSetAffiliatorStatus: () => ({ mutateAsync: statusMutate, isPending: false }),
  useCreateAffiliator: () => ({ mutateAsync: createMutate, isPending: false }),
  useBulkCreateAffiliators: () => ({ mutateAsync: bulkMutate, isPending: false }),
  usePromoteAffiliator: () => ({ mutateAsync: promoteMutate, isPending: false }),
  useSetAffiliatorSchools: () => ({ mutateAsync: schoolsMutate, isPending: false }),
}));
vi.mock("@/lib/hooks/admin-accounts", () => ({
  useAdminAccounts: () => ({ data: [{ id: "acc1", name: "Pak Guru", email: "guru@example.com" }] }),
}));

const rows: Affiliator[] = [
  { id: "a1", name: "Andi", email: "andi@example.com", status: "active", school_ids: ["s1", "s2"], student_count: 40, order_count: 3, paid_amount: 900000, created_at: "", updated_at: "" },
  { id: "a2", name: "Bela", email: "bela@example.com", status: "deactivated", school_ids: ["s3"], student_count: 5, order_count: 0, paid_amount: 0, created_at: "", updated_at: "" },
];

describe("AdminAffiliatorsPage", () => {
  beforeEach(() => {
    allowed = true;
    listCalls.length = 0;
    listState = { data: rows, isLoading: false, isError: false, refetch: vi.fn() };
    detailState = { data: undefined, isLoading: true };
    for (const m of [statusMutate, createMutate, bulkMutate, promoteMutate, schoolsMutate]) m.mockReset().mockResolvedValue({});
    vi.stubGlobal("confirm", vi.fn(() => true));
  });

  it("shows each affiliator with schools, students, orders, amount and status", () => {
    render(<AdminAffiliatorsPage />);

    const row = screen.getByText("Andi").closest("tr")!;
    expect(within(row).getByText("andi@example.com")).toBeInTheDocument();
    expect(within(row).getByText("Rp900.000")).toBeInTheDocument();
    expect(within(row).getByText("Aktif")).toBeInTheDocument();
    expect(within(screen.getByText("Bela").closest("tr")!).getByText("Nonaktif")).toBeInTheDocument();
  });

  it("is limited to super admins", () => {
    allowed = false;
    render(<AdminAffiliatorsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });

  it("searches by name or email without another request", () => {
    render(<AdminAffiliatorsPage />);

    fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama atau email" }), { target: { value: "bela@" } });

    expect(screen.queryByText("Andi")).not.toBeInTheDocument();
    expect(screen.getByText("Bela")).toBeInTheDocument();
  });

  it("filters by status through the server and shows the chip", () => {
    render(<AdminAffiliatorsPage />);

    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    fireEvent.change(screen.getByRole("combobox", { name: "Status" }), { target: { value: "deactivated" } });

    expect(listCalls.at(-1)).toBe("deactivated");
    expect(screen.getByRole("group", { name: "Filter aktif" })).toHaveTextContent("Nonaktif");
  });

  it("asks before deactivating, then deactivates", async () => {
    render(<AdminAffiliatorsPage />);

    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Nonaktifkan" }));

    expect(confirm).toHaveBeenCalled();
    await waitFor(() => expect(statusMutate).toHaveBeenCalledWith({ id: "a1", status: "deactivated" }));
  });

  it("does not deactivate when the admin declines", () => {
    vi.stubGlobal("confirm", vi.fn(() => false));
    render(<AdminAffiliatorsPage />);

    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Nonaktifkan" }));

    expect(statusMutate).not.toHaveBeenCalled();
  });

  it("reactivates without asking", async () => {
    render(<AdminAffiliatorsPage />);

    fireEvent.click(within(screen.getByText("Bela").closest("tr")!).getByRole("button", { name: "Aktifkan" }));

    expect(confirm).not.toHaveBeenCalled();
    await waitFor(() => expect(statusMutate).toHaveBeenCalledWith({ id: "a2", status: "active" }));
  });

  it("creates an affiliator only with name, email, a long enough password and a school", async () => {
    render(<AdminAffiliatorsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Tambah affiliator" }));
    const dialog = screen.getByRole("dialog");
    const submit = within(dialog).getByRole("button", { name: "Tambah affiliator" });
    expect(submit).toBeDisabled();

    fireEvent.change(within(dialog).getByLabelText("Nama"), { target: { value: "Citra" } });
    fireEvent.change(within(dialog).getByLabelText("Email"), { target: { value: "citra@example.com" } });
    fireEvent.change(within(dialog).getByLabelText(/Kata sandi/), { target: { value: "short" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "add school" }));
    expect(submit).toBeDisabled();

    fireEvent.change(within(dialog).getByLabelText(/Kata sandi/), { target: { value: "longenough1" } });
    expect(submit).toBeEnabled();
    fireEvent.click(submit);

    await waitFor(() =>
      expect(createMutate).toHaveBeenCalledWith({ name: "Citra", email: "citra@example.com", password: "longenough1", school_ids: ["s1"] }),
    );
  });

  it("promotes the chosen school admin account to the chosen schools", async () => {
    render(<AdminAffiliatorsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Jadikan affiliator" }));
    const dialog = screen.getByRole("dialog");

    fireEvent.change(within(dialog).getByRole("combobox", { name: "Akun school admin" }), { target: { value: "acc1" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "add school" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Jadikan affiliator" }));

    await waitFor(() => expect(promoteMutate).toHaveBeenCalledWith({ id: "acc1", school_ids: ["s1"] }));
  });

  it("bulk-creates from a CSV with the schools applied to every row and reports failures", async () => {
    bulkMutate.mockResolvedValue({
      results: [
        { row: 1, email: "a@example.com", ok: true },
        { row: 2, email: "b@example.com", ok: false, error: "email already registered" },
      ],
    });
    render(<AdminAffiliatorsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Impor massal" }));
    const dialog = screen.getByRole("dialog");

    const csv = "name,email,password\nA,a@example.com,password1\nB,b@example.com,password2\n";
    const file = new File([csv], "affiliators.csv", { type: "text/csv" });
    Object.defineProperty(file, "text", { value: () => Promise.resolve(csv) });
    fireEvent.change(within(dialog).getByLabelText("Berkas CSV"), { target: { files: [file] } });
    fireEvent.click(within(dialog).getByRole("button", { name: "add school" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Buat akun" }));

    await waitFor(() =>
      expect(bulkMutate).toHaveBeenCalledWith([
        { name: "A", email: "a@example.com", password: "password1", school_ids: ["s1"] },
        { name: "B", email: "b@example.com", password: "password2", school_ids: ["s1"] },
      ]),
    );
    expect(await screen.findByText("1 dari 2 akun dibuat.")).toBeInTheDocument();
    expect(screen.getByText("#2 b@example.com: email already registered")).toBeInTheDocument();
  });

  it("rejects a CSV with the wrong header before calling the server", async () => {
    render(<AdminAffiliatorsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Impor massal" }));
    const dialog = screen.getByRole("dialog");

    const csv = "name,email\nA,a@example.com\n";
    const file = new File([csv], "bad.csv", { type: "text/csv" });
    Object.defineProperty(file, "text", { value: () => Promise.resolve(csv) });
    fireEvent.change(within(dialog).getByLabelText("Berkas CSV"), { target: { files: [file] } });
    fireEvent.click(within(dialog).getByRole("button", { name: "add school" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Buat akun" }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Kolom name, email, dan password wajib ada di baris pertama."));
    expect(bulkMutate).not.toHaveBeenCalled();
  });

  it("shows an affiliator's totals and latest orders in the detail dialog", () => {
    detailState = {
      isLoading: false,
      data: {
        affiliator: { ...rows[0], schools: [{ id: "s1", name: "SMAN 1" }] },
        totals: { paid_orders: 3, paid_units: 40, paid_amount: 900000, discount_total: 50000, pending_orders: 1 },
        recent_orders: [{ id: "o1", status: "paid", product_name: "Try Out UTBK", participant_count: 2, total: 90000 }],
      },
    };
    render(<AdminAffiliatorsPage />);

    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Detail" }));
    const dialog = screen.getByRole("dialog");

    expect(within(dialog).getByText("SMAN 1")).toBeInTheDocument();
    expect(within(dialog).getByText("Try Out UTBK · 2")).toBeInTheDocument();
    expect(within(dialog).getByText("Rp90.000 · Dibayar")).toBeInTheDocument();
  });

  it("saves the schools chosen in the schools dialog", async () => {
    detailState = {
      isLoading: false,
      data: { affiliator: { ...rows[0], schools: [{ id: "s2", name: "SMAN 2" }] }, totals: {}, recent_orders: [] },
    };
    render(<AdminAffiliatorsPage />);

    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Atur sekolah" }));
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByTestId("picked")).toHaveTextContent("s2");
    fireEvent.click(within(dialog).getByRole("button", { name: "add school" }));
    fireEvent.click(within(dialog).getByRole("button", { name: "Simpan" }));

    await waitFor(() => expect(schoolsMutate).toHaveBeenCalledWith({ id: "a1", school_ids: ["s2", "s1"] }));
  });

  it("shows an empty state and a retry on failure", () => {
    listState = { data: [], isLoading: false, isError: false, refetch: vi.fn() };
    const { unmount } = render(<AdminAffiliatorsPage />);
    expect(screen.getByText("Belum ada affiliator.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    listState = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AdminAffiliatorsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });
});
