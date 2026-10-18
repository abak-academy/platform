import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UnitSalesBlock } from "./UnitSalesBlock";
import { useUIStore } from "@/stores/ui";
import { explicitUnitRange, unitPeriodSearch } from "@/lib/unit-sales-period";
import type { PeriodPreset } from "@/components/admin/PeriodBar";
import type { UnitSalesResponse } from "@/lib/types";

const { useSales, download, retry, replace } = vi.hoisted(() => ({ useSales: vi.fn(), download: vi.fn(), retry: vi.fn(), replace: vi.fn() }));
vi.mock("@/lib/hooks/admin-unit-sales", () => ({ useAdminUnitSales: useSales, downloadUnitSalesCsv: download }));
const typeFigures = { paid_orders: 2, refund_orders: 1, paid_units: 7, refund_units: 3, net_units: 4 };
const productFigures = [
  { paid_orders: 2, refund_orders: 1, paid_units: 4, refund_units: 1, net_units: 3 },
  { paid_orders: 1, refund_orders: 0, paid_units: 3, refund_units: 2, net_units: 1 },
];
const data: UnitSalesResponse = { from: null, to: null, types: ["book", "merchandise", "medal", "course", "exam"].map((type) => ({
  type: type as UnitSalesResponse["types"][number]["type"],
  ...(type === "book" ? typeFigures : { paid_orders: 0, refund_orders: 0, paid_units: 0, refund_units: 0, net_units: 0 }),
  products: type === "book" ? [{ product_id: "p1", name: "Buku Contoh", ...productFigures[0] }, { product_id: "p2", name: "Buku Kedua", ...productFigures[1] }] : [],
})) };

let search = "";
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(search), useRouter: () => ({ replace }) }));

beforeEach(() => {
  search = "";
  vi.clearAllMocks();
  useUIStore.setState({ lang: "id" });
  useSales.mockReturnValue({ data, isLoading: false, isError: false, refetch: retry });
  download.mockResolvedValue(undefined);
});

afterEach(() => { vi.useRealTimers(); useUIStore.setState({ lang: "id" }); });

