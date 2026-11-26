import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { GuestCart } from "./GuestCart";
import { ApiError } from "@/lib/api";
import { useGuestCart } from "@/stores/guest-cart";
import type { Product } from "@/lib/types";

const { apiFetch } = vi.hoisted(() => ({ apiFetch: vi.fn() }));
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), apiFetch }));

const course: Product = { id: "c1", type: "course", name: "Kursus Fisika", price: 90000 };
const book: Product = { id: "b1", type: "book", name: "Buku Olimpiade", price: 50000, stock: 3 };

const catalog: Record<string, Product> = {};

function renderCart() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <GuestCart />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  useGuestCart.setState({ lines: [] });
  for (const k of Object.keys(catalog)) delete catalog[k];
  Object.assign(catalog, { c1: course, b1: book });
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string) => {
    const p = catalog[decodeURIComponent(path.split("/").pop()!)];
    if (!p) throw new ApiError("product_not_found", "not found", 404);
    return p;
  });
});

describe("GuestCart", () => {
  it("shows an empty state linking to the catalog", () => {
    renderCart();
    expect(screen.getByText("Keranjangmu masih kosong")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /toko/i })).toHaveAttribute("href", "/catalog");
  });

  it("keeps one line when a course is added twice", async () => {
    expect(useGuestCart.getState().add(course)).toBe("added");
    expect(useGuestCart.getState().add(course)).toBe("duplicate");
    renderCart();
    expect(await screen.findAllByText("Kursus Fisika")).toHaveLength(1);
    expect(screen.getByTestId("guest-subtotal")).toHaveTextContent("Rp90.000");
  });

  it("disables + at the stock cap and raises the quantity below it", async () => {
    useGuestCart.getState().add(book, 2);
    renderCart();
    const plus = await screen.findByRole("button", { name: /\+$/ });
    await waitFor(() => expect(plus).toBeEnabled());
    await userEvent.click(plus);
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(plus).toBeDisabled();
    expect(screen.getByTestId("guest-subtotal")).toHaveTextContent("Rp150.000");
  });

  it("keeps the cart across a reload from persisted storage", async () => {
    useGuestCart.getState().add(course);
    const raw = localStorage.getItem("abak-guest-cart")!;
    useGuestCart.setState({ lines: [] });
    localStorage.setItem("abak-guest-cart", raw);
    await useGuestCart.persist.rehydrate();
    renderCart();
    expect(await screen.findByText("Kursus Fisika")).toBeInTheDocument();
  });

  it("offers only login to pay, with none of the signed-in checkout controls", async () => {
    useGuestCart.getState().add(course);
    renderCart();
    await screen.findByText("Kursus Fisika");
    expect(screen.getByRole("link", { name: "Masuk untuk bayar" })).toHaveAttribute("href", "/login?next=/cart");
    expect(screen.getByText("Keranjangmu tetap tersimpan setelah masuk.")).toBeInTheDocument();
    expect(screen.queryByRole("textbox")).toBeNull();
    expect(screen.queryByRole("combobox")).toBeNull();
    expect(screen.queryByText(/kurir|ongkir/i)).toBeNull();
    expect(screen.queryByRole("button", { name: /bayar/i })).toBeNull();
  });

  it("shows the address note only when a physical line is present", async () => {
    useGuestCart.getState().add(course);
    const { unmount } = renderCart();
    await screen.findByText("Kursus Fisika");
    expect(screen.queryByText(/Alamat dan ongkos kirim/)).toBeNull();
    expect(screen.getByText(/Punya kode promo/)).toBeInTheDocument();
    unmount();

    useGuestCart.getState().add(book);
    renderCart();
    expect(await screen.findByText(/Alamat dan ongkos kirim/)).toBeInTheDocument();
  });

  it("marks a 404 line unavailable and leaves it out of the subtotal", async () => {
    useGuestCart.getState().add(course);
    useGuestCart.getState().add(book, 1);
    delete catalog.c1;
    renderCart();
    expect(await screen.findByText("Tidak tersedia lagi")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId("guest-subtotal")).toHaveTextContent("Rp50.000"));
  });

  it("treats a product outside its availability window as unavailable", async () => {
    useGuestCart.getState().add(course);
    catalog.c1 = { ...course, available_until: "2020-01-01T00:00:00Z" };
    renderCart();
    expect(await screen.findByText("Tidak tersedia lagi")).toBeInTheDocument();
    expect(screen.getByTestId("guest-subtotal")).toHaveTextContent("Rp0");
  });

  it("shows the changed price and the new subtotal", async () => {
    useGuestCart.getState().add(course);
    catalog.c1 = { ...course, price: 120000 };
    renderCart();
    expect(await screen.findByText("Harga berubah")).toBeInTheDocument();
    const subtotal = screen.getByTestId("guest-subtotal");
    await waitFor(() => expect(subtotal).toHaveTextContent("Rp120.000"));
  });

  it("removes a line", async () => {
    useGuestCart.getState().add(course);
    renderCart();
    const row = (await screen.findByText("Kursus Fisika")).closest("li")!;
    await userEvent.click(within(row).getByRole("button", { name: /hapus/i }));
    expect(useGuestCart.getState().lines).toEqual([]);
    expect(screen.getByText("Keranjangmu masih kosong")).toBeInTheDocument();
  });

  it("persists the unavailable mark on the stored line", async () => {
    useGuestCart.getState().add(course);
    delete catalog.c1;
    renderCart();
    await screen.findByText("Tidak tersedia lagi");
    await waitFor(() => expect(useGuestCart.getState().lines[0].unavailable).toBe(true));
  });

  it("updates the stored price snapshot and still shows Harga berubah", async () => {
    useGuestCart.getState().add(course);
    catalog.c1 = { ...course, price: 120000 };
    renderCart();
    await waitFor(() => expect(useGuestCart.getState().lines[0].unit_price).toBe(120000));
    expect(screen.getByText("Harga berubah")).toBeInTheDocument();
  });
});
