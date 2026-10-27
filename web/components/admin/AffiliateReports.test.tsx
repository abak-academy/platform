import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { toast } from "sonner";
import { AffiliatorsReport, ProductsReport, PromosReport, SchoolsReport, TrendReport } from "./AffiliateReports";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("@/components/admin/AffiliatorDialogs", () => ({
  AffiliatorDetailDialog: ({ id }: { id: string | null }) => (id ? <div role="dialog">detail {id}</div> : null),
}));

const reportCalls: { name: string; params: Record<string, unknown> }[] = [];
let reports: Record<string, { data?: { data: unknown[]; total: number }; isLoading: boolean; isError: boolean; refetch: () => void }>;
let trend: { data?: unknown[]; isLoading: boolean; isError: boolean; refetch: () => void };
const download = vi.fn();
vi.mock("@/lib/hooks/admin-affiliators", () => ({
  useAffiliateReport: (name: string, params: Record<string, unknown>) => {
    reportCalls.push({ name, params });
    return reports[name];
  },
  useAffiliateTrend: () => trend,
  downloadAffiliateReportCsv: (...a: unknown[]) => download(...a),
}));

const period = { from: "2026-10-01", to: "2026-10-31" };
const last = (name: string) => reportCalls.filter((c) => c.name === name).at(-1)!.params;
const ok = (data: unknown[], total = data.length) => ({ data: { data, total }, isLoading: false, isError: false, refetch: vi.fn() });

