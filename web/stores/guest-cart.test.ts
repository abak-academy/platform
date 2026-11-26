import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Product } from "@/lib/types";
import { useGuestCart } from "./guest-cart";

const KEY = "abak-guest-cart";
const course: Product = { id: "c1", type: "course", name: "Kursus", price: 100000, image_url: "k/c.png" };
const exam: Product = { id: "e1", type: "exam", name: "Tryout", price: 50000 };
const book = (id: string, stock?: number): Product => ({ id, type: "book", name: `Buku ${id}`, price: 75000, stock });

const st = () => useGuestCart.getState();

beforeEach(() => {
  localStorage.clear();
  useGuestCart.setState({ lines: [] });
});

describe("guest cart rules", () => {
  it("keeps one line when a course is added twice", () => {
    expect(st().add(course)).toBe("added");
    expect(st().add(course)).toBe("duplicate");
    expect(st().lines).toHaveLength(1);
    expect(st().lines[0].qty).toBe(1);
  });

  it("forces digital qty to 1 and ignores setQty", () => {
    st().add(exam, 5);
    st().setQty("e1", 4);
    expect(st().lines[0].qty).toBe(1);
  });

  it("increases a physical line and caps at stock", () => {
    expect(st().add(book("b1", 3), 2)).toBe("added");
    expect(st().add(book("b1", 3), 1)).toBe("added");
    expect(st().lines[0].qty).toBe(3);
    expect(st().add(book("b1", 3), 1)).toBe("capped");
    expect(st().lines[0].qty).toBe(3);
  });

  it("clips a first add above stock and refuses stock 0", () => {
    expect(st().add(book("b1", 2), 5)).toBe("capped");
    expect(st().lines[0].qty).toBe(2);
    expect(st().add(book("b2", 0))).toBe("capped");
    expect(st().lines).toHaveLength(1);
  });

  it("setQty clamps to 1..stock", () => {
    st().add(book("b1", 5));
    st().setQty("b1", 9, 5);
    expect(st().lines[0].qty).toBe(5);
    st().setQty("b1", 0, 5);
    expect(st().lines[0].qty).toBe(1);
  });

  it("refuses a 21st line", () => {
    for (let i = 0; i < 20; i++) expect(st().add(book(`b${i}`, 9))).toBe("added");
    expect(st().add(book("extra", 9))).toBe("full");
    expect(st().add(book("b0", 9))).toBe("added");
    expect(st().lines).toHaveLength(20);
  });

  it("remove, removeMany and clear", () => {
    st().add(course);
    st().add(exam);
    st().add(book("b1", 3));
    st().remove("c1");
    expect(st().lines.map((l) => l.product_id)).toEqual(["e1", "b1"]);
    st().removeMany(["e1"]);
    expect(st().lines.map((l) => l.product_id)).toEqual(["b1"]);
    st().clear();
    expect(st().lines).toEqual([]);
  });
});

describe("guest cart persistence", () => {
  it("writes exactly { v: 1, lines } with the snapshot fields", () => {
    st().add(course);
    expect(JSON.parse(localStorage.getItem(KEY)!)).toEqual({
      v: 1,
      lines: [{ product_id: "c1", qty: 1, name: "Kursus", type: "course", unit_price: 100000, image_url: "k/c.png" }],
    });
  });

  it("rehydrates a valid v1 payload", async () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ v: 1, lines: [{ product_id: "c1", qty: 1, name: "K", type: "course", unit_price: 1, image_url: "" }] }),
    );
    await useGuestCart.persist.rehydrate();
    expect(st().lines.map((l) => l.product_id)).toEqual(["c1"]);
  });

  it.each([
    ["garbage", "{not json"],
    ["unknown version", JSON.stringify({ v: 2, lines: [{ product_id: "c1", qty: 1, name: "K", type: "course", unit_price: 1, image_url: "" }] })],
    ["wrong shape", JSON.stringify({ v: 1, lines: "x" })],
  ])("resets to empty on %s", async (_n, raw) => {
    localStorage.setItem(KEY, raw);
    vi.resetModules();
    const fresh = await import("./guest-cart");
    expect(fresh.useGuestCart.getState().lines).toEqual([]);
  });

  it("drops malformed lines but keeps valid ones", async () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({
        v: 1,
        lines: [{ product_id: "c1", qty: 0, name: "K", type: "course", unit_price: 1, image_url: "" }, { product_id: "e1", qty: 1, name: "T", type: "exam", unit_price: 1, image_url: "" }],
      }),
    );
    await useGuestCart.persist.rehydrate();
    expect(st().lines.map((l) => l.product_id)).toEqual(["e1"]);
  });
});

describe("guest cart syncLine", () => {
  const book = { id: "b1", type: "book", name: "Buku", price: 1000, stock: 5 } as Product;

  it("updates the price snapshot and flags/unflags unavailable, persisted", () => {
    useGuestCart.getState().add(book);
    useGuestCart.getState().syncLine("b1", { unit_price: 1500, unavailable: true });
    expect(useGuestCart.getState().lines[0]).toMatchObject({ unit_price: 1500, unavailable: true });
    expect(JSON.parse(localStorage.getItem(KEY)!).lines[0]).toMatchObject({ unit_price: 1500, unavailable: true });
    useGuestCart.getState().syncLine("b1", { unavailable: false });
    expect(useGuestCart.getState().lines[0]).not.toHaveProperty("unavailable");
    expect(useGuestCart.getState().lines[0].unit_price).toBe(1500);
  });

  it("is a no-op when nothing changed", () => {
    useGuestCart.getState().add(book);
    const before = useGuestCart.getState().lines;
    useGuestCart.getState().syncLine("b1", { unit_price: 1000, unavailable: false });
    expect(useGuestCart.getState().lines).toBe(before);
  });

  it("loads an old payload without the flag and one with it", async () => {
    const l = { product_id: "x", qty: 1, name: "X", type: "book", unit_price: 1, image_url: "" };
    localStorage.setItem(KEY, JSON.stringify({ v: 1, lines: [l, { ...l, product_id: "y", unavailable: true }] }));
    await useGuestCart.persist.rehydrate();
    expect(useGuestCart.getState().lines.map((x) => x.unavailable)).toEqual([undefined, true]);
  });
});
