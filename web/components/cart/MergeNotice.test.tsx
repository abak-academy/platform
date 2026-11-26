import { beforeEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MergeNotice } from "./MergeNotice";
import { mergeGuestCart, useMergeNoticeStore, type MergeOutcome } from "@/lib/guest-cart-merge";
import { ApiError } from "@/lib/api";
import type { Order } from "@/lib/types";

function show(...outcomes: MergeOutcome[]) {
  act(() => useMergeNoticeStore.getState().show(outcomes));
}

beforeEach(() => act(() => useMergeNoticeStore.getState().dismiss()));

describe("MergeNotice", () => {
  it("renders nothing when there are no outcomes", () => {
    const { container } = render(<MergeNotice />);
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing when every outcome succeeded unchanged", () => {
    show({ product_id: "p1", name: "Buku A", code: "added" });
    const { container } = render(<MergeNotice />);
    expect(container).toBeEmptyDOMElement();
  });

  it("shows the old and new price for a price change", () => {
    show({ product_id: "p1", name: "Buku A", code: "added", price_changed: { from: 50000, to: 60000 } });
    render(<MergeNotice />);
    expect(screen.getByText("Buku A")).toBeInTheDocument();
    expect(screen.getByText(/Harga berubah: Rp50\.000 → Rp60\.000/)).toBeInTheDocument();
  });

  it("explains an item already in the cart", () => {
    show({ product_id: "p1", name: "Kursus B", code: "already_in_cart" });
    render(<MergeNotice />);
    expect(screen.getByText(/Sudah ada di keranjangmu/)).toBeInTheDocument();
  });

  it("explains an unavailable item", () => {
    show({ product_id: "p1", name: "Kursus B", code: "unavailable" });
    render(<MergeNotice />);
    expect(screen.getByText(/Tidak tersedia lagi/)).toBeInTheDocument();
  });

  it("explains an out-of-stock item", () => {
    show({ product_id: "p1", name: "Buku A", code: "out_of_stock" });
    render(<MergeNotice />);
    expect(screen.getByText(/Stok habis/)).toBeInTheDocument();
  });

  it("shows the specific digital conflict text produced by the merge", async () => {
    const conflict = new ApiError("digital_target_conflict", "x", 409, {
      code: "digital_target_conflict",
      message: "x",
      details: { conflicts: [{ kind: "course", reason: "owned" }] },
    });
    const { outcomes } = await mergeGuestCart(
      [{ product_id: "p1", qty: 1, name: "Kursus B", type: "course", unit_price: 1000, image_url: "" }],
      {
        getCart: async () => ({ id: "o1", status: "cart", items: [] }) as unknown as Order,
        addItem: async () => {
          throw conflict;
        },
      },
    );
    show(...outcomes);
    render(<MergeNotice />);
    expect(screen.getByText(/Kamu sudah memiliki kursus ini\./)).toBeInTheDocument();
  });

  it("clears the store when dismissed", async () => {
    show({ product_id: "p1", name: "Buku A", code: "out_of_stock" });
    render(<MergeNotice />);
    await userEvent.click(screen.getByRole("button", { name: "Tutup" }));
    expect(useMergeNoticeStore.getState().outcomes).toEqual([]);
    expect(screen.queryByText("Buku A")).toBeNull();
  });
});