describe("affiliate reports", () => {
  beforeEach(() => {
    reportCalls.length = 0;
    download.mockReset().mockResolvedValue(undefined);
    vi.mocked(toast.error).mockReset();
    trend = { data: [], isLoading: false, isError: false, refetch: vi.fn() };
    reports = {
      affiliators: ok([
        { affiliator_id: "a1", name: "Andi", email: "andi@example.com", status: "active", school_count: 2, student_count: 40, paid_orders: 3, paid_units: 7, paid_amount: 900000, discount_total: 50000, pending_orders: 1, pending_amount: 75000, expired_orders: 2, orders_in_period: 6, last_order_at: "2026-10-20T03:00:00Z" },
        { affiliator_id: "a2", name: "Bela", email: "bela@example.com", status: "deactivated", school_count: 1, student_count: 5, paid_orders: 0, paid_units: 0, paid_amount: 0, discount_total: 0, pending_orders: 0, pending_amount: 0, expired_orders: 0, orders_in_period: 0, last_order_at: null },
      ], 57),
      schools: ok([{ school_id: "s1", school_name: "SMAN 1", affiliator_count: 2, student_count: 100, covered_students: 60, unpurchased_students: 40, paid_orders: 3, paid_units: 60, paid_amount: 1800000, discount_total: 200000 }]),
      products: ok([{ school_id: "s1", school_name: "SMAN 1", product_id: "p1", product_name: "Try Out UTBK", product_type: "exam", units: 25, orders: 2 }]),
      promos: ok([
        { code: "AFF10", affiliator_only: true, discount_percent: 10, discount_amount: null, min_qty: 2, min_order_amount: null, starts_at: null, expires_at: null, max_uses: 100, used_count: 12, active_now: true, orders: 4, discount_total: 80000 },
        { code: "PUBLIK", affiliator_only: false, discount_percent: null, discount_amount: 20000, min_qty: null, min_order_amount: null, starts_at: "2026-01-01T00:00:00Z", expires_at: "2026-02-01T00:00:00Z", max_uses: null, used_count: 3, active_now: false, orders: 1, discount_total: 20000 },
      ]),
    };
  });

  describe("per affiliator", () => {
    it("shows sales, backlog, expired and last order for each affiliator", () => {
      render(<AffiliatorsReport period={period} />);

      const row = screen.getByText("Andi").closest("tr")!;
      expect(within(row).getByText("Rp900.000")).toBeInTheDocument();
      expect(within(row).getByText("1 · Rp75.000")).toBeInTheDocument();
      expect(within(row).getByText("Aktif")).toBeInTheDocument();
      expect(within(row).getByText("20 Okt 2026")).toBeInTheDocument();
      expect(within(screen.getByText("Bela").closest("tr")!).getByText("-")).toBeInTheDocument();
    });

    it("asks for the period, page size and first page, and shows the total", () => {
      render(<AffiliatorsReport period={period} />);

      expect(last("affiliators")).toMatchObject({ from: "2026-10-01", to: "2026-10-31", limit: 20, offset: 0 });
      expect(screen.getByTestId("pager-range")).toHaveTextContent("1–2 dari 57");
    });

    it("pages by offset and returns to page one when searching", () => {
      render(<AffiliatorsReport period={period} />);
      fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
      expect(last("affiliators")).toMatchObject({ offset: 20 });

      fireEvent.change(screen.getByRole("searchbox", { name: "Cari nama atau email" }), { target: { value: "bela" } });
      expect(last("affiliators")).toMatchObject({ q: "bela", offset: 0 });
    });

    it("filters to affiliators with no activity", () => {
      render(<AffiliatorsReport period={period} />);

      fireEvent.click(screen.getByLabelText("Hanya yang tanpa aktivitas"));

      expect(last("affiliators")).toMatchObject({ inactive: true, offset: 0 });
    });

    it("opens the detail of one affiliator", () => {
      render(<AffiliatorsReport period={period} />);

      fireEvent.click(within(screen.getByText("Bela").closest("tr")!).getByRole("button", { name: "Detail" }));

      expect(screen.getByRole("dialog")).toHaveTextContent("detail a2");
    });

    it("exports the whole report for the same period, search and flag, without paging", async () => {
      render(<AffiliatorsReport period={period} />);
      fireEvent.click(screen.getByLabelText("Hanya yang tanpa aktivitas"));
      fireEvent.click(screen.getByRole("button", { name: "Ekspor CSV" }));

      await waitFor(() => expect(download).toHaveBeenCalledWith("affiliators", { from: "2026-10-01", to: "2026-10-31", q: undefined, inactive: true }));
    });

    it("tells the admin when the download fails", async () => {
      download.mockRejectedValue(new Error("403"));
      render(<AffiliatorsReport period={period} />);
      fireEvent.click(screen.getByRole("button", { name: "Ekspor CSV" }));

      await waitFor(() => expect(toast.error).toHaveBeenCalledWith("Gagal mengunduh laporan."));
    });
  });

  describe("per school", () => {
    it("shows coverage next to sales and explains the different time bases", () => {
      render(<SchoolsReport period={period} />);

      const cells = within(screen.getByText("SMAN 1").closest("tr")!).getAllByRole("cell").map((c) => c.textContent);
      // school, affiliators, students, covered, not covered, orders, units, amount, discount
      expect(cells).toEqual(["SMAN 1", "2", "100", "60", "40", "3", "60", "Rp1.800.000", "Rp200.000"]);
      expect(screen.getByText(/dihitung dari seluruh waktu/)).toBeInTheDocument();
    });

    it("searches by school name", () => {
      render(<SchoolsReport period={period} />);

      fireEvent.change(screen.getByRole("searchbox", { name: "Cari sekolah" }), { target: { value: "sman" } });

      expect(last("schools")).toMatchObject({ q: "sman" });
    });
  });

  describe("best sellers", () => {
    it("shows units and orders of each product per school with a readable type", () => {
      render(<ProductsReport period={period} />);

      const row = screen.getByText("Try Out UTBK").closest("tr")!;
      expect(within(row).getByText("Ujian")).toBeInTheDocument();
      expect(within(row).getByText("25")).toBeInTheDocument();
      expect(within(row).getByText("SMAN 1")).toBeInTheDocument();
    });
  });

  describe("promo usage", () => {
    it("shows scope, discount, validity, state, lifetime use and the period's use", () => {
      render(<PromosReport period={period} />);

      const cells = (code: string) => within(screen.getByText(code).closest("tr")!).getAllByRole("cell").map((c) => c.textContent);
      // code, scope, discount, validity, state, use, orders in period, discount in period
      expect(cells("AFF10")).toEqual(["AFF10", "Affiliator", "10%", "Tanpa batas", "Aktif", "12 / 100", "4", "Rp80.000"]);
      const plain = cells("PUBLIK");
      expect(plain.slice(1, 3)).toEqual(["Publik", "Rp20.000"]);
      expect(plain.slice(4)).toEqual(["Tidak aktif", "3 / ∞", "1", "Rp20.000"]);
      expect(plain[3]).toMatch(/2026.*–.*2026/);
    });
  });

  describe("trend", () => {
    it("lists the months with orders, units, amount, discount and active affiliators", () => {
      trend = { data: [{ month: "2026-09", orders: 2, units: 10, amount: 500000, discount: 20000, active_affiliators: 2 }], isLoading: false, isError: false, refetch: vi.fn() };
      render(<TrendReport period={period} />);

      const row = screen.getByText("2026-09").closest("tr")!;
      expect(within(row).getByText("Rp500.000")).toBeInTheDocument();
      expect(within(row).getByText("Rp20.000")).toBeInTheDocument();
      expect(screen.getByText(/sampai 24 bulan/)).toBeInTheDocument();
    });

    it("has no pager (it is not paged) but can be exported", async () => {
      render(<TrendReport period={period} />);

      expect(screen.queryByTestId("pager-range")).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("button", { name: "Ekspor CSV" }));
      await waitFor(() => expect(download).toHaveBeenCalledWith("trend", period));
    });
  });

  it("explains an empty report and offers a retry when loading fails", () => {
    reports.products = ok([]);
    const { unmount } = render(<ProductsReport period={period} />);
    expect(screen.getByText("Belum ada pembelian pada periode ini.")).toBeInTheDocument();
    unmount();

    const refetch = vi.fn();
    reports.products = { data: undefined, isLoading: false, isError: true, refetch };
    render(<ProductsReport period={period} />);
    fireEvent.click(screen.getByRole("button", { name: "Muat Ulang" }));
    expect(refetch).toHaveBeenCalled();
  });
});
