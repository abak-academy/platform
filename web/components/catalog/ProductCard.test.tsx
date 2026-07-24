import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProductCard } from "./ProductCard";

describe("ProductCard", () => {
  it("renders the cover as a contain-fitted image, resolving object keys and preserving absolute URLs", () => {
    const { rerender } = render(
      <ProductCard
        product={{
          id: "merch-key",
          type: "merchandise",
          name: "Kaos Akademi",
          price: 75000,
          image_url: "avatars/store/tee.png",
        }}
      />,
    );

    let img = screen.getByAltText("Kaos Akademi") as HTMLImageElement;
    expect(img.src).toContain("http://localhost:8080/api/v1/files/avatars/store/tee.png");
    expect(img.className).toContain("object-contain");

    rerender(
      <ProductCard
        product={{
          id: "merch-legacy",
          type: "merchandise",
          name: "Tote Akademi",
          price: 50000,
          image_url: "https://cdn.example.com/tote.png",
        }}
      />,
    );

    img = screen.getByAltText("Tote Akademi") as HTMLImageElement;
    expect(img.src).toContain("https://cdn.example.com/tote.png");
  });

  it("falls back to a gradient placeholder that fills the same 3:4 box when there is no image", () => {
    render(
      <ProductCard product={{ id: "medal-fallback", type: "medal", name: "Medali", price: 10000 }} />,
    );

    expect(screen.queryByAltText("Medali")).toBeNull();
    const box = screen.getByTestId("product-cover");
    expect(box.style.background).toContain("linear-gradient");
    expect(box.className).toContain("aspect-[3/4]");
  });

  it("shows only the type badge, name and price — no description", () => {
    render(
      <ProductCard
        product={{
          id: "book-1",
          type: "book",
          name: "Kumpulan Soal KoSSMI Fisika",
          description: "Deskripsi panjang yang tidak boleh muncul di kartu.",
          price: 20000,
        }}
      />,
    );

    expect(screen.getByText("Buku")).toBeTruthy();
    expect(screen.getByText("Kumpulan Soal KoSSMI Fisika")).toBeTruthy();
    expect(screen.getByText("Rp20.000")).toBeTruthy();
    expect(screen.queryByText(/Deskripsi panjang/)).toBeNull();
  });
});
