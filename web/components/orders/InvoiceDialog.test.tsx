import { readFileSync } from "node:fs";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { InvoiceDialog } from "./InvoiceDialog";
import { useAuthStore } from "@/stores/auth";
import { useUIStore } from "@/stores/ui";
import type { Order } from "@/lib/types";

const { savePageAsPdf, toastSuccess, toastError } = vi.hoisted(() => ({
  savePageAsPdf: vi.fn(),
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}));
vi.mock("@/lib/capture-pdf", () => ({ savePageAsPdf }));
vi.mock("sonner", () => ({ toast: { success: toastSuccess, error: toastError, info: vi.fn() } }));

const DIGITAL: Order = {
  id: "0b6c1f3e-aaaa-bbbb-cccc-1234567890ab",
  student_id: "s1",
  status: "paid",
  subtotal: 150000,
  discount: 20000,
  shipping_cost: 0,
  total: 130000,
  payment_method: "gopay",
  paid_at: "2026-11-20T03:00:00Z",
  created_at: "2026-11-20T02:00:00Z",
  items: [
    { id: "i1", order_id: "o1", product_id: "p1", product_type: "course", name: "Kursus Matematika", unit_price: 75000, qty: 2, jumlah: 150000 },
  ],
};

const PHYSICAL = {
  ...DIGITAL,
  id: "ffff0000-aaaa-bbbb-cccc-99999999aaaa",
  status: "payment_pending",
  paid_at: null,
  shipped_at: null,
  payment_method: "",
  discount: 0,
  shipping_cost: 18000,
  subtotal: 100000,
  total: 118000,
  shipping_address: { penerima: "Penerima Uji", telepon: "080000000000", alamat: "Jl. Contoh 1", kode_pos: "40111" },
  selected_courier: "JNE",
  selected_service: "REG",
  tracking_number: "",
  items: [{ id: "i2", order_id: "o2", product_id: "p2", product_type: "book", name: "Buku Latihan", unit_price: 100000, qty: 1, jumlah: 100000 }],
} as unknown as Order;

function show(order: Order) {
  return render(<InvoiceDialog order={order} open onOpenChange={() => {}} />);
}

beforeEach(() => {
  vi.clearAllMocks();
  useUIStore.setState({ lang: "id" });
  useAuthStore.setState({ user: { id: "s1", name: "Siswa Uji", email: "siswa@example.test" } });
  window.print = vi.fn();
});

