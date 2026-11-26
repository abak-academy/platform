import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CatalogPage from "./page";
import { useProducts } from "@/lib/hooks/products";
import type { Product } from "@/lib/types";

let query = "";
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => "/catalog",
  useSearchParams: () => new URLSearchParams(query),
}));
vi.mock("@/lib/hooks/products", () => ({ useProducts: vi.fn() }));
vi.mock("@/lib/hooks/product-groupings", () => ({
  useProductGroupings: () => ({ data: [{ id: "g1", name: "ESOLIO", status: "published", display_order: 1 }] }),
}));
const promoHook = vi.hoisted(() => vi.fn());
vi.mock("@/lib/hooks/orders", () => ({ useActivePromoCodes: promoHook }));

const product = (id: string, type: Product["type"], name: string): Product => ({
  id,
  type,
  name,
  price: 50000,
  stock_limited: true,
});
const fixture = [
  product("b1", "book", "Buku Latihan"),
  product("c1", "course", "Kursus Aljabar"),
  product("e1", "exam", "Tryout Nasional"),
];

beforeEach(() => {
  query = "";
  vi.mocked(useProducts).mockReset();
  vi.mocked(useProducts).mockReturnValue({ data: fixture, isLoading: false, isError: false } as never);
});

describe("CatalogPage", () => {
  it("shows the shop hero copy", () => {
    render(<CatalogPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Buku, kursus & perlengkapan belajar" })).toBeInTheDocument();
    expect(screen.getByText("TOKO ABAK", { exact: false })).toBeInTheDocument();
    expect(
      screen.getByText("Semua yang kamu butuhkan buat latihan, belajar, dan tampil di hari lomba."),
    ).toBeInTheDocument();
  });

  it("renders no promo line and never calls a promo hook", () => {
    render(<CatalogPage />);
    expect(screen.queryByText(/promo/i)).not.toBeInTheDocument();
    expect(promoHook).not.toHaveBeenCalled();
  });

  it("keeps exam products out of the grid", () => {
    render(<CatalogPage />);
    expect(screen.getByText("Buku Latihan")).toBeInTheDocument();
    expect(screen.getByText("Kursus Aljabar")).toBeInTheDocument();
    expect(screen.queryByText("Tryout Nasional")).not.toBeInTheDocument();
    expect(document.querySelector("[data-reveal-grid]")).not.toBeNull();
  });

  it("labels the grouping filter Event", () => {
    render(<CatalogPage />);
    expect(screen.getByRole("combobox", { name: "Event" })).toBeInTheDocument();
  });

  it("preselects courses from ?type=course", () => {
    query = "type=course";
    render(<CatalogPage />);
    expect(useProducts).toHaveBeenCalledWith("course", undefined);
    expect(screen.getByRole("button", { name: "Kursus" })).toHaveAttribute("aria-current", "true");
  });
});
