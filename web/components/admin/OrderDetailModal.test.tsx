import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import userEvent from "@testing-library/user-event";
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

describe("OrderDetailModal actions", () => {
  it("renders no action footer when the caller allows neither action", () => {
    render(<OrderDetailModal order={physicalOrder} onOpenChange={vi.fn()} />);
    expect(screen.queryByRole("button", { name: "Kirim" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Refund" })).toBeNull();
  });

  it("renders only the action the caller allowed", () => {
    render(<OrderDetailModal order={physicalOrder} onOpenChange={vi.fn()} onRefund={vi.fn()} />);
    expect(screen.getByRole("button", { name: "Refund" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Kirim" })).toBeNull();
  });

  it("invokes the caller's handlers", async () => {
    const onShip = vi.fn();
    const onRefund = vi.fn();
    render(
      <OrderDetailModal
        order={physicalOrder}
        onOpenChange={vi.fn()}
        onShip={onShip}
        onRefund={onRefund}
      />,
    );
    await userEvent.click(screen.getByRole("button", { name: "Kirim" }));
    await userEvent.click(screen.getByRole("button", { name: "Refund" }));
    expect(onShip).toHaveBeenCalledOnce();
    expect(onRefund).toHaveBeenCalledOnce();
  });

  it("disables refund while a refund is in flight", () => {
    render(
      <OrderDetailModal
        order={physicalOrder}
        onOpenChange={vi.fn()}
        onRefund={vi.fn()}
        isRefunding
      />,
    );
    expect(screen.getByRole("button", { name: "Refund" })).toBeDisabled();
  });
});

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

  it("shows the Cetak Resi print action when tracking_number is present", () => {
    const shipped = { ...physicalOrder, tracking_number: "JP1234567" };
    render(<OrderDetailModal order={shipped} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Cetak Resi")).toBeTruthy();
  });

  it("hides the Cetak Resi print action when tracking_number is empty", () => {
    const unshipped = { ...physicalOrder, tracking_number: "" };
    render(<OrderDetailModal order={unshipped} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Cetak Resi")).toBeNull();
  });

  it("shows waybill_source next to the resi, admin-only", () => {
    const biteshipShipped = {
      ...physicalOrder,
      tracking_number: "JP1234567",
      waybill_source: "biteship",
    };
    render(<OrderDetailModal order={biteshipShipped} onOpenChange={vi.fn()} />);
    expect(screen.getByText(/Booking otomatis \(Biteship\)/)).toBeTruthy();
  });

  it("renders the shipment timeline with events ordered newest-first", () => {
    const withEvents = {
      ...physicalOrder,
      tracking_number: "JP1234567",
      shipment_events: [
        {
          id: "e1",
          order_id: "o1",
          status: "confirmed",
          occurred_at: "2026-07-20T01:00:00Z",
          created_at: "2026-07-20T01:00:05Z",
        },
        {
          id: "e2",
          order_id: "o1",
          status: "delivered",
          occurred_at: "2026-07-22T01:00:00Z",
          created_at: "2026-07-22T01:00:05Z",
        },
      ],
    };
    render(<OrderDetailModal order={withEvents} onOpenChange={vi.fn()} />);
    const statuses = screen.getAllByTestId("shipment-event-status").map((el) => el.textContent);
    expect(statuses).toEqual(["delivered", "confirmed"]);
  });

  it("renders no timeline heading for an order with no shipment events", () => {
    const noEvents = { ...physicalOrder, tracking_number: "JP1234567", shipment_events: [] };
    render(<OrderDetailModal order={noEvents} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Riwayat Pengiriman")).toBeNull();
  });
});
