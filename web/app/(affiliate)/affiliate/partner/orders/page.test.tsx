import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import PartnerOrdersPage from "./page";

const hook = vi.hoisted(() => ({
  state: {} as Record<string, unknown>,
  params: vi.fn(),
  fetchAll: vi.fn(),
  downloadCsv: vi.fn(),
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), push: vi.fn() }),
  usePathname: () => "/affiliate/partner/orders",
  useSearchParams: () => new URLSearchParams(hook.search.value),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() } }));
vi.mock("@/lib/csv", () => ({ downloadCsv: (...a: unknown[]) => hook.downloadCsv(...a) }));
vi.mock("@/components/cart/SnapCheckout", () => ({ SnapCheckout: () => <button>Bayar-cart</button> }));
vi.mock("@/components/partner/RetryPaymentButton", () => ({ RetryPaymentButton: () => <button>Bayar-lagi</button> }));
vi.mock("@/lib/hooks/partner", () => ({
  partnerKeys: { orders: () => ["partner", "orders"] },
  usePartnerOrders: (params: unknown) => {
    hook.params(params);
    return hook.state;
  },
  fetchAllPartnerPages: (...a: unknown[]) => hook.fetchAll(...a),
}));

const order = (over: Record<string, unknown>) => ({
  id: "o1",
  status: "paid",
  subtotal: 100000,
  discount: 0,
  total: 100000,
  created_at: "2026-11-09T10:00:00Z",
  items: [{ product_id: "p1", name: "Tryout UTBK", qty: 2, unit_price: 50000 }],
  ...over,
});
const page = (data: unknown[], total = data.length) => ({ isLoading: false, isError: false, data: { data, page: 1, per_page: 20, total } });

describe("PartnerOrdersPage", () => {
  beforeEach(() => {
    hook.search.value = "";
    hook.params.mockClear();
    hook.downloadCsv.mockClear();
  });

  it("lists the orders, offers payment only where it applies, and links to the detail", () => {
    hook.state = page([order({ id: "a", status: "cart" }), order({ id: "b", status: "payment_pending" }), order({ id: "c", status: "paid" })]);
    render(<PartnerOrdersPage />);

    expect(screen.getAllByText("Tryout UTBK")).toHaveLength(3);
    expect(screen.getAllByRole("button", { name: "Bayar-cart" })).toHaveLength(1);
    expect(screen.getAllByRole("button", { name: "Bayar-lagi" })).toHaveLength(1);
    expect(screen.getAllByRole("link", { name: "Lihat rincian" })[2]).toHaveAttribute("href", "/affiliate/partner/orders/c");
  });

  it("asks for the page, search, status and dates the URL holds", () => {
    hook.search.value = "q=utbk&status=paid&from=2026-11-01&to=2026-11-30&page=2";
    hook.state = page([order({})], 30);
    render(<PartnerOrdersPage />);
    expect(hook.params).toHaveBeenLastCalledWith({ q: "utbk", status: "paid", from: "2026-11-01", to: "2026-11-30", page: 2, per_page: 20 });
  });

  it("downloads every order the filters match", async () => {
    hook.search.value = "status=paid";
    hook.state = page([order({})], 45);
    hook.fetchAll.mockResolvedValue([order({}), order({ id: "o2" })]);
    render(<PartnerOrdersPage />);

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    await waitFor(() => expect(hook.downloadCsv).toHaveBeenCalled());
    expect(hook.fetchAll).toHaveBeenCalledWith("/partners/me/orders", { q: "", status: "paid", from: "", to: "" });
    expect(hook.downloadCsv.mock.calls[0][2]).toHaveLength(2);
  });

  it("says nothing matches when a filter is on, and nothing yet otherwise", () => {
    hook.search.value = "q=zzz";
    hook.state = page([]);
    const filtered = render(<PartnerOrdersPage />);
    expect(screen.getByText(/tidak ada yang cocok/i)).toBeInTheDocument();
    filtered.unmount();

    hook.search.value = "";
    render(<PartnerOrdersPage />);
    expect(screen.getByText(/belum ada pembelian/i)).toBeInTheDocument();
  });
});
