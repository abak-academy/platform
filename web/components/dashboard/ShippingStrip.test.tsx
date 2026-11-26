import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ShippingStrip } from "./ShippingStrip";
import type { Order } from "@/lib/types";

const toast = vi.hoisted(() => ({ success: vi.fn() }));
vi.mock("sonner", () => ({ toast }));

const order = (over: Partial<Order> = {}): Order => ({
  id: "ord-1",
  student_id: "stu-1",
  status: "shipped",
  subtotal: 100000,
  discount: 0,
  shipping_cost: 10000,
  total: 110000,
  selected_courier: "JNE",
  selected_service: "REG",
  tracking_number: "RESI123",
  paid_at: "2026-11-20T12:00:00Z",
  shipped_at: "2026-11-23T12:00:00Z",
  items: [
    { id: "i1", order_id: "ord-1", product_id: "p1", product_type: "book", name: "Buku Latihan", unit_price: 1, qty: 1, jumlah: 1 },
    { id: "i2", order_id: "ord-1", product_id: "p2", product_type: "course", name: "Kursus Matematika", unit_price: 1, qty: 1, jumlah: 1 },
  ],
  ...over,
});

describe("ShippingStrip", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows the shipped title, physical item names only, and a detail link", () => {
    render(<ShippingStrip order={order()} />);
    expect(screen.getByText("Paketmu sedang dikirim")).toBeInTheDocument();
    expect(screen.getByText("Buku Latihan")).toBeInTheDocument();
    expect(screen.queryByText(/Kursus Matematika/)).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Detail" })).toHaveAttribute("href", "/orders/ord-1");
  });

  it("shows the processing title and Dikemas reached without a date", () => {
    render(<ShippingStrip order={order({ status: "processing", shipped_at: undefined, tracking_number: undefined })} />);
    expect(screen.getByText("Pesananmu sedang dikemas")).toBeInTheDocument();
    const packed = screen.getByText("Dikemas").closest("li")!;
    expect(packed).toHaveAttribute("data-reached", "true");
    expect(packed).toHaveTextContent("—");
    expect(screen.getByText("Dikirim").closest("li")).toHaveAttribute("data-reached", "false");
    expect(screen.getByText("Diterima").closest("li")).toHaveAttribute("data-reached", "false");
  });

  it("renders dates for the paid and shipped steps", () => {
    render(<ShippingStrip order={order()} />);
    expect(screen.getByText("Dibayar").closest("li")).toHaveTextContent("20 Nov");
    expect(screen.getByText("Dikirim").closest("li")).toHaveTextContent("23 Nov");
  });

  it("shows courier and copies the resi with a toast", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(<ShippingStrip order={order()} />);
    expect(screen.getByText(/JNE REG/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Salin resi" }));
    expect(writeText).toHaveBeenCalledWith("RESI123");
    expect(toast.success).toHaveBeenCalledWith("Resi disalin");
  });

  it("without a resi shows the pending copy and no copy button", () => {
    render(<ShippingStrip order={order({ tracking_number: undefined })} />);
    expect(screen.getByText("Resi muncul setelah paket diserahkan ke kurir")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Salin resi" })).not.toBeInTheDocument();
  });
});
