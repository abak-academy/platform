import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ReportsPage from "./page";

const h = vi.hoisted(() => ({ summary: vi.fn(), liability: vi.fn(), capability: vi.fn() }));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAffiliateReportSummary: h.summary,
  useAffiliateLiabilityReport: h.liability,
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: h.capability }));

const row = {
  affiliator_id: "a1",
  affiliator_code: "BUDI1234",
  affiliator_name: "Budi Santoso",
  level: "silver",
  students_new: 3,
  net_revenue: 900000,
  commissions: { available: 40000, paid: 10000, cancelled: 99000 },
  clicks: 12,
  conversion: 0.25,
};

describe("AdminAffiliateReportsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => m.mockReset());
    h.capability.mockReturnValue(true);
    h.summary.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [row], totals: { ...row, affiliator_name: "", affiliator_code: "" } },
    });
    h.liability.mockReturnValue({ data: { by_status: { available: 40000 }, payout_liability: 25000 } });
  });

  it("excludes cancelled commission from the row total", () => {
    render(<ReportsPage />);

    expect(screen.getByText("Rp50.000", { selector: "td" })).toBeInTheDocument();
    expect(screen.queryByText("Rp149.000")).toBeNull();
  });

  it("shows the payout liability", () => {
    render(<ReportsPage />);

    expect(screen.getByText("Rp25.000")).toBeInTheDocument();
  });

  it("does not query with an inverted range", () => {
    render(<ReportsPage />);

    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "2999-01-01" } });

    expect(screen.getByRole("alert")).toHaveTextContent("Tanggal akhir tidak boleh sebelum tanggal awal.");
    expect(h.summary).toHaveBeenLastCalledWith("", "");
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<ReportsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
