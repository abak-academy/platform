import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, within, waitFor, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { UnitSalesBlock } from "./UnitSalesBlock";
import { useUIStore } from "@/stores/ui";
import type { UnitSalesResponse } from "@/lib/types";

const { useSales, download, retry } = vi.hoisted(() => ({ useSales: vi.fn(), download: vi.fn(), retry: vi.fn() }));
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

beforeEach(() => {
  vi.clearAllMocks();
  useUIStore.setState({ lang: "id" });
  useSales.mockReturnValue({ data, isLoading: false, isError: false, refetch: retry });
  download.mockResolvedValue(undefined);
});

afterEach(() => { vi.useRealTimers(); useUIStore.setState({ lang: "id" }); });

function bookRow() { return screen.getByRole("button", { name: /lihat produk: buku/i }); }

describe("UnitSalesBlock", () => {
  it("renders five whole native rows with figures and no currency", () => {
    const { container } = render(<UnitSalesBlock />);
    const rows = screen.getAllByRole("button", { name: /lihat produk:/i });
    expect(rows.map((row) => row.getAttribute("aria-label"))).toEqual([
      "Lihat produk: Buku", "Lihat produk: Merchandise", "Lihat produk: Medali", "Lihat produk: Kursus", "Lihat produk: Ujian",
    ]);
    for (const row of rows) { expect(row.tagName).toBe("BUTTON"); expect(row).toHaveAttribute("type", "button"); }
    expect(document.getElementById(rows[0].getAttribute("aria-describedby")!)).toHaveTextContent("Order dibayar2Order refund1Unit dibayar7Unit refund3Unit neto4");
    expect(container.textContent).not.toMatch(/rupiah|Rp\b|IDR|revenue|harga/i);
  });

  it.each(["click", "Enter", "Space"])("opens with %s, shows matching totals, and restores focus on Escape", async (method) => {
    const user = userEvent.setup();
    render(<UnitSalesBlock />);
    const row = bookRow();
    if (method === "click") await user.click(row);
    else { row.focus(); await user.keyboard(method === "Enter" ? "{Enter}" : " "); }
    const modal = await screen.findByRole("dialog");
    expect(within(modal).getByText("Buku Contoh")).toBeInTheDocument();
    const total = within(modal).getByTestId("unit-sales-total");
    expect(total).toHaveTextContent("Order dibayar2Order refund1Unit dibayar7Unit refund3Unit neto4");
    expect(within(modal).getByText("Buku Kedua")).toBeInTheDocument();
    expect(modal.textContent).not.toMatch(/rupiah|Rp\b|IDR|revenue|harga/i);
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(row).toHaveFocus();
  });

  it("downloads the opened type with the current explicit period", async () => {
    const user = userEvent.setup();
    render(<UnitSalesBlock />);
    await user.click(screen.getByRole("button", { name: /7 hari/i }));
    const range = useSales.mock.lastCall![0];
    expect(range).toEqual({ from: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), to: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/) });
    await user.click(bookRow());
    await user.click(screen.getByRole("button", { name: /unduh csv/i }));
    expect(download).toHaveBeenCalledWith("book", range);
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
    expect(openEnded.from).toBeFalsy();
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "" } });
    expect(useSales.mock.lastCall![0]).toEqual(openEnded);
    fireEvent.change(screen.getByLabelText("Dari tanggal"), { target: { value: "2026-02-10" } });
    fireEvent.change(screen.getByLabelText("Sampai tanggal"), { target: { value: "2026-02-01" } });
    expect(useSales.mock.lastCall![0]).toEqual({ from: "2026-02-10", to: "" });
  });

  it("shows empty products and disables download; close restores row focus", async () => {
    const user = userEvent.setup();
    render(<UnitSalesBlock />);
    const row = screen.getByRole("button", { name: /lihat produk: medali/i });
    await user.click(row);
    expect(screen.getByText(/belum ada aktivitas produk/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /unduh csv/i })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: /tutup/i }));
    await waitFor(() => expect(row).toHaveFocus());
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
  });

  it("downloads all-time without dates", async () => {
    render(<UnitSalesBlock />);
    await userEvent.click(screen.getByRole("button", { name: "Semua waktu" }));
    await userEvent.click(bookRow());
    await userEvent.click(screen.getByRole("button", { name: "Unduh CSV" }));
    expect(download).toHaveBeenCalledWith("book", {});
  });

  it("renders English labels and figures", async () => {
    useUIStore.setState({ lang: "en" });
    render(<UnitSalesBlock />);
    const row = screen.getByRole("button", { name: "View products: Books" });
    expect(row).toHaveTextContent("Paid orders2");
    expect(row).toHaveTextContent("Paid units7");
    await userEvent.click(row);
    expect(screen.getByRole("button", { name: "Download CSV" })).toBeEnabled();
    expect(screen.getByRole("dialog").textContent).not.toMatch(/rupiah|Rp\b|IDR|revenue|harga/i);
  });

  it("reports a failed download", async () => {
    download.mockRejectedValue(new Error("failed"));
    render(<UnitSalesBlock />);
    await userEvent.click(bookRow());
    await userEvent.click(screen.getByRole("button", { name: /unduh csv/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/gagal/i);
  });
});
