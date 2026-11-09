import { render, screen, fireEvent, within } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import ReportsPage from "./page";

const h = vi.hoisted(() => ({
  summary: vi.fn(),
  liability: vi.fn(),
  capability: vi.fn(),
  orders: vi.fn(),
  download: vi.fn(),
  partner: vi.fn(),
  downloadPartner: vi.fn(),
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
  useAdminAffiliateReportOrders: h.orders,
  downloadAffiliateCsv: h.download,
  useAdminPartnerReport: h.partner,
  downloadPartnerReportCsv: h.downloadPartner,
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

const order = {
  id: "c1",
  order_id: "8f3a1b2c-0000-0000-0000-000000000000",
  affiliator_name: "Budi Santoso",
  affiliator_code: "BUDI1234",
  student_name: "Sari Wulandari",
  product_name: "Kursus Matematika",
  product_type: "course",
  base_amount: 1000000,
  percentage: 10,
  amount: 100000,
  status: "available",
  paid_at: "2026-10-02T00:00:00Z",
  created_at: "2026-10-02T00:00:00Z",
};

describe("AdminAffiliateReportsPage", () => {
  beforeEach(() => {
    Object.values(h).forEach((m) => "mockReset" in m && m.mockReset());
    h.search.value = "";
    h.capability.mockReturnValue(true);
    h.summary.mockReturnValue({
      isLoading: false,
      isError: false,
      data: { data: [row], totals: { ...row, affiliator_name: "", affiliator_code: "" }, total: 1, page: 1, per_page: 20 },
    });
    h.orders.mockReturnValue({ isLoading: false, isError: false, data: { data: [order], total: 1, page: 1, per_page: 20 } });
    h.liability.mockReturnValue({
      data: { by_status: { pending: 1000, available: 40000, paid: 5000, held: 0, cancelled: 2000 }, payout_liability: 25000 },
    });
  });

  it("excludes cancelled commission from the row total", () => {
    render(<ReportsPage />);

    expect(screen.getByText("Rp50.000", { selector: "td" })).toBeInTheDocument();
    expect(screen.queryByText("Rp149.000")).toBeNull();
  });

  it("shows the payout liability and every commission status as stat cards", () => {
    render(<ReportsPage />);

    expect(screen.getByText("Rp25.000")).toBeInTheDocument();
    expect(screen.getByText("Rp40.000")).toBeInTheDocument();
    expect(screen.getByText("Rp2.000")).toBeInTheDocument();
  });

  it("puts the period in the filter popover and defaults the report to the current month", () => {
    render(<ReportsPage />);

    expect(screen.queryByLabelText("Dari tanggal")).toBeNull();
    const args = h.summary.mock.lastCall![0];
    expect(args.from).toMatch(/^\d{4}-\d{2}-01$/);
    expect(args.to).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(h.orders).not.toHaveBeenCalled();
  });

  it("uses the period, search and level chosen in the URL", () => {
    h.search.value = "from=2026-09-01&to=2026-09-30&q=budi&type=gold&page=2";
    render(<ReportsPage />);

    expect(h.summary).toHaveBeenLastCalledWith({ from: "2026-09-01", to: "2026-09-30", q: "budi", level: "gold", page: 2, per_page: 20 });
  });

  it("switches tabs through the URL", () => {
    render(<ReportsPage />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Detail order" }));

    expect(h.replace).toHaveBeenCalledWith("/admin/affiliates/reports?tab=orders", { scroll: false });
  });

  it("jumps from a summary row to that affiliate's orders", () => {
    h.search.value = "from=2026-09-01";
    render(<ReportsPage />);

    fireEvent.click(screen.getByRole("button", { name: "Lihat order" }));

    expect(h.replace).toHaveBeenCalledWith("/admin/affiliates/reports?tab=orders&q=BUDI1234&from=2026-09-01", { scroll: false });
  });

  it("lists every affiliate's orders on the detail tab and downloads them with the filters", () => {
    h.search.value = "tab=orders&status=available&q=sari";
    h.download.mockResolvedValue(undefined);
    render(<ReportsPage />);

    expect(h.summary).not.toHaveBeenCalled();
    expect(h.orders).toHaveBeenLastCalledWith({ q: "sari", status: "available", type: "", from: "", to: "", page: 1, per_page: 20 });
    expect(screen.getByText("Sari Wulandari")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "#00000000" })).toHaveAttribute("href", "/admin/orders?order=8f3a1b2c-0000-0000-0000-000000000000");
    expect(screen.getByText("BUDI1234")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));
    expect(h.download).toHaveBeenCalledWith("report-orders", { q: "sari", status: "available", type: "", from: "", to: "" });
  });

  describe("Mitra tab", () => {
    const partnerRow = {
      order_id: "9a9a9a9a-0000-0000-0000-000000001234",
      created_at: "2026-10-05T00:00:00Z",
      status: "paid",
      partner_id: "m1",
      partner_name: "Sekolah Mitra Jaya",
      products: "Tryout UTBK",
      seats: 2,
      students: "Ani; Budi",
      student_list: ["Ani", "Budi"],
      promo_code: "DEMO-ORDER10",
      subtotal: 200000,
      discount: 20000,
      total: 180000,
    };

    beforeEach(() => {
      h.partner.mockReturnValue({
        isLoading: false,
        isError: false,
        data: { data: [partnerRow], total: 1, page: 1, per_page: 20, summary: { orders: 3, paid_orders: 2, seats: 5, paid_total: 360000, paid_discount: 40000 } },
      });
    });

    it("switches to it through the URL", () => {
      render(<ReportsPage />);
      fireEvent.mouseDown(screen.getByRole("tab", { name: "Mitra" }));
      expect(h.replace).toHaveBeenCalledWith("/admin/affiliates/reports?tab=partner", { scroll: false });
    });

    it("shows what each Mitra bought, for which students and with which voucher, plus the totals", () => {
      h.search.value = "tab=partner&status=paid&q=utbk";
      render(<ReportsPage />);

      expect(h.partner).toHaveBeenLastCalledWith({ q: "utbk", status: "paid", from: "", to: "", page: 1, per_page: 20 });
      expect(h.summary).not.toHaveBeenCalled();
      expect(screen.getByText("Sekolah Mitra Jaya")).toBeInTheDocument();
      expect(screen.getByText("Tryout UTBK")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: /lihat daftar/i })).toHaveTextContent("2 siswa");
      expect(screen.getByText("DEMO-ORDER10")).toBeInTheDocument();
      expect(screen.getByText("Rp360.000")).toBeInTheDocument();
      expect(screen.getByRole("link", { name: "#00001234" })).toHaveAttribute("href", "/admin/orders?order=9a9a9a9a-0000-0000-0000-000000001234");
    });

    it("lists the students of a purchase in a popup", () => {
      h.search.value = "tab=partner";
      render(<ReportsPage />);

      expect(screen.queryByText("Siswa yang dibelikan")).toBeNull();
      fireEvent.click(screen.getByRole("button", { name: /lihat daftar/i }));

      expect(screen.getByText("Siswa yang dibelikan")).toBeInTheDocument();
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByText("Ani")).toBeInTheDocument();
      expect(within(dialog).getByText("Budi")).toBeInTheDocument();
    });

    it("puts the Mitra totals above the tabs, in place of the Affiliator summary", () => {
      h.search.value = "tab=partner";
      render(<ReportsPage />);

      const totals = screen.getByText("Rp360.000");
      const tabs = screen.getByRole("tab", { name: "Mitra" });
      expect(Boolean(totals.compareDocumentPosition(tabs) & Node.DOCUMENT_POSITION_FOLLOWING)).toBe(true);
      expect(screen.queryByText("Rp25.000")).toBeNull();
    });

    it("downloads the purchases with the filters applied", () => {
      h.search.value = "tab=partner&status=paid&from=2026-10-01";
      h.downloadPartner.mockResolvedValue(undefined);
      render(<ReportsPage />);

      fireEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));
      expect(h.downloadPartner).toHaveBeenCalledWith({ q: "", status: "paid", from: "2026-10-01", to: "" });
    });
  });

  it("is blocked without the capability", () => {
    h.capability.mockReturnValue(false);
    render(<ReportsPage />);

    expect(screen.getByTestId("no-access")).toBeInTheDocument();
  });
});
