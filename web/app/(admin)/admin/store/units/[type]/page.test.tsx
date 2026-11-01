import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import UnitSalesDetailPage from "./page";
import { explicitUnitRange, unitPeriodSearch } from "@/lib/unit-sales-period";
import type { PeriodPreset } from "@/components/admin/PeriodBar";
import { useUIStore } from "@/stores/ui";
import type { UnitSalesResponse } from "@/lib/types";

const { sales, download, retry } = vi.hoisted(() => ({ sales: vi.fn(), download: vi.fn(), retry: vi.fn() }));
let search = "";
let role = "admin_store";
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(search) }));
vi.mock("@/lib/hooks/admin-unit-sales", () => ({ useAdminUnitSales: sales, downloadUnitSalesCsv: download }));
vi.mock("@/stores/auth", () => ({ useAuthStore: (selector: (state: unknown) => unknown) => selector({ token: "t", user: { role } }) }));
vi.mock("@/lib/hooks/auth", () => ({ useMe: () => ({ data: null }) }));

const data: UnitSalesResponse = { from: "2026-01-01", to: "2026-01-31", types: [
  { type: "book", paid_orders: 11, refund_orders: 12, paid_units: 21, refund_units: 24, net_units: -3, products: [
    { product_id: "p1", name: "Produk A", paid_orders: 8, refund_orders: 9, paid_units: 16, refund_units: 18, net_units: -2 },
    { product_id: "p2", name: "Produk B", paid_orders: 6, refund_orders: 7, paid_units: 5, refund_units: 6, net_units: -1 },
  ] },
  ...(["merchandise", "medal", "course", "exam"] as const).map((type) => ({ type, paid_orders: 0, refund_orders: 0, paid_units: 0, refund_units: 0, net_units: 0, products: [] })),
] };

beforeEach(() => {
  vi.clearAllMocks();
  vi.setSystemTime(new Date("2026-08-02T17:30:00Z"));
  useUIStore.setState({ lang: "en" });
  role = "admin_store";
  search = "preset=custom&from=2026-01-01&to=2026-01-31";
  sales.mockReturnValue({ data, isLoading: false, isError: false, refetch: retry });
  download.mockResolvedValue(undefined);
});
afterEach(() => { vi.useRealTimers(); useUIStore.setState({ lang: "id" }); });

async function renderPage(type = "book") {
  await act(async () => { render(<UnitSalesDetailPage params={Promise.resolve({ type })} />); });
}

