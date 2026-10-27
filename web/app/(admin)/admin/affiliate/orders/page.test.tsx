import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import AffiliateOrdersPage from "./page";
import type { AffiliatorOrderRow } from "@/lib/types";

vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => true }));
vi.mock("@/components/admin/AffiliatorNotice", () => ({ AffiliatorNotice: () => null }));
vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));

const calls: Record<string, unknown>[] = [];
let state: { data?: { data: AffiliatorOrderRow[]; next_cursor?: string }; isLoading: boolean; isError: boolean; refetch: () => void };
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorOrders: (opts: Record<string, unknown>) => {
    calls.push(opts);
    return state;
  },
}));

const order: AffiliatorOrderRow = {
  id: "o1", status: "payment_pending", subtotal: 100000, discount: 10000, total: 90000, product_name: "Try Out UTBK",
  product_type: "exam", participant_count: 2, school_name: "SMAN 1", promo_code: "AFF10", payment_expires_at: null,
  created_at: "2026-10-20T03:00:00Z",
};

describe("AffiliateOrdersPage", () => {
  beforeEach(() => {
    calls.length = 0;
    state = { data: { data: [order, { ...order, id: "o2", status: "paid", promo_code: null }] }, isLoading: false, isError: false, refetch: vi.fn() };
  });

  it("lists orders with product, school, seats, promo, total and a readable status", () => {
    render(<AffiliateOrdersPage />);

    expect(screen.getAllByText("Try Out UTBK")).toHaveLength(2);
    expect(screen.getAllByText("SMAN 1")).toHaveLength(2);
    expect(screen.getAllByText("Rp90.000")).toHaveLength(2);
    expect(screen.getByText("AFF10")).toBeInTheDocument();
    expect(screen.getByText("Menunggu pembayaran")).toBeInTheDocument();
    expect(screen.getByText("Dibayar")).toBeInTheDocument();
  });

  it("links to a new purchase", () => {
    render(<AffiliateOrdersPage />);

    expect(screen.getByRole("link", { name: "Pembelian baru" })).toHaveAttribute("href", "/admin/affiliate/orders/new");
  });

  it("filters by status and shows the filter as a removable chip", () => {
    render(<AffiliateOrdersPage />);

    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    fireEvent.change(screen.getByRole("combobox", { name: "Status" }), { target: { value: "paid" } });
    expect(calls.at(-1)).toMatchObject({ status: "paid" });
    expect(screen.getByRole("group", { name: "Filter aktif" })).toHaveTextContent("Dibayar");

    fireEvent.click(screen.getByRole("button", { name: /Hapus filter: Dibayar/ }));
    expect(calls.at(-1)?.status).toBeUndefined();
  });

  it("pages with the server cursor", () => {
    state = { ...state, data: { data: [order], next_cursor: "c2" } };
    render(<AffiliateOrdersPage />);

    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(calls.at(-1)).toMatchObject({ cursor: "c2" });
  });

  it("explains an empty list and offers a retry on failure", () => {
    state = { data: { data: [] }, isLoading: false, isError: false, refetch: vi.fn() };
    const { unmount } = render(<AffiliateOrdersPage />);
    expect(screen.getByText("Belum ada pembelian.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    state = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AffiliateOrdersPage />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });
});
