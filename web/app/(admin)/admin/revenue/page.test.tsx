import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import RevenuePage from "./page";
import type { AdminRevenue } from "@/lib/types";
import type { ResolvedRole } from "@/lib/hooks/use-capability";

let revenueState = {
  data: null as AdminRevenue | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
};

let roleState: ResolvedRole = {
  role: "super_admin",
  hydrated: true,
  meIsError: false,
};

vi.mock("@/lib/hooks/admin-revenue", () => ({
  useAdminRevenue: () => revenueState,
}));

vi.mock("@/lib/hooks/use-capability", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/hooks/use-capability")>()),
  useResolvedRole: () => roleState,
}));

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <RevenuePage />
    </QueryClientProvider>,
  );
}

function baseRevenue(): AdminRevenue {
  return {
    total: 2_500_000,
    product_revenue: 2_300_000,
    shipping_total: 250_000,
    discount_total: 50_000,
    order_count: 4,
    by_type: {
      book: { total: 1_000_000, count: 10 },
      course: { total: 1_500_000, count: 3 },
    },
    top_products: [
      {
        product_id: "p1",
        name: "Buku Latihan TKA",
        product_type: "book",
        qty_sold: 12,
        order_count: 9,
        product_revenue: 1_000_000,
      },
      {
        product_id: "p2",
        name: "Kelas Intensif",
        product_type: "course",
        qty_sold: 3,
        order_count: 3,
        product_revenue: 1_500_000,
      },
    ],
    from: "2026-07-01",
    to: "2026-07-31",
  };
}

describe("RevenuePage", () => {
  beforeEach(() => {
    roleState = { role: "super_admin", hydrated: true, meIsError: false };
    revenueState = {
      data: baseRevenue(),
      isLoading: false,
      isError: false,
      error: null,
    };
  });

  it("renders KPI tiles from revenue data", async () => {
    renderPage();

    const stats = await screen.findByTestId("revenue-stats");
    expect(within(stats).getByText("Rp2.500.000")).toBeInTheDocument();
    expect(within(screen.getByTestId("stat-order-count")).getByText("4")).toBeInTheDocument();
    expect(within(stats).getByText("Rp625.000")).toBeInTheDocument();
  });

  it("labels the period with the range the server echoed, not local state", async () => {
    renderPage();

    const label = await screen.findByTestId("revenue-period-label");
    expect(label).toHaveTextContent("1 Jul 2026");
    expect(label).toHaveTextContent("31 Jul 2026");
  });

  it("counts one multi-type order once instead of summing by_type counts", async () => {
    revenueState = {
      data: {
        ...baseRevenue(),
        order_count: 1,
        by_type: {
          book: { total: 150_000, count: 1 },
          medal: { total: 50_000, count: 1 },
        },
      },
      isLoading: false,
      isError: false,
      error: null,
    };

    renderPage();

    const tile = await screen.findByTestId("stat-order-count");
    expect(within(tile).getByText("1")).toBeInTheDocument();
    expect(within(tile).queryByText("2")).not.toBeInTheDocument();
  });

  it("renders revenue-by-type bars", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByTestId("bar-book")).toBeInTheDocument();
    });
    expect(screen.getByTestId("bar-course")).toBeInTheDocument();

    expect(
      screen.getByText((_, node) => node?.textContent === "Rp1.000.000 · 10 pesanan"),
    ).toBeInTheDocument();
    expect(
      screen.getByText((_, node) => node?.textContent === "Rp1.500.000 · 3 pesanan"),
    ).toBeInTheDocument();
  });

  it("renders top products from the server payload", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("Produk terlaris")).toBeInTheDocument();
    });

    expect(screen.getByText("Buku Latihan TKA")).toBeInTheDocument();
    expect(screen.getByText("Kelas Intensif")).toBeInTheDocument();

    const headers = screen.getAllByRole("columnheader");
    expect(headers.map((h) => h.textContent)).toEqual(
      expect.arrayContaining(["Produk", "Terjual", "Pesanan", "Pendapatan"]),
    );
  });

  it("reconciles product revenue, shipping and discount to the total", async () => {
    renderPage();

    const block = await screen.findByTestId("revenue-reconciliation");
    expect(within(block).getByText("Pendapatan produk")).toBeInTheDocument();
    expect(within(block).getByText("Rp2.300.000")).toBeInTheDocument();
    expect(within(block).getByText(/Rp250\.000/)).toBeInTheDocument();
    expect(within(block).getByText(/Rp50\.000/)).toBeInTheDocument();
    expect(within(block).getByText("Rp2.500.000")).toBeInTheDocument();
  });

  it("shows loading skeletons while loading", () => {
    revenueState = {
      data: null,
      isLoading: true,
      isError: false,
      error: null,
    };

    renderPage();

    expect(document.querySelectorAll("[data-slot='skeleton']").length).toBeGreaterThan(0);
  });

  it("shows skeletons, not NoAccess, while the role is still hydrating", () => {
    roleState = { role: undefined, hydrated: false, meIsError: false };

    renderPage();

    expect(document.querySelectorAll("[data-slot='skeleton']").length).toBeGreaterThan(0);
    expect(screen.queryByText("Tidak punya akses")).not.toBeInTheDocument();
  });

  it("shows NoAccess once a role without revenue:read is known", () => {
    roleState = { role: "admin_school", hydrated: true, meIsError: false };

    renderPage();

    expect(screen.getByText("Tidak punya akses")).toBeInTheDocument();
  });

  it("surfaces an API error as inline error text", async () => {
    revenueState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
    };

    renderPage();

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
