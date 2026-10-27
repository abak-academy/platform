import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { AffiliatorListTab } from "./AffiliatorListTab";
import type { Affiliator } from "@/lib/types";

vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/admin/AffiliatorCreateDialog", () => ({
  AffiliatorCreateDialog: ({ open }: { open: boolean }) => (open ? <div role="dialog">create dialog</div> : null),
}));

const listCalls: Record<string, unknown>[] = [];
let listState: { data?: { data: Affiliator[]; total: number; next_cursor?: string }; isLoading: boolean; isError: boolean; refetch: () => void };
const statusMutate = vi.fn();
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  useAdminAffiliators: (p: Record<string, unknown>) => {
    listCalls.push(p);
    return listState;
  },
  useSetAffiliatorStatus: () => ({ mutateAsync: statusMutate, isPending: false }),
}));

const rows: Affiliator[] = [
  { id: "a1", name: "Andi", email: "andi@example.com", status: "active", school_ids: ["s1", "s2"], student_count: 40, order_count: 3, paid_amount: 900000, created_at: "", updated_at: "" },
  { id: "a2", name: "Bela", email: "bela@example.com", status: "deactivated", school_ids: ["s3"], student_count: 5, order_count: 0, paid_amount: 0, created_at: "", updated_at: "" },
];

describe("AffiliatorListTab", () => {
  beforeEach(() => {
    listCalls.length = 0;
    listState = { data: { data: rows, total: 2 }, isLoading: false, isError: false, refetch: vi.fn() };
    statusMutate.mockReset().mockResolvedValue({});
    vi.stubGlobal("confirm", vi.fn(() => true));
  });

  it("shows each affiliator with schools, students, orders, amount and status", () => {
    render(<AffiliatorListTab />);

    const row = screen.getByText("Andi").closest("tr")!;
    expect(within(row).getByText("andi@example.com")).toBeInTheDocument();
    expect(within(row).getByText("Rp900.000")).toBeInTheDocument();
    expect(within(row).getByText("Aktif")).toBeInTheDocument();
    expect(within(screen.getByText("Bela").closest("tr")!).getByText("Nonaktif")).toBeInTheDocument();
  });

  it("offers nothing but activate or deactivate on a row", () => {
    render(<AffiliatorListTab />);

    const rowButtons = (name: string) => within(screen.getByText(name).closest("tr")!).getAllByRole("button").map((b) => b.textContent);
    expect(rowButtons("Andi")).toEqual(["Nonaktifkan"]);
    expect(rowButtons("Bela")).toEqual(["Aktifkan"]);
  });

  it("asks before deactivating, then deactivates", async () => {
    render(<AffiliatorListTab />);

    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Nonaktifkan" }));

    expect(confirm).toHaveBeenCalled();
    await waitFor(() => expect(statusMutate).toHaveBeenCalledWith({ id: "a1", status: "deactivated" }));
  });

  it("does not deactivate when the admin declines, and reactivates without asking", async () => {
    vi.stubGlobal("confirm", vi.fn(() => false));
    const { unmount } = render(<AffiliatorListTab />);
    fireEvent.click(within(screen.getByText("Andi").closest("tr")!).getByRole("button", { name: "Nonaktifkan" }));
    expect(statusMutate).not.toHaveBeenCalled();
    unmount();

    vi.stubGlobal("confirm", vi.fn(() => true));
    render(<AffiliatorListTab />);
    fireEvent.click(within(screen.getByText("Bela").closest("tr")!).getByRole("button", { name: "Aktifkan" }));
    expect(confirm).not.toHaveBeenCalled();
    await waitFor(() => expect(statusMutate).toHaveBeenCalledWith({ id: "a2", status: "active" }));
  });

  it("searches on the server and starts over from page one", () => {
    listState = { data: { data: rows, total: 40, next_cursor: "c2" }, isLoading: false, isError: false, refetch: vi.fn() };
    render(<AffiliatorListTab />);
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(listCalls.at(-1)).toMatchObject({ cursor: "c2" });

    fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama atau email" }), { target: { value: "bela" } });

    expect(listCalls.at(-1)).toMatchObject({ q: "bela" });
    expect(listCalls.at(-1)?.cursor).toBeUndefined();
  });

  it("filters by status with a removable chip", () => {
    render(<AffiliatorListTab />);

    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    fireEvent.change(screen.getByRole("combobox", { name: "Status" }), { target: { value: "deactivated" } });
    expect(listCalls.at(-1)).toMatchObject({ status: "deactivated" });
    expect(screen.getByRole("group", { name: "Filter aktif" })).toHaveTextContent("Nonaktif");

    fireEvent.click(screen.getByRole("button", { name: /Hapus filter: Nonaktif/ }));
    expect(listCalls.at(-1)?.status).toBeUndefined();
  });

  it("pages with the server cursor, shows the range and changes the page size", () => {
    listState = { data: { data: rows, total: 57, next_cursor: "c2" }, isLoading: false, isError: false, refetch: vi.fn() };
    render(<AffiliatorListTab />);
    expect(screen.getByTestId("pager-range")).toHaveTextContent("1–2 dari 57");

    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(listCalls.at(-1)).toMatchObject({ cursor: "c2", limit: 20 });
    expect(screen.getByTestId("pager-range")).toHaveTextContent("21–22 dari 57");

    fireEvent.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(listCalls.at(-1)?.cursor).toBeUndefined();

    fireEvent.change(screen.getByRole("combobox", { name: "Baris per halaman" }), { target: { value: "50" } });
    expect(listCalls.at(-1)).toMatchObject({ limit: 50 });
  });

  it("opens the create dialog", () => {
    render(<AffiliatorListTab />);

    fireEvent.click(screen.getByRole("button", { name: "Tambah affiliator" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("create dialog");
  });

  it("explains an empty list and offers a retry on failure", () => {
    listState = { data: { data: [], total: 0 }, isLoading: false, isError: false, refetch: vi.fn() };
    const { unmount } = render(<AffiliatorListTab />);
    expect(screen.getByText("Belum ada affiliator.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    listState = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AffiliatorListTab />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });
});
