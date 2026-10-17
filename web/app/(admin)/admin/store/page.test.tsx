import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import StoreDashboardPage from "./page";
import type { OrderSummary, PromoCode } from "@/lib/types";

const sampleSummary: OrderSummary = {
  couriers: [],
  buckets: {
    needs_confirm: 7,
    ready_to_ship: 23,
    shipment_failed: 2,
    in_transit: 11, shipped: 0,
    completed: 0,
    payment_expired: 0,
    cancelled: 0,
    created_this_month: 148,
    completed_this_month: 96,
    total: 512,
  },
  top_products: [
    { product_id: "p1", name: "Buku Latihan TKA", product_type: "book", qty_sold: 340, order_count: 210 },
    { product_id: "p2", name: "Medali Juara", product_type: "medal", qty_sold: 88, order_count: 61 },
  ],
};

const summaryRequest = vi.fn();
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(), useRouter: () => ({ replace: vi.fn() }) }));

let summaryState = {
  data: null as OrderSummary | null,
  isLoading: false,
};

vi.mock("@/lib/hooks/admin-orders", () => ({
  useAdminOrderSummary: (range: unknown) => { summaryRequest(range); return summaryState; },
}));

vi.mock("@/lib/hooks/admin-unit-sales", () => ({
  useAdminUnitSales: () => ({ data: { from: null, to: null, types: [] }, isLoading: false, isError: false }),
  downloadUnitSalesCsv: vi.fn(),
}));

const PROMOS: PromoCode[] = [
  { id: "p1", code: "HEMAT10", used_count: 3, max_uses: 100, expires_at: null },
  { id: "p2", code: "KILAT", used_count: 1, max_uses: 50, expires_at: "2026-08-15T00:00:00+07:00" },
  { id: "p3", code: "LAMA", used_count: 9, max_uses: 10, expires_at: "2026-08-01T00:00:00+07:00" },
  { id: "p4", code: "HABIS", used_count: 20, max_uses: 20, expires_at: null },
];

let promoState = {
  data: PROMOS as PromoCode[],
};

vi.mock("@/lib/hooks/admin-promos", () => ({
  useAdminPromoCodes: () => promoState,
}));

let authStore: {
  token: string | null;
  user: { role?: string; name?: string } | null;
} = {
  token: "t",
  user: { role: "admin_store", name: "Siti" },
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));

let meState: {
  data: { role?: string; name?: string } | null;
  isError: boolean;
  isLoading: boolean;
} = { data: null, isError: false, isLoading: false };

vi.mock("@/lib/hooks/auth", async () => {
  const actual = await vi.importActual<typeof import("@/lib/hooks/auth")>(
    "@/lib/hooks/auth"
  );
  return {
    ...actual,
    useMe: ({ enabled }: { enabled?: boolean }) =>
      enabled
        ? meState
        : { data: null, isError: false, isLoading: false },
  };
});

