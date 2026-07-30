import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { OrderDetailModal } from "./OrderDetailModal";

const physicalOrder = {
  id: "o1",
  status: "paid",
  subtotal: 10000,
  discount: 0,
  total: 10000,
  shipping_cost: 15000,
  selected_courier: "JNE",
  selected_service: "REG",
  student_id: "s1",
  items: [{ id: "i1", order_id: "o1", product_id: "p1", product_type: "book", name: "Buku", unit_price: 1, qty: 1, jumlah: 1 }],
} as any;

describe("OrderDetailModal", () => {
  // The old check inferred the badge from selected_courier; both cases below
  // are where that inference gets it backwards.
  it("flags an estimate even when the courier name looks like a real carrier", () => {
    const estimate = { ...physicalOrder, is_estimate: true, selected_courier: "JNE" };
    render(<OrderDetailModal order={estimate} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not flag a real quote even when the courier name is the fallback label", () => {
    const real = { ...physicalOrder, is_estimate: false, selected_courier: "Ongkir Flat" };
    render(<OrderDetailModal order={real} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });
});
