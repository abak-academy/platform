import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import StoreDashboardPage from "./page";
import type { OrderSummary } from "@/lib/types";

const sampleSummary: OrderSummary = {
  buckets: {
    needs_confirm: 7,
    ready_to_ship: 23,
    shipment_failed: 2,
    in_transit: 11,
    created_this_month: 148,
    completed_this_month: 96,
    total: 512,
  },
  top_products: [
    { product_id: "p1", name: "Buku Latihan TKA", product_type: "book", qty_sold: 340, order_count: 210 },
    { product_id: "p2", name: "Medali Juara", product_type: "medal", qty_sold: 88, order_count: 61 },
  ],
};

let summaryState = {
  data: null as OrderSummary | null,
  isLoading: false,
};

vi.mock("@/lib/hooks/admin-orders", () => ({
  useAdminOrderSummary: () => summaryState,
}));

describe("StoreDashboardPage", () => {
  beforeEach(() => {
    summaryState = { data: sampleSummary, isLoading: false };
  });

  it("renders the queue counts and the month volume", () => {
    render(<StoreDashboardPage />);

    const needsConfirm = screen.getByText("Perlu konfirmasi").closest("div")!.parentElement!;
    expect(within(needsConfirm).getByText("7")).toBeInTheDocument();

    const readyToShip = screen.getByText("Siap kirim").closest("div")!.parentElement!;
    expect(within(readyToShip).getByText("23")).toBeInTheDocument();

    const failed = screen.getByText("Pengiriman gagal").closest("div")!.parentElement!;
    expect(within(failed).getByText("2")).toBeInTheDocument();

    expect(screen.getByText("148")).toBeInTheDocument();
    expect(screen.getByText("96")).toBeInTheDocument();
  });

  it("renders the top products by quantity sold", () => {
    render(<StoreDashboardPage />);

    const row = screen.getByText("Buku Latihan TKA").closest("tr")!;
    expect(within(row).getByText("340")).toBeInTheDocument();
    expect(within(row).getByText("210")).toBeInTheDocument();
    expect(screen.getByText("Medali Juara")).toBeInTheDocument();
  });

  // admin_store must never see revenue aggregates. Asserted against the rendered
  // text rather than the props, because what leaks is what a user can read.
  it("prints no rupiah figure anywhere on the page", () => {
    const { container } = render(<StoreDashboardPage />);

    expect(container.textContent).not.toMatch(/Rp\s?[\d.]/);
  });

  it("offers no route into the revenue report", () => {
    const { container } = render(<StoreDashboardPage />);

    expect(container.querySelector('a[href="/admin/revenue"]')).toBeNull();
    expect(container.querySelector('a[href="/admin/orders"]')).not.toBeNull();
    expect(container.querySelector('a[href="/admin/products"]')).not.toBeNull();
  });

  it("shows skeletons while the summary loads", () => {
    summaryState = { data: null, isLoading: true };

    render(<StoreDashboardPage />);

    expect(screen.queryByText("Perlu konfirmasi")).toBeNull();
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
        "/admin/orders?status=paid",
        "/admin/orders?status=shipment_failed",
      ]),
    );
  });
});
