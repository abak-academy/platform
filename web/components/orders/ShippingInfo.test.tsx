import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ShippingInfo } from "./ShippingInfo";

const physicalOrder = {
  id: "o1",
  status: "paid",
  shipping_cost: 15000,
  selected_courier: "JNE",
  selected_service: "REG",
  tracking_number: "JP1234567",
  shipping_address: {
    penerima: "Saifullah Panca",
    telepon: "08123456789",
    alamat: "Jl. Merdeka No. 1",
    kode_pos: "40123",
  },
  items: [{ id: "i1", product_id: "p1", product_type: "book", name: "Buku", unit_price: 1, qty: 1 }],
} as any;

describe("ShippingInfo", () => {
  it("shows courier, service, address and tracking for a physical order", () => {
    render(<ShippingInfo order={physicalOrder} />);
    expect(screen.getByText("JNE — REG")).toBeTruthy();
    expect(screen.getByText(/Jl. Merdeka No. 1/)).toBeTruthy();
    expect(screen.getByText("JP1234567")).toBeTruthy();
  });

  it("renders nothing for a digital-only order", () => {
    const digital = {
      ...physicalOrder,
      items: [{ id: "i1", product_id: "p1", product_type: "exam", name: "Ujian", unit_price: 1, qty: 1 }],
    };
    const { container } = render(<ShippingInfo order={digital} />);
    expect(container.firstChild).toBeNull();
  });

  // The old check inferred the badge from selected_courier; both cases below
  // are where that inference gets it backwards.
  it("flags an estimate even when the courier name looks like a real carrier", () => {
    const estimate = { ...physicalOrder, is_estimate: true, selected_courier: "JNE", selected_service: "REG" };
    render(<ShippingInfo order={estimate} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not flag a real quote even when the courier name is the fallback label", () => {
    const real = { ...physicalOrder, is_estimate: false, selected_courier: "Ongkir Flat", selected_service: "Standar" };
    render(<ShippingInfo order={real} />);
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });

  it("shows shipment_status alongside the resi", () => {
    const withStatus = { ...physicalOrder, shipment_status: "delivered" };
    render(<ShippingInfo order={withStatus} />);
    expect(screen.getByText("delivered")).toBeTruthy();
  });

  // A buyer has no use for "we typed this in by hand"; only an admin does.
  it("never shows waybill_source — that is admin-only, in OrderDetailModal", () => {
    const withWaybillSource = { ...physicalOrder, waybill_source: "manual" };
    render(<ShippingInfo order={withWaybillSource} />);
    expect(screen.queryByText(/Input manual/)).toBeNull();
    expect(screen.queryByText(/Booking otomatis/)).toBeNull();
  });

  it("renders the shipment timeline when events are present", () => {
    const withEvents = {
      ...physicalOrder,
      shipment_events: [
        {
          id: "e1",
          order_id: "o1",
          status: "Pesanan dikonfirmasi",
          occurred_at: "2026-07-20T01:00:00Z",
          created_at: "2026-07-20T01:00:05Z",
        },
      ],
    };
    render(<ShippingInfo order={withEvents} />);
    expect(screen.getByText("Riwayat Pengiriman")).toBeTruthy();
    expect(screen.getByTestId("shipment-event-status").textContent).toBe("Pesanan dikonfirmasi");
  });

  it("renders no timeline heading when there are no shipment events", () => {
    render(<ShippingInfo order={physicalOrder} />);
    expect(screen.queryByText("Riwayat Pengiriman")).toBeNull();
  });
});
