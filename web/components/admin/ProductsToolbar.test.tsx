import { describe, it, expect, vi, beforeAll, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, within, act, waitFor } from "@testing-library/react";
import { ProductsToolbar, DEFAULT_PRODUCTS_QUERY, applyProductsQuery, type ProductsQuery, type ProductsSort } from "./ProductsToolbar";
import { useUIStore } from "@/stores/ui";
import type { Product } from "@/lib/types";

beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});
beforeEach(() => useUIStore.setState({ lang: "en" }));
afterEach(() => act(() => useUIStore.setState({ lang: "id" })));

function renderToolbar(value: ProductsQuery = DEFAULT_PRODUCTS_QUERY) {
  const onChange = vi.fn();
  render(<ProductsToolbar value={value} onChange={onChange} types={["course", "exam"]} />);
  return onChange;
}
function openPanel() {
  fireEvent.click(screen.getByRole("button", { name: /^Filters/ }));
}
async function openSelect(name: string) {
  fireEvent.pointerDown(screen.getByRole("combobox", { name }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  return screen.findByRole("listbox");
}
const active: ProductsQuery = { q: "test", type: "course", status: "hidden", sort: "price_desc", limit: 50 };
const sorts = [
  ["default", "Default order"], ["name_asc", "Name A-Z"], ["name_desc", "Name Z-A"],
  ["price_asc", "Price low-high"], ["price_desc", "Price high-low"],
  ["stock_asc", "Stock low-high"], ["stock_desc", "Stock high-low"],
] as const;

describe("ProductsToolbar", () => {
  it.each(["id", "en"] as const)("translates the panel and sorts in %s", async (lang) => {
    useUIStore.setState({ lang });
    renderToolbar();
    const sortLabel = lang === "id" ? "Urutkan produk" : "Sort products";
    const listbox = await openSelect(sortLabel);
    expect(within(listbox).getAllByRole("option").map((o) => o.textContent)).toEqual(lang === "en" ? sorts.map((s) => s[1]) : ["Urutan bawaan", "Nama A-Z", "Nama Z-A", "Harga terendah-tertinggi", "Harga tertinggi-terendah", "Stok terendah-tertinggi", "Stok tertinggi-terendah"]);
    fireEvent.keyDown(listbox, { key: "Escape" });
    fireEvent.click(screen.getByRole("button", { name: lang === "id" ? "Filter" : "Filters" }));
    expect(screen.getByRole("dialog", { name: lang === "id" ? "Filter produk" : "Filter products" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: lang === "id" ? "Reset filter" : "Reset filters" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: lang === "id" ? "Tampilkan produk" : "Show products" })).toBeInTheDocument();
  });
  it("emits search immediately and clears only search", () => {
    const onChange = renderToolbar(active);
    fireEvent.change(screen.getByRole("searchbox", { name: "Search product name" }), { target: { value: "new" } });
    expect(onChange).toHaveBeenLastCalledWith({ ...active, q: "new" });
    fireEvent.click(screen.getByRole("button", { name: "Clear search" }));
    expect(onChange).toHaveBeenLastCalledWith({ ...active, q: undefined });
  });
  it("has no count, chips or clear button without active filters", () => {
    renderToolbar();
    expect(screen.getByRole("button", { name: "Filters" })).toHaveTextContent(/^Filters$/);
    expect(screen.queryByRole("group", { name: "Active filters" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Clear search" })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Rows per page" })).not.toBeInTheDocument();
  });
  it.each([ ["Product type", ["All types", "Course", "Exam"]], ["Status", ["All", "Draft", "Published", "Hidden", "Archived"]] ])("offers the permitted %s options and keeps the panel open after selection", async (name, labels) => {
    const onChange = renderToolbar();
    openPanel();
    const listbox = await openSelect(name as string);
    expect(within(listbox).getAllByRole("option").map((o) => o.textContent)).toEqual(labels);
    fireEvent.click(screen.getByRole("option", { name: (labels as string[])[1] }));
    expect(onChange).toHaveBeenCalledWith({ ...DEFAULT_PRODUCTS_QUERY, [name === "Status" ? "status" : "type"]: name === "Status" ? "draft" : "course" });
    expect(screen.getByRole("dialog", { name: "Filter products" })).toBeInTheDocument();
  });
  it.each([ ["Product type: Course", "type"], ["Status: Hidden", "status"] ] as const)("removes only the %s chip", (label, key) => {
    const onChange = renderToolbar(active);
    expect(screen.getByRole("button", { name: "Filters (2)" })).toHaveTextContent("2");
    expect(within(screen.getByRole("group", { name: "Active filters" })).getAllByRole("button")).toHaveLength(2);
    fireEvent.click(screen.getByRole("button", { name: `Remove filter: ${label}` }));
    expect(onChange).toHaveBeenCalledWith({ ...active, [key]: undefined });
  });
  it("resets just type and status", () => {
    const onChange = renderToolbar(active);
    openPanel();
    fireEvent.click(screen.getByRole("button", { name: "Reset filters" }));
    expect(onChange).toHaveBeenCalledWith({ ...active, type: undefined, status: undefined });
  });
  it.each(["show", "escape", "outside"])("closes the panel via %s", async (action) => {
    renderToolbar();
    openPanel();
    const panel = screen.getByRole("dialog", { name: "Filter products" });
    if (action === "show") fireEvent.click(screen.getByRole("button", { name: "Show products" }));
    if (action === "escape") fireEvent.keyDown(panel, { key: "Escape" });
    if (action === "outside") {
      await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
      fireEvent.pointerDown(document.body, { button: 0, pointerType: "mouse" });
      fireEvent.click(document.body);
    }
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Filter products" })).not.toBeInTheDocument());
  });
  it.each(sorts)("maps %s to its sort option", async (sort, label) => {
    const value = { ...DEFAULT_PRODUCTS_QUERY, sort: (sort === "default" ? "name_asc" : "default") as ProductsSort };
    const onChange = renderToolbar(value);
    const menu = await openSelect("Sort products");
    expect(within(menu).getAllByRole("option").map((o) => o.textContent)).toEqual(sorts.map((s) => s[1]));
    fireEvent.click(screen.getByRole("option", { name: label }));
    expect(onChange).toHaveBeenCalledWith({ ...value, sort });
  });
});

const products: Product[] = [
  { id: "a", name: "Bravo", type: "book", price: 30, stock: 2, status: "published" },
  { id: "b", name: "alpha", type: "medal", price: 10, stock: 9, status: "hidden" },
  { id: "c", name: "Charlie", type: "exam", price: 20, stock: 5, stock_limited: true },
  { id: "d", name: "Delta", type: "course", price: 40, stock: 100, status: "archived" },
  { id: "e", name: "Echo", type: "exam", price: 50, stock: 100, stock_limited: false },
  { id: "f", name: "Foxtrot", type: "merchandise", price: 60, stock: 1 },
];
describe("applyProductsQuery", () => {
  it.each([
    [{}, "abcdef"], [{ sort: "name_asc" }, "bacdef"], [{ sort: "name_desc" }, "fedcab"],
    [{ sort: "price_asc" }, "bcadef"], [{ sort: "price_desc" }, "fedacb"],
    [{ sort: "stock_asc" }, "facbde"], [{ sort: "stock_desc" }, "bcafde"],
    [{ q: "  BRa  " }, "a"], [{ status: "draft" }, "cef"], [{ type: "exam", status: "draft" }, "ce"],
  ] as [Partial<ProductsQuery>, string][])("filters and sorts %j", (query, ids) => {
    expect(applyProductsQuery(products, { ...DEFAULT_PRODUCTS_QUERY, ...query }).map((p) => p.id).join("")).toBe(ids);
    expect(products.map((p) => p.id).join("")).toBe("abcdef");
  });
  it.each(sorts)("preserves API order on %s ties", (sort) => {
    const ties = products.map((p) => ({ ...p, name: p.id === "a" ? "ALPHA" : "alpha", price: 10, type: "book" as const, stock: 5 }));
    expect(applyProductsQuery(ties, { ...DEFAULT_PRODUCTS_QUERY, sort }).map((p) => p.id)).toEqual(products.map((p) => p.id));
  });
});