describe("unit sales detail", () => {
  it("separates quantities and distinct orders and preserves negative net", async () => {
    await renderPage();
    const units = screen.getByRole("region", { name: "Unit quantities" });
    expect(units).toHaveTextContent("-3Net unitsPaid units21Refunded units24");
    const orders = screen.getByRole("region", { name: "Order context" });
    expect(orders).toHaveTextContent("Paid orders11Refunded orders12");
    expect(orders).toHaveTextContent(/not summed from product rows/);
    expect(screen.getAllByRole("columnheader").map((th) => th.textContent)).toEqual(["Product", "Paid units", "Refunded units", "Net units"]);
    const rows = screen.getAllByRole("row").slice(1);
    expect(rows.map((row) => row.textContent)).toEqual(["Produk APaid units16Refunded units18Net units-2", "Produk BPaid units5Refunded units6Net units-1"]);
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", `/admin/store?${search}`);
    expect(screen.getByText("Custom · 2026-01-01 – 2026-01-31 · WIB")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/Rp\b|IDR|rupiah|currency|buyer|order.?id|harga/i);
  });

  it.each([
    ["custom", { from: "2026-01-01", to: "2026-01-31" }],
    ["all_time", {}],
  ])("downloads %s with the URL range", async (preset, range) => {
    search = preset === "all_time" ? "preset=all_time" : search;
    await renderPage();
    await userEvent.click(screen.getByRole("button", { name: "Download CSV" }));
    expect(download).toHaveBeenCalledWith("book", range);
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", `/admin/store?${search}`);
    if (preset === "all_time") expect(screen.getByText("All time")).toBeInTheDocument();
  });

  it.each([["7d", "7 days"], ["30d", "30 days"], ["90d", "90 days"], ["this_month", "This month"]])("keeps %s when the URL dates are that preset's current window", async (preset, label) => {
    const range = explicitUnitRange(preset as PeriodPreset, {});
    search = unitPeriodSearch({ preset: preset as PeriodPreset, range });
    await renderPage();
    expect(screen.getByText(`${label} · ${range.from} – ${range.to} · WIB`)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", `/admin/store?${search}`);
    expect(sales).toHaveBeenCalledWith(range);
  });

  it.each(["7d", "30d", "90d", "this_month"])("labels %s with stale URL dates as Custom", async (preset) => {
    search = `preset=${preset}&from=2026-01-01&to=2026-01-31`;
    await renderPage();
    expect(screen.getByText("Custom · 2026-01-01 – 2026-01-31 · WIB")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", "/admin/store?preset=custom&from=2026-01-01&to=2026-01-31");
    expect(sales).toHaveBeenCalledWith({ from: "2026-01-01", to: "2026-01-31" });
  });

  it.each(["from", "to"])("preserves custom ranges with only %s", async (bound) => {
    search = `preset=custom&${bound}=2026-01-01`;
    await renderPage();
    await userEvent.click(screen.getByRole("button", { name: "Download CSV" }));
    expect(download).toHaveBeenCalledWith("book", { [bound]: "2026-01-01" });
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", `/admin/store?${search}`);
  });

  it("disables CSV while downloading and reports failures", async () => {
    let rejectDownload!: (reason: Error) => void;
    download.mockReturnValue(new Promise((_, reject) => { rejectDownload = reject; }));
    await renderPage();
    await userEvent.click(screen.getByRole("button", { name: "Download CSV" }));
    expect(screen.getByRole("button", { name: "Downloading…" })).toBeDisabled();
    await act(async () => { rejectDownload(new Error("failed")); });
    expect(await screen.findByRole("alert")).toHaveTextContent("Failed to download CSV.");
    expect(screen.getByRole("button", { name: "Download CSV" })).toBeEnabled();
  });

  it("shows empty products and disables download", async () => {
    await renderPage("medal");
    expect(screen.getByText("No product activity in this period.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Download CSV" })).toBeDisabled();
  });

  it("does not request an unknown type and offers Back", async () => {
    await renderPage("unknown");
    expect(screen.getByText("Product type not found.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", `/admin/store?${search}`);
    expect(sales).not.toHaveBeenCalled();
  });

  it.each(["", "preset=nope", "preset=7d", "preset=custom&from=2026-02-30", "preset=custom&from=2026-02-10&to=2026-02-01"])("defaults invalid/missing period %s to 30d in WIB", async (params) => {
    search = params;
    await renderPage();
    expect(screen.getByText("30 days · 2026-07-05 – 2026-08-03 · WIB")).toBeInTheDocument();
    expect(sales).toHaveBeenCalledWith({ from: "2026-07-05", to: "2026-08-03" });
    expect(screen.getByRole("link", { name: /back to summary/i })).toHaveAttribute("href", "/admin/store?preset=30d&from=2026-07-05&to=2026-08-03");
  });

  it.each(["loading", "error"])("shows %s and retry", async (state) => {
    sales.mockReturnValue({ isLoading: state === "loading", isError: state === "error", refetch: retry });
    await renderPage();
    expect(screen.getByRole(state === "loading" ? "status" : "alert")).toHaveTextContent(state === "loading" ? "Loading units sold…" : "Failed to load units sold.");
    if (state === "error") {
      await userEvent.click(screen.getByRole("button", { name: /retry/i }));
      expect(retry).toHaveBeenCalledOnce();
    }
  });

  it.each(["admin_store", "super_admin", "admin_exam", "admin_school", "student"])("gates %s with the Store capability", async (currentRole) => {
    role = currentRole;
    await renderPage();
    if (["admin_store", "super_admin"].includes(role)) expect(screen.getByRole("heading", { name: "Books" })).toBeInTheDocument();
    else {
      expect(screen.getByTestId("no-access")).toBeInTheDocument();
      expect(sales).not.toHaveBeenCalled();
    }
  });

  it("renders Indonesian labels", async () => {
    useUIStore.setState({ lang: "id" });
    await renderPage();
    expect(screen.getByRole("region", { name: "Kuantitas unit" })).toHaveTextContent("-3");
    expect(screen.getByRole("region", { name: "Konteks order" })).toHaveTextContent("Order dibayar11Order refund12");
    expect(screen.getByRole("button", { name: "Unduh CSV" })).toBeEnabled();
  });
});
