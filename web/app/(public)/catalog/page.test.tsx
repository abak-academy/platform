import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CatalogPage from "./page";
import { useProducts } from "@/lib/hooks/products";
import { useAuthStore } from "@/stores/auth";
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

  it("wraps every product card for the tilt effect", () => {
    render(<CatalogPage />);
    expect(document.querySelectorAll("[data-reveal-grid] > [data-tilt]")).toHaveLength(2);
    expect(document.querySelectorAll("[data-tilt]")).toHaveLength(2);
    expect(document.querySelector("[data-tilt] [data-tilt]")).toBeNull();
  });

  it("reserves layout while loading: 8 cards with the same grid and 3:4 cover as the loaded grid", () => {
    vi.mocked(useProducts).mockReturnValue({ data: undefined, isLoading: true, isError: false } as never);
    render(<CatalogPage />);
    const sk = screen.getByTestId("catalog-skeleton");
    expect(sk.children).toHaveLength(8);
    expect(sk.className).toContain("grid-cols-2");
    expect(sk.className).toContain("lg:grid-cols-4");
    expect(sk.querySelector(".aspect-\\[3\\/4\\]")).not.toBeNull();
  });

  it("loads only the first row of cover images eagerly", () => {
    const many = Array.from({ length: 6 }, (_, i) => ({
      id: `p${i}`, type: "book", name: `Buku ${i}`, price: 1000, image_url: "https://cdn.example.com/a.png",
    }));
    vi.mocked(useProducts).mockReturnValue({ data: many, isLoading: false, isError: false } as never);
    render(<CatalogPage />);
    const imgs = screen.getAllByRole("img");
    expect(imgs.map((i) => i.getAttribute("loading"))).toEqual(["eager", "eager", "eager", "eager", "lazy", "lazy"]);
  });

  describe("empty store", () => {
    beforeEach(() => {
      vi.mocked(useProducts).mockReturnValue({ data: [], isLoading: false, isError: false } as never);
    });

    it("offers a guest no dashboard link and no inert notify button", () => {
      useAuthStore.setState({ token: null });
      render(<CatalogPage />);
      expect(screen.getByRole("heading", { name: /toko/i })).toBeInTheDocument();
      expect(screen.queryByRole("link", { name: /beranda/i })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /beritahu/i })).not.toBeInTheDocument();
    });

    it("links a signed-in student back to the dashboard", () => {
      useAuthStore.setState({ token: "tok" });
      render(<CatalogPage />);
      expect(screen.getByRole("link", { name: /beranda/i })).toHaveAttribute("href", "/");
      useAuthStore.setState({ token: null });
    });
  });

  it("has no event filter (events live on /event)", () => {
    render(<CatalogPage />);
    expect(screen.queryByRole("combobox")).not.toBeInTheDocument();
    expect(screen.queryByText("Semua event")).not.toBeInTheDocument();
  });

  it("preselects courses from ?type=course", () => {
    query = "type=course";
    render(<CatalogPage />);
    expect(screen.getByRole("tab", { name: /Kursus/ })).toHaveAttribute("aria-selected", "true");
  });
});

describe("CatalogPage suspense shell", () => {
  it("renders hero and skeleton grid while useSearchParams suspends", async () => {
    const nav = await import("next/navigation");
    const spy = vi.spyOn(nav, "useSearchParams").mockImplementation(() => {
      throw new Promise(() => {});
    });
    render(<CatalogPage />);
    expect(screen.getByRole("heading", { level: 1 })).toBeInTheDocument();
    expect(screen.getByTestId("catalog-skeleton")).toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/cari/i)).not.toBeInTheDocument();
    spy.mockRestore();
  });
});
