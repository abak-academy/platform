import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CartLineItem } from "./CartLineItem";

const base = {
  id: "item-1",
  product_id: "p1",
  name: "Item",
  unit_price: 10000,
  qty: 1,
  jumlah: 10000,
};

describe("CartLineItem", () => {
  it("hides the quantity stepper for digital products", () => {
    render(
      <CartLineItem
        item={{ ...base, product_type: "exam" } as any}
        onRemove={() => {}}
        onQtyChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText("Tambah jumlah")).toBeNull();
    expect(screen.getByText("Produk digital dibeli 1× per akun.")).toBeTruthy();
  });

  it("keeps the stepper for physical products", () => {
    render(
      <CartLineItem
        item={{ ...base, product_type: "book" } as any}
        onRemove={() => {}}
        onQtyChange={() => {}}
      />,
    );
    expect(screen.getByLabelText("Tambah jumlah")).toBeTruthy();
  });

  // An exam in the cart was labelled "Buku": the badge map covered three of the
  // five product types and a `?? book` fallback turned every miss into a
  // confident wrong answer. Every type is asserted here so the next one added
  // cannot slip through the same gap.
  it.each([
    ["book", "Buku"],
    ["course", "Kursus"],
    ["exam", "Ujian"],
    ["merchandise", "Merchandise"],
    ["medal", "Medali"],
  ])("labels a %s line item as %s", (productType, label) => {
    render(
      <CartLineItem
        item={{ ...base, product_type: productType } as any}
        onRemove={() => {}}
        onQtyChange={() => {}}
      />,
    );
    expect(screen.getByText(label)).toBeTruthy();
  });
});
