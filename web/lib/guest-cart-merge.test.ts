import { describe, expect, it, vi } from "vitest";
import { ApiError } from "@/lib/api";
import type { Order, OrderItem } from "@/lib/types";
import type { GuestLine } from "@/stores/guest-cart";
import { mergeGuestCart, type MergeDeps } from "./guest-cart-merge";

const line = (id: string, over: Partial<GuestLine> = {}): GuestLine => ({
  product_id: id,
  qty: 1,
  name: `P-${id}`,
  type: "book",
  unit_price: 1000,
  image_url: "",
  ...over,
});

const item = (id: string, price = 1000): OrderItem => ({
  id: `i-${id}`,
  order_id: "o1",
  product_id: id,
  product_type: "book",
  name: id,
  unit_price: price,
  qty: 1,
  jumlah: price,
});

const order = (items: OrderItem[] = []): Order => ({ id: "o1", status: "cart", items }) as Order;
const err = (status: number, code: string) => new ApiError(code, code, status, { code, message: code });

function deps(add: MergeDeps["addItem"], items: OrderItem[] = []): MergeDeps {
  return { getCart: async () => order(items), addItem: add };
}

describe("mergeGuestCart outcomes", () => {
  it("added: sends the line, removes it, no notice flag when price is unchanged", async () => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => order([item(b.product_id)]));
    const r = await mergeGuestCart([line("a", { qty: 2 })], deps(add));
    expect(add).toHaveBeenCalledWith("o1", { product_id: "a", qty: 2 });
    expect(r.outcomes).toEqual([{ product_id: "a", name: "P-a", code: "added" }]);
    expect(r.removeIds).toEqual(["a"]);
    expect(r.stopped).toBeNull();
    expect(r.cartCount).toBe(1);
  });

  it("added with price_changed when the server price differs", async () => {
    const r = await mergeGuestCart([line("a")], deps(async () => order([item("a", 1500)])));
    expect(r.outcomes[0]).toMatchObject({ code: "added", price_changed: { from: 1000, to: 1500 } });
  });

  it("already_in_cart by pre-check without calling the server", async () => {
    const add = vi.fn();
    const r = await mergeGuestCart([line("a")], deps(add, [item("a")]));
    expect(add).not.toHaveBeenCalled();
    expect(r.outcomes[0].code).toBe("already_in_cart");
    expect(r.removeIds).toEqual(["a"]);
  });

  it.each([
    ["already_in_cart", err(400, "invalid_qty")],
    ["unavailable", err(404, "product_not_found")],
    ["out_of_stock", err(409, "out_of_stock")],
    ["digital_conflict", err(409, "digital_target_conflict")],
  ])("%s removes the line and continues", async (code, e) => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => {
      if (b.product_id === "a") throw e;
      return order([item(b.product_id)]);
    });
    const r = await mergeGuestCart([line("a"), line("b")], deps(add));
    expect(r.outcomes.map((o) => o.code)).toEqual([code, "added"]);
    expect(r.removeIds).toEqual(["a", "b"]);
    expect(r.stopped).toBeNull();
  });

  it("locally unavailable lines are not sent", async () => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => order([item(b.product_id)]));
    const r = await mergeGuestCart([line("a"), line("b")], { ...deps(add), isUnavailable: async (id) => id === "a" });
    expect(add).toHaveBeenCalledTimes(1);
    expect(r.outcomes.map((o) => o.code)).toEqual(["unavailable", "added"]);
    expect(r.removeIds).toEqual(["a", "b"]);
  });

  it("a failing availability check stops and keeps this and later lines", async () => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => order([item(b.product_id)]));
    const isUnavailable = vi.fn(async (id: string) => {
      if (id === "b") throw err(503, "unavailable");
      return false;
    });
    const r = await mergeGuestCart([line("a"), line("b"), line("c")], { ...deps(add), isUnavailable });
    expect(r.stopped).toBe("failed");
    expect(r.removeIds).toEqual(["a"]);
    expect(add).toHaveBeenCalledTimes(1);
  });

  it("401 stops and keeps this and later lines", async () => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => {
      if (b.product_id === "b") throw err(401, "unauthorized");
      return order([item(b.product_id)]);
    });
    const r = await mergeGuestCart([line("a"), line("b"), line("c")], deps(add));
    expect(r.stopped).toBe("unauthorized");
    expect(r.removeIds).toEqual(["a"]);
    expect(add).toHaveBeenCalledTimes(2);
  });

  it.each([
    ["5xx", err(503, "unavailable")],
    ["network", new TypeError("fetch failed")],
    ["unknown 4xx", err(422, "weird")],
  ])("%s stops and keeps unsent lines", async (_n, e) => {
    const add = vi.fn(async (_o: string, b: { product_id: string }) => {
      if (b.product_id === "b") throw e;
      return order([item(b.product_id)]);
    });
    const r = await mergeGuestCart([line("a"), line("b"), line("c")], deps(add));
    expect(r.stopped).toBe("failed");
    expect(r.removeIds).toEqual(["a"]);
    expect(add).toHaveBeenCalledTimes(2);
  });

  it("cart mint failure keeps everything", async () => {
    const r = await mergeGuestCart([line("a")], { getCart: async () => { throw err(500, "x"); }, addItem: vi.fn() });
    expect(r).toMatchObject({ stopped: "failed", removeIds: [], outcomes: [] });
  });

  it("retry after a partial failure does not duplicate", async () => {
    const server: OrderItem[] = [];
    let failB = true;
    const d: MergeDeps = {
      getCart: async () => order([...server]),
      addItem: async (_o, b) => {
        if (b.product_id === "b" && failB) throw err(500, "boom");
        server.push(item(b.product_id));
        return order([...server]);
      },
    };
    const lines = [line("a"), line("b")];
    const first = await mergeGuestCart(lines, d);
    expect(first.removeIds).toEqual(["a"]);
    failB = false;
    const rest = lines.filter((l) => !first.removeIds.includes(l.product_id));
    const second = await mergeGuestCart(rest, d);
    expect(second.stopped).toBeNull();
    expect(server.map((i) => i.product_id)).toEqual(["a", "b"]);
    const again = await mergeGuestCart(lines, d);
    expect(again.outcomes.map((o) => o.code)).toEqual(["already_in_cart", "already_in_cart"]);
    expect(server).toHaveLength(2);
  });

  it("digital_conflict carries the server error with its conflicts", async () => {
    const e = new ApiError("digital_target_conflict", "x", 409, {
      code: "digital_target_conflict",
      message: "x",
      details: { conflicts: [{ kind: "course", reason: "owned" }] },
    });
    const r = await mergeGuestCart([line("a")], deps(async () => { throw e; }));
    expect(r.outcomes[0].error).toBe(e);
  });
});
