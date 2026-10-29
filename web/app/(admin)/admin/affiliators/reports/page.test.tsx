import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import AffiliateReportsPage from "./page";

let allowed = true;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => allowed }));

const periods: Record<string, unknown>[] = [];
vi.mock("@/components/admin/AffiliateReports", () => {
  const stub = (name: string) => ({ period }: { period: Record<string, unknown> }) => {
    periods.push({ name, ...period });
    return <div>{name} report</div>;
  };
  return {
    AFFILIATE_REPORT_TABS: [
      { id: "affiliators", labelKey: "aff_report_tab_affiliators" },
      { id: "schools", labelKey: "aff_report_tab_schools" },
      { id: "products", labelKey: "aff_report_tab_products" },
      { id: "promos", labelKey: "aff_report_tab_promos" },
      { id: "trend", labelKey: "aff_report_tab_trend" },
    ],
    AffiliatorsReport: stub("affiliators"),
    SchoolsReport: stub("schools"),
    ProductsReport: stub("products"),
    PromosReport: stub("promos"),
    TrendReport: stub("trend"),
  };
});

describe("AffiliateReportsPage", () => {
  beforeEach(() => {
    allowed = true;
    periods.length = 0;
  });

  it("offers the five reports, per affiliator first", () => {
    render(<AffiliateReportsPage />);

    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Per affiliator", "Per sekolah", "Produk terlaris", "Penggunaan promo", "Tren bulanan",
    ]);
    expect(screen.getByText("affiliators report")).toBeInTheDocument();
  });

  it("starts on the last 30 days with an explicit start date", () => {
    render(<AffiliateReportsPage />);

    const p = periods.at(-1)!;
    expect(p.from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(p.to).toBeUndefined();
  });

  it("sends no dates for all time", () => {
    render(<AffiliateReportsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Semua waktu" }));

    const p = periods.at(-1)!;
    expect(p.from).toBeUndefined();
    expect(p.to).toBeUndefined();
  });

  it("opens another report from its tab", () => {
    render(<AffiliateReportsPage />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Tren bulanan" }));

    expect(screen.getByText("trend report")).toBeInTheDocument();
  });

  it("is limited to super admins", () => {
    allowed = false;
    render(<AffiliateReportsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