describe("InvoiceDialog", () => {
  it("renders the paid digital invoice fields", () => {
    show(DIGITAL);
    const paper = document.querySelector(".invoice-paper") as HTMLElement;
    expect(paper).toBeTruthy();
    expect(paper).toHaveTextContent("Abak Academy");
    expect(paper).toHaveTextContent(window.location.host);
    expect(paper).toHaveTextContent("INVOICE");
    expect(paper).toHaveTextContent(`INV/${DIGITAL.id}`);
    expect(paper).toHaveTextContent("Ditagihkan kepada");
    expect(paper).toHaveTextContent("Siswa Uji");
    expect(paper).toHaveTextContent("siswa@example.test");
    expect(paper).toHaveTextContent("LUNAS");
    expect(paper).not.toHaveTextContent("BELUM DIBAYAR");
    expect(paper).toHaveTextContent("Kursus Matematika");
    expect(paper).toHaveTextContent("Rp150.000");
    expect(paper).toHaveTextContent("Diskon");
    expect(paper).toHaveTextContent("Rp130.000");
    expect(paper).toHaveTextContent(/Metode pembayaran: GoPay · /);
    expect(paper).toHaveTextContent("Invoice ini berfungsi sebagai bukti pemesanan dan/atau pembelian.");
    expect(paper).not.toHaveTextContent("Ongkos kirim");
    expect(paper).not.toHaveTextContent("Rincian pengiriman");
  });

  it("stamps an unpaid physical order and shows shipping with the address", () => {
    show(PHYSICAL);
    const paper = document.querySelector(".invoice-paper") as HTMLElement;
    expect(paper).toHaveTextContent("BELUM DIBAYAR");
    expect(paper).not.toHaveTextContent("LUNAS");
    expect(paper).toHaveTextContent("Ongkos kirim");
    expect(paper).toHaveTextContent("Rp18.000");
    expect(paper).not.toHaveTextContent("Diskon");
  });

  it("lists recipient, address, courier and a pending resi for a physical order", () => {
    show(PHYSICAL);
    const ship = screen.getByText("Rincian pengiriman").closest("section") as HTMLElement;
    expect(ship).toHaveTextContent("PenerimaPenerima Uji · 080000000000");
    expect(ship).toHaveTextContent("AlamatJl. Contoh 1 40111");
    expect(ship).toHaveTextContent("KurirJNE · REG");
    expect(ship).toHaveTextContent("No. resiBelum tersedia");
  });

  it("shows the resi once the order has one", () => {
    show({ ...PHYSICAL, status: "shipped", tracking_number: "JNE0012345678" } as Order);
    const ship = screen.getByText("Rincian pengiriman").closest("section") as HTMLElement;
    expect(ship).toHaveTextContent("No. resiJNE0012345678");
    expect(ship).not.toHaveTextContent("Belum tersedia");
  });

  it("falls back to a dash when the payment method is an empty string", () => {
    show(PHYSICAL);
    expect(document.querySelector(".invoice-paper")).toHaveTextContent(/Metode pembayaran: —/);
  });

  it("formats dates in the active language in Jakarta time", () => {
    useUIStore.setState({ lang: "en" });
    show({ ...DIGITAL, created_at: "2026-11-30T20:00:00Z" });
    const paper = document.querySelector(".invoice-paper") as HTMLElement;
    expect(paper).toHaveTextContent("Dec 1, 2026");
    expect(paper).toHaveTextContent("This invoice serves as proof of order and/or purchase.");
  });

  it("scopes the A4 @page rule to the open dialog, not globals.css", () => {
    const a4Styles = () => [...document.querySelectorAll("style")].filter((el) => el.textContent?.includes("size:A4"));
    expect(a4Styles()).toHaveLength(0);
    show(DIGITAL);
    expect(a4Styles()).toHaveLength(1);
    expect(readFileSync("app/globals.css", "utf8")).not.toMatch(/@page/);
  });

  it("invents no legal entity details", () => {
    show(DIGITAL);
    expect(document.querySelector(".invoice-paper")!.textContent).not.toMatch(/npwp|pt\.? |cv\.? |tanda tangan/i);
  });

  it("prints with window.print", async () => {
    show(DIGITAL);
    await userEvent.click(screen.getByRole("button", { name: "Cetak" }));
    expect(window.print).toHaveBeenCalledTimes(1);
  });

  it("downloads the paper node as Invoice-<last 8>.pdf, disabled while generating", async () => {
    let finish!: () => void;
    savePageAsPdf.mockReturnValue(new Promise<void>((r) => (finish = r)));
    show(DIGITAL);
    const button = screen.getByRole("button", { name: "Unduh PDF" });
    await userEvent.click(button);
    expect(savePageAsPdf).toHaveBeenCalledWith(document.querySelector(".invoice-paper"), `Invoice-${DIGITAL.id.slice(-8)}.pdf`, expect.any(String));
    expect(screen.getByRole("button", { name: /membuat pdf/i })).toBeDisabled();
    await act(async () => finish());
    await waitFor(() => expect(toastSuccess).toHaveBeenCalledWith("Invoice diunduh"));
    expect(screen.getByRole("button", { name: "Unduh PDF" })).toBeEnabled();
  });

  it("toasts a failure and re-enables the button", async () => {
    savePageAsPdf.mockRejectedValue(new Error("boom"));
    show(DIGITAL);
    await userEvent.click(screen.getByRole("button", { name: "Unduh PDF" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Gagal membuat PDF. Coba lagi."));
    expect(toastSuccess).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Unduh PDF" })).toBeEnabled();
  });
});