describe("UnitSalesBlock", () => {
  it("renders five units-only table rows with net units as the lead", () => {
    const { container } = render(<UnitSalesBlock />);
    expect(screen.getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual(["Tipe produk", "Unit dibayar", "Unit refund", "Unit neto"]);
    const rows = screen.getAllByRole("row").slice(1);
    expect(rows.map((row) => within(row).getByRole("link").getAttribute("aria-label"))).toEqual([
      "Lihat produk: Buku", "Lihat produk: Merchandise", "Lihat produk: Medali", "Lihat produk: Kursus", "Lihat produk: Ujian",
    ]);
    expect(within(rows[0]).getAllByRole("cell").map((cell) => cell.textContent)).toEqual(["Buku", "Unit dibayar7", "Unit refund3", "Unit neto4"]);
    expect(within(rows[0]).getByRole("cell", { name: "4" })).toHaveClass("font-bold", "text-title-large", "text-right", "tabular-nums");
    expect(rows[1]).toHaveTextContent("MerchandiseUnit dibayar0Unit refund0Unit neto0");
    expect(container.textContent).not.toMatch(/Order dibayar|Order refund|rupiah|Rp\b|IDR|revenue|harga/i);
  });

  it("stretches a native link across the row and keeps Enter activation", async () => {
    render(<UnitSalesBlock />);
    const link = screen.getByRole("link", { name: "Lihat produk: Buku" });
    expect(link).toHaveClass("after:absolute", "after:inset-0");
    expect(link.closest("tr")).toHaveClass("relative", "focus-within:outline");
    expect(link).toHaveAttribute("href", expect.stringMatching(/^\/admin\/store\/units\/book\?preset=30d&from=.*&to=.*/));
    const activate = vi.fn((event: Event) => event.preventDefault());
    link.addEventListener("click", activate);
    link.focus();
    await userEvent.keyboard("{Enter}");
    expect(activate).toHaveBeenCalledOnce();
  });

  it.each(["7d", "custom", "all_time"])("restores %s from URL and carries it in all row links", (preset) => {
    const range = preset === "7d" ? explicitUnitRange("7d", {}) : preset === "custom" ? { from: "2026-01-01", to: "2026-01-31" } : {};
    search = unitPeriodSearch({ preset: preset as PeriodPreset, range });
    render(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: preset === "7d" ? "7 hari" : preset === "custom" ? "Custom" : "Semua waktu" })).toHaveAttribute("aria-pressed", "true");
    for (const link of screen.getAllByRole("link")) expect(link.getAttribute("href")).toContain(`?${search}`);
    expect(useSales.mock.lastCall![0]).toEqual(range);
  });

  it("shows a preset whose URL dates are not that preset's current window as Custom", () => {
    search = "preset=30d&from=2026-01-01&to=2026-01-31";
    render(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "Custom" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "30 hari" })).toHaveAttribute("aria-pressed", "false");
    expect(useSales.mock.lastCall![0]).toEqual({ from: "2026-01-01", to: "2026-01-31" });
  });

  it.each(["", "preset=bad", "preset=7d&from=bad&to=2026-01-31", "preset=custom&from=2026-02-30&to=2026-03-01", "preset=custom&from=2026-02-10&to=2026-02-01", "preset=30d"])("defaults invalid/missing URL %s to explicit 30d", (params) => {
    search = params;
    render(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "30 hari" })).toHaveAttribute("aria-pressed", "true");
    expect(useSales.mock.lastCall![0]).toEqual({ from: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), to: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });
  });

  it("restores explicit 30d when external navigation clears the URL", () => {
    search = "preset=all_time";
    const view = render(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "Semua waktu" })).toHaveAttribute("aria-pressed", "true");
    search = "";
    view.rerender(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "30 hari" })).toHaveAttribute("aria-pressed", "true");
    expect(useSales.mock.lastCall![0]).toEqual(explicitUnitRange("30d", {}));
    expect(useSales.mock.lastCall![0].from).toBeTruthy();
    expect(useSales.mock.lastCall![0].to).toBeTruthy();
    expect(replace).not.toHaveBeenCalled();
  });

  it("keeps its applied range when its own URL catches up after external navigation", async () => {
    search = "preset=all_time";
    const view = render(<UnitSalesBlock />);
    search = "";
    view.rerender(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "30 hari" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "7 hari" }));
    expect(screen.getByRole("button", { name: "7 hari" })).toHaveAttribute("aria-pressed", "true");
    const applied = useSales.mock.lastCall![0];
    expect(applied).toEqual(explicitUnitRange("7d", {}));
    useSales.mockClear();
    search = replace.mock.lastCall![0].split("?")[1];
    view.rerender(<UnitSalesBlock />);
    expect(screen.getByRole("button", { name: "7 hari" })).toHaveAttribute("aria-pressed", "true");
    expect(useSales).toHaveBeenCalledOnce();
    expect(useSales.mock.lastCall![0]).toBe(applied);
    expect(replace).toHaveBeenCalledOnce();
  });

  it("updates the URL and links when the period changes", async () => {
    render(<UnitSalesBlock />);
    await userEvent.click(screen.getByRole("button", { name: "Semua waktu" }));
    expect(screen.getByRole("link", { name: "Lihat produk: Buku" })).toHaveAttribute("href", "/admin/store/units/book?preset=all_time");
    expect(replace).toHaveBeenCalledWith("/admin/store?preset=all_time", { scroll: false });
  });
  it("changes the query range, sends no dates for all-time, and accepts custom dates", async () => {
    const user = userEvent.setup();
    render(<UnitSalesBlock />);
    const initial = useSales.mock.lastCall![0];
    expect(initial.from).toBeTruthy(); expect(initial.to).toBeTruthy();
    await user.click(screen.getByRole("button", { name: /semua waktu/i }));
    expect(useSales.mock.lastCall![0]).toEqual({});
    await user.click(screen.getByRole("button", { name: /custom/i }));
    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "2026-01-01" } });
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "2026-01-31" } });
    expect(useSales.mock.lastCall![0]).toEqual({ from: "2026-01-01", to: "2026-01-31" });
  });

  it("keeps the last applied period while custom dates are cleared or reversed", () => {
    render(<UnitSalesBlock />);
    fireEvent.click(screen.getByRole("button", { name: /custom/i }));
    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "" } });
    const openEnded = useSales.mock.lastCall![0];
    const appliedHref = screen.getByRole("link", { name: "Lihat produk: Buku" }).getAttribute("href");
    expect(openEnded.from).toBeFalsy();
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "" } });
    expect(useSales.mock.lastCall![0]).toEqual(openEnded);
    expect(screen.getByRole("link", { name: "Lihat produk: Buku" })).toHaveAttribute("href", appliedHref);
    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "2026-02-10" } });
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "2026-02-01" } });
    expect(useSales.mock.lastCall![0]).toEqual({ from: "2026-02-10", to: "" });
    expect(screen.getByRole("link", { name: "Lihat produk: Buku" })).toHaveAttribute("href", "/admin/store/units/book?preset=custom&from=2026-02-10");
  });

  it("renders English units-only labels", () => {
    useUIStore.setState({ lang: "en" });
    render(<UnitSalesBlock />);
    expect(screen.getByRole("link", { name: "View products: Books" })).toBeInTheDocument();
    expect(screen.getAllByRole("columnheader").map((cell) => cell.textContent)).toEqual(["Product type", "Paid units", "Refunded units", "Net units"]);
    expect(screen.getByText("Units are quantities, not order counts. Net units = paid units − refunded units.")).toBeInTheDocument();
  });

  it("shows loading then error with retry", async () => {
    useSales.mockReturnValue({ isLoading: true });
    const view = render(<UnitSalesBlock />);
    expect(screen.getByRole("status")).toHaveTextContent(/memuat/i);
    useSales.mockReturnValue({ isError: true, refetch: retry });
    view.rerender(<UnitSalesBlock />);
    expect(screen.getByRole("alert")).toHaveTextContent(/gagal/i);
    await userEvent.click(screen.getByRole("button", { name: /coba lagi/i }));
    expect(retry).toHaveBeenCalled();
  });

  it.each([
    ["7 hari", "2026-07-28"], ["30 hari", "2026-07-05"],
    ["90 hari", "2026-05-06"], ["Bulan ini", "2026-08-01"],
  ])("sends explicit WIB dates for %s", (label, from) => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-02T17:30:00Z"));
    render(<UnitSalesBlock />);
    fireEvent.click(screen.getByRole("button", { name: label }));
    expect(useSales.mock.lastCall![0]).toEqual({ from, to: "2026-08-03" });
    expect(screen.getByRole("link", { name: "Lihat produk: Buku" })).toHaveAttribute("href", expect.stringContaining(`from=${from}&to=2026-08-03`));
  });

});
