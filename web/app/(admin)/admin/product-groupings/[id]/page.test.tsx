import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import ProductGroupingDetailPage from "./page";

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "g1" }),
}));

const h = vi.hoisted(() => ({
  grouping: vi.fn(),
  setMembers: vi.fn(),
}));

vi.mock("@/lib/hooks/admin-product-groupings", () => ({
  useAdminProductGrouping: h.grouping,
  useSetGroupingProducts: () => ({ mutateAsync: h.setMembers, isPending: false }),
}));

vi.mock("@/lib/hooks/admin-products", () => ({
  useAdminProducts: () => ({ data: [] }),
}));

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const grouping = {
  id: "g1",
  name: "Paket Hemat",
  description: "",
  status: "published",
  display_order: 0,
  products: [
    { id: "p1", name: "Produk A", price: 10000 },
    { id: "p2", name: "Produk B", price: 20000 },
    { id: "p3", name: "Produk C", price: 30000 },
  ],
};

function itemNames() {
  return screen.getAllByRole("listitem").map((li) => li.textContent ?? "");
}

beforeEach(() => {
  h.grouping.mockReturnValue({ data: grouping, isLoading: false, isError: false, error: null });
  h.setMembers.mockResolvedValue(grouping);
});

describe("ProductGroupingDetailPage member order", () => {
  it("every member row offers a drag handle alongside the arrow buttons", () => {
    render(<ProductGroupingDetailPage />);

    expect(screen.getAllByRole("button", { name: "Seret untuk mengurutkan" })).toHaveLength(3);
    expect(screen.getAllByRole("button", { name: "Naikkan urutan" })).toHaveLength(3);
    expect(screen.getAllByRole("button", { name: "Turunkan urutan" })).toHaveLength(3);
  });

  it("moves a product up with the arrow button and saves the resulting order", async () => {
    render(<ProductGroupingDetailPage />);
    expect(itemNames()[0]).toContain("Produk A");

    // Move Produk B (index 1) up, ahead of Produk A.
    fireEvent.click(screen.getAllByRole("button", { name: "Naikkan urutan" })[1]);
    expect(itemNames()[0]).toContain("Produk B");
    expect(itemNames()[1]).toContain("Produk A");

    fireEvent.click(screen.getByRole("button", { name: "Simpan perubahan" }));

    await waitFor(() => expect(h.setMembers).toHaveBeenCalledWith({ id: "g1", productIds: ["p2", "p1", "p3"] }));
  });

  it("disables moving the first item up and the last item down", () => {
    render(<ProductGroupingDetailPage />);

    expect(screen.getAllByRole("button", { name: "Naikkan urutan" })[0]).toBeDisabled();
    expect(screen.getAllByRole("button", { name: "Turunkan urutan" }).at(-1)).toBeDisabled();
  });
});
