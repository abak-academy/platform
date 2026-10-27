import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import AffiliateDashboardPage from "./page";
import type { AffiliatorDashboard } from "@/lib/types";

let allowed = true;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => allowed }));
vi.mock("@/components/admin/AffiliatorNotice", () => ({
  AffiliatorNotice: () => null,
  useAffiliatorReadOnly: () => false,
}));

const dashboardCalls: { from?: string; to?: string }[] = [];
let dashboardState: { data?: AffiliatorDashboard; isLoading: boolean; isError: boolean; refetch: () => void };
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorDashboard: (range: { from?: string; to?: string }) => {
    dashboardCalls.push(range);
    return dashboardState;
  },
}));

const sample: AffiliatorDashboard = {
  schools: [
    { school_id: "s1", school_name: "SMAN 1", student_count: 30 },
    { school_id: "s2", school_name: "SMAN 2", student_count: 12 },
  ],
  totals: { paid_orders: 3, paid_units: 40, paid_amount: 1800000, discount_total: 200000, pending_orders: 1 },
  monthly: [{ month: "2026-10", orders: 3, units: 40, amount: 1800000 }],
  top_products: [{ school_id: "s1", school_name: "SMAN 1", product_id: "p1", product_name: "Try Out UTBK", product_type: "exam", units: 25 }],
  unpurchased_students: 7,
};

describe("AffiliateDashboardPage", () => {
  beforeEach(() => {
    allowed = true;
    dashboardCalls.length = 0;
    dashboardState = { data: sample, isLoading: false, isError: false, refetch: vi.fn() };
  });

  it("shows the totals, schools, trend and best sellers", () => {
    render(<AffiliateDashboardPage />);

    // The paid amount is both a stat card and the one trend row.
    expect(screen.getAllByText("Rp1.800.000")).toHaveLength(2);
    expect(screen.getByText("Rp200.000")).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument(); // 30 + 12 students
    const schools = screen.getByRole("region", { name: "Sekolah binaan" });
    expect(within(schools).getByText("SMAN 2")).toBeInTheDocument();
    expect(screen.getByText("2026-10")).toBeInTheDocument();
    expect(screen.getByText("Try Out UTBK (Ujian)")).toBeInTheDocument();
  });

  it("links to the students that have no purchase yet", () => {
    render(<AffiliateDashboardPage />);

    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lihat siswanya" })).toHaveAttribute("href", "/admin/affiliate/students?not_purchased=true");
  });

  it("starts on all time and sends an explicit window for the 30-day preset", () => {
    render(<AffiliateDashboardPage />);
    expect(dashboardCalls.at(-1)).toEqual({});

    fireEvent.click(screen.getByRole("button", { name: /30/ }));
    const last = dashboardCalls.at(-1);
    expect(last?.from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(last?.to).toBeUndefined();
  });

  it("explains empty tables instead of showing blanks", () => {
    dashboardState = { ...dashboardState, data: { ...sample, schools: [], monthly: [], top_products: [] } };
    render(<AffiliateDashboardPage />);

    expect(screen.getByText("Belum ada sekolah yang ditetapkan. Hubungi super admin.")).toBeInTheDocument();
    expect(screen.getAllByText("Belum ada pembelian pada periode ini.")).toHaveLength(2);
  });

  it("offers a reload when the dashboard fails to load", () => {
    const refetch = vi.fn();
    dashboardState = { data: undefined, isLoading: false, isError: true, refetch };
    render(<AffiliateDashboardPage />);

    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });

  it("shows placeholders while loading", () => {
    dashboardState = { data: undefined, isLoading: true, isError: false, refetch: vi.fn() };
    render(<AffiliateDashboardPage />);

    expect(screen.queryByText("Rp1.800.000")).not.toBeInTheDocument();
    expect(screen.queryByRole("region", { name: "Sekolah binaan" })).not.toBeInTheDocument();
  });

  it("refuses a caller without affiliator access", () => {
    allowed = false;
    render(<AffiliateDashboardPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
