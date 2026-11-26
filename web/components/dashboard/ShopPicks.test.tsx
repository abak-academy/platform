import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ShopPicks } from "./ShopPicks";
import type { Product } from "@/lib/types";

const product = (over: Partial<Product>): Product => ({ id: "p", type: "book", name: "Buku", price: 50000, stock: 5, ...over });

describe("ShopPicks", () => {
  it("renders nothing when no product qualifies", () => {
    const { container } = render(<ShopPicks products={[product({ type: "exam", name: "Ujian A" })]} enrolledCourseIds={[]} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders heading, shop link and product cards linking to the catalog", () => {
    render(<ShopPicks products={[product({ id: "b1", name: "Buku Satu" })]} enrolledCourseIds={[]} />);
    expect(screen.getByText("Dari Toko")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Ke toko/ })).toHaveAttribute("href", "/catalog");
    expect(screen.getByRole("link", { name: /Buku Satu/ })).toHaveAttribute("href", "/catalog/b1");
  });

  it("shows at most four cards and never an exam product", () => {
    const products = [
      product({ id: "e", type: "exam", name: "Ujian X" }),
      ...[1, 2, 3, 4, 5].map((n) => product({ id: `b${n}`, name: `Buku ${n}` })),
    ];
    render(<ShopPicks products={products} enrolledCourseIds={[]} />);
    expect(screen.queryByText("Ujian X")).not.toBeInTheDocument();
    expect(screen.getAllByTestId("product-cover")).toHaveLength(4);
    expect(screen.queryByText("Buku 5")).not.toBeInTheDocument();
  });

  it("skips owned courses and out-of-stock physical products", () => {
    render(
      <ShopPicks
        products={[
          product({ id: "c1", type: "course", name: "Kursus Dimiliki", course_ids: ["k1"] }),
          product({ id: "b0", name: "Buku Habis", stock: 0 }),
          product({ id: "b1", name: "Buku Ada" }),
        ]}
        enrolledCourseIds={["k1"]}
      />,
    );
    expect(screen.queryByText("Kursus Dimiliki")).not.toBeInTheDocument();
    expect(screen.queryByText("Buku Habis")).not.toBeInTheDocument();
    expect(screen.getByText("Buku Ada")).toBeInTheDocument();
  });
});
