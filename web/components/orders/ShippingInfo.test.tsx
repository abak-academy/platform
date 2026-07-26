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

  it("flags a flat-rate estimate as not being a carrier quote", () => {
    const flat = { ...physicalOrder, selected_courier: "Ongkir Flat", selected_service: "Standar" };
    render(<ShippingInfo order={flat} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  // Orders placed before the fallback was renamed still say "Flat". They must
  // keep the badge, or history silently reads as though those were real quotes.
  it("still flags orders stored under the pre-rename fallback label", () => {
    const legacy = { ...physicalOrder, selected_courier: "Flat", selected_service: "Standard" };
    render(<ShippingInfo order={legacy} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not flag a real carrier quote as an estimate", () => {
    const real = { ...physicalOrder, selected_courier: "JNE", selected_service: "REG" };
    render(<ShippingInfo order={real} />);
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });
});
