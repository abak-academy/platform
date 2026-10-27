import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ReportsPage from "./page";

const h = vi.hoisted(() => ({
  summary: vi.fn(),
  liability: vi.fn(),
  capability: vi.fn(),
  commissions: vi.fn(),
  download: vi.fn(),
  replace: vi.fn(),
  search: { value: "" },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: h.replace }),
  usePathname: () => "/admin/affiliates/reports",
  useSearchParams: () => new URLSearchParams(h.search.value),
}));

vi.mock("@/lib/hooks/admin-affiliates", () => ({
  useAffiliateReportSummary: h.summary,
  useAffiliateLiabilityReport: h.liability,
  useAdminAffiliateCommissions: h.commissions,
  downloadAffiliateCsv: h.download,
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
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.search.value = "";
    h.commissions.mockReturnValue({
      isLoading: false,
      isError: false,
      data: {
        data: [
          {
            id: "c1",
            order_id: "8f3a1b2c-0000-0000-0000-000000000000",
            student_name: "Sari Wulandari",
            product_name: "Kursus Matematika",
            product_type: "course",
            base_amount: 1000000,
            percentage: 10,
            amount: 100000,
            status: "available",
            paid_at: "2026-10-02T00:00:00Z",
            created_at: "2026-10-02T00:00:00Z",
          },
        ],
        total: 1,
        page: 1,
        per_page: 10,
      },
    });
    h.capability.mockReturnValue(true);
    h.summary.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [row], totals: { ...row, affiliator_name: "", affiliator_code: "" }, total: 1, page: 1, per_page: 20 },
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
    expect(h.summary).toHaveBeenLastCalledWith(expect.objectContaining({ from: "", to: "" }));
  });

  it("passes the search, level filter and page from the URL to the report", () => {
    h.search.value = "q=budi&type=gold&page=2";
    render(<ReportsPage />);

    expect(h.summary).toHaveBeenLastCalledWith(expect.objectContaining({ q: "budi", level: "gold", page: 2, per_page: 20 }));
  });

  it("opens the order details of one affiliator under its row and closes them again", () => {
    render(<ReportsPage />);
    expect(screen.queryByText("Kursus Matematika")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Lihat order" }));

    expect(screen.getByText("Kursus Matematika")).toBeInTheDocument();
    expect(screen.getByText("Sari Wulandari")).toBeInTheDocument();
    expect(screen.getByText("8f3a1b2c")).toBeInTheDocument();
    expect(h.commissions).toHaveBeenLastCalledWith("a1", expect.objectContaining({ from: expect.any(String), to: expect.any(String), page: 1, per_page: 10 }));

    fireEvent.click(screen.getByRole("button", { name: "Tutup" }));
    expect(screen.queryByText("Kursus Matematika")).toBeNull();
  });

  it("downloads the order CSV for that affiliator with the period", () => {
    h.download.mockResolvedValue(undefined);
    render(<ReportsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Lihat order" }));
    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));

    expect(h.download).toHaveBeenCalledWith("commissions", expect.objectContaining({ q: "", status: "" }), "a1");
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<ReportsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