describe("admin_store dashboard", () => {
  beforeEach(() => {
    summaryRequest.mockClear();
    vi.setSystemTime(new Date("2026-08-12T00:00:00+07:00"));
    summaryState = { data: sampleSummary, isLoading: false };
    promoState = { data: PROMOS };
    authStore = { token: "t", user: { role: "admin_store", name: "Siti" } };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("renders the queue counts and the month volume", () => {
    render(<StoreDashboardPage />);

    const needsConfirm = screen.getByText("Perlu konfirmasi").closest("a")!;
    expect(within(needsConfirm).getByText("7")).toBeInTheDocument();

    const readyToShip = screen.getByText("Siap kirim").closest("a")!;
    expect(within(readyToShip).getByText("23")).toBeInTheDocument();

    const failed = screen.getByText("Pengiriman gagal").closest("a")!;
    expect(within(failed).getByText("2")).toBeInTheDocument();

    expect(screen.getByText("148")).toBeInTheDocument();
    expect(screen.getByText("96")).toBeInTheDocument();
  });

  it("includes the unit sales block with its own all-time control", () => {
    render(<StoreDashboardPage />);
    expect(screen.getByRole("heading", { name: "Unit terjual per tipe produk" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Semua waktu" })).toBeInTheDocument();
  });

  it("renders the top products by quantity sold", () => {
    render(<StoreDashboardPage />);

    const row = screen.getByText("Buku Latihan TKA").closest("li")!;
    expect(within(row).getByText("340")).toBeInTheDocument();
    expect(screen.queryByText("210")).not.toBeInTheDocument();
    expect(screen.queryByText("61")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Produk teratas · bulan berjalan (WIB)" })).toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: /order/i })).not.toBeInTheDocument();
    expect(row.querySelector("[style*=width]")).toBeNull();
    expect(screen.getByText("Medali Juara")).toBeInTheDocument();
  });

  it("shows skeletons while the summary loads", () => {
    summaryState = { data: null, isLoading: true };

    render(<StoreDashboardPage />);

    expect(screen.queryByText("Perlu konfirmasi")).toBeNull();
  });

  it("keeps the three queue cards deep-linking to their filtered lists", () => {
    render(<StoreDashboardPage />);
    expect(screen.getByRole("link", { name: /perlu konfirmasi/i })).toHaveAttribute(
      "href", "/admin/orders?status=pending",
    );
  });

  // Each queue card must open its own queue; all three pointing at the
  // unfiltered list made them a count with nowhere to go.
  it("deep links each queue card to its matching filter", () => {
    render(<StoreDashboardPage />);
    const hrefs = Array.from(document.querySelectorAll('a[href^="/admin/orders"]')).map((a) =>
      a.getAttribute("href"),
    );
    expect(hrefs).toEqual(
      expect.arrayContaining([
        "/admin/orders?status=pending",
        "/admin/orders?queue=ready_to_ship",
        "/admin/orders?queue=shipment_failed",
      ]),
    );
  });

  // The card reads buckets.ready_to_ship (paid + processing, physical item
  // only); status=paid alone is a bigger, wrong set. See admin_order.go's
  // OrderFilter.ReadyToShip.
  it("links the ready-to-ship card to the ready_to_ship queue, not status=paid", () => {
    render(<StoreDashboardPage />);
    expect(document.querySelector('a[href="/admin/orders?status=paid"]')).toBeNull();
    expect(document.querySelector('a[href="/admin/orders?queue=ready_to_ship"]')).not.toBeNull();
  });

  it("counts active and expiring-soon promos, ignoring expired and exhausted ones", () => {
    render(<StoreDashboardPage />);
    expect(screen.getByTestId("store-promos-active")).toHaveTextContent("2");
    expect(screen.getByTestId("store-promos-expiring")).toHaveTextContent("1");
  });

  it("counts an unlimited promo (max_uses: null) as active, not expiring", () => {
    promoState = {
      data: [{ id: "u1", code: "UNLIMITED", used_count: 500, max_uses: null, expires_at: null }],
    };
    render(<StoreDashboardPage />);
    expect(screen.getByTestId("store-promos-active")).toHaveTextContent("1");
    expect(screen.getByTestId("store-promos-expiring")).toHaveTextContent("0");
  });

  it("counts a promo with a finite max_uses and no expiry as active, not expiring", () => {
    promoState = {
      data: [{ id: "f1", code: "FINITE", used_count: 3, max_uses: 100, expires_at: null }],
    };
    render(<StoreDashboardPage />);
    expect(screen.getByTestId("store-promos-active")).toHaveTextContent("1");
    expect(screen.getByTestId("store-promos-expiring")).toHaveTextContent("0");
  });

  it("renders no aggregate revenue figure anywhere", () => {
    const { container } = render(<StoreDashboardPage />);
    expect(container.textContent).not.toMatch(/pendapatan/i);
    expect(container.textContent).not.toMatch(/Rp\s?[\d.]{4,}/);
  });

  it("offers no route into the revenue report", () => {
    const { container } = render(<StoreDashboardPage />);

    expect(container.querySelector('a[href="/admin/revenue"]')).toBeNull();
    expect(container.querySelector('a[href="/admin/orders?status=pending"]')).not.toBeNull();
    expect(container.querySelector('a[href="/admin/products"]')).toBeNull();
  });

  it("keeps the hero, compact volume, and promo link and removes Quick Actions", () => {
    render(<StoreDashboardPage />);
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.queryByText("Akses Cepat")).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /kelola produk|buat promo|kirim pengumuman/i })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /lihat promo/i })).toHaveAttribute("href", "/admin/promos");
    const volume = screen.getByRole("region", { name: "Volume bulan ini" });
    expect(volume).toHaveTextContent("148");
    expect(volume).toHaveTextContent("96");
    expect(volume).toHaveClass("flex", "!py-3");
    expect(summaryRequest).toHaveBeenCalledWith({ status: "all" });
  });

  it("keeps the shipment failure accent only for nonzero failures", () => {
    const view = render(<StoreDashboardPage />);
    const failed = screen.getByRole("link", { name: /pengiriman gagal/i });
    expect(within(failed).getByText("2")).toHaveStyle({ color: "var(--md-sys-color-error)" });
    summaryState = { data: { ...sampleSummary, buckets: { ...sampleSummary.buckets, shipment_failed: 0 } }, isLoading: false };
    view.rerender(<StoreDashboardPage />);
    expect(within(screen.getByRole("link", { name: /pengiriman gagal/i })).getByText("0").style.color).toBe("");
  });

  it("keeps the top products empty state", () => {
    summaryState = { data: { ...sampleSummary, top_products: [] }, isLoading: false };
    render(<StoreDashboardPage />);
    expect(screen.getByText("Belum ada produk terjual bulan ini.")).toBeInTheDocument();
  });

  it("refuses a role without orders capability", () => {
    authStore = { token: "t", user: { role: "admin_school", name: "Budi" } };
    render(<StoreDashboardPage />);
    expect(screen.getByTestId("no-access")).toBeTruthy();
  });
});
