import { create } from "zustand";
import { persist, type PersistStorage } from "zustand/middleware";
import type { Product, ProductType } from "@/lib/types";

export const GUEST_CART_MAX_LINES = 20;
const DIGITAL_TYPES: ProductType[] = ["course", "exam"];

export interface GuestLine {
  product_id: string;
  qty: number;
  name: string;
  type: ProductType;
  unit_price: number;
  image_url: string;
}

export type GuestAddResult = "added" | "duplicate" | "capped" | "full";

interface GuestCartState {
  lines: GuestLine[];
  add: (product: Product, qty?: number) => GuestAddResult;
  setQty: (productId: string, qty: number, stock?: number) => void;
  remove: (productId: string) => void;
  removeMany: (productIds: string[]) => void;
  clear: () => void;
}

export const isDigitalType = (type: ProductType) => DIGITAL_TYPES.includes(type);

function stockCap(product: Pick<Product, "type" | "stock" | "stock_limited">): number {
  const limited = !isDigitalType(product.type) || product.stock_limited;
  return limited && typeof product.stock === "number" ? product.stock : Infinity;
}

function isLine(l: unknown): l is GuestLine {
  const x = l as GuestLine;
  return (
    !!x &&
    typeof x.product_id === "string" &&
    Number.isInteger(x.qty) &&
    x.qty >= 1 &&
    typeof x.name === "string" &&
    typeof x.type === "string" &&
    typeof x.unit_price === "number" &&
    typeof x.image_url === "string"
  );
}

// Persisted shape is the spec's flat { v: 1, lines }, not zustand's { state, version } envelope.
const storage: PersistStorage<{ lines: GuestLine[] }> = {
  getItem: (name) => {
    try {
      const raw = JSON.parse(localStorage.getItem(name) ?? "null");
      if (!raw || raw.v !== 1 || !Array.isArray(raw.lines)) return null;
      const lines: GuestLine[] = raw.lines.filter(isLine).slice(0, GUEST_CART_MAX_LINES);
      return { state: { lines }, version: 1 };
    } catch {
      return null;
    }
  },
  setItem: (name, value) => {
    try {
      localStorage.setItem(name, JSON.stringify({ v: 1, lines: value.state.lines }));
    } catch {}
  },
  removeItem: (name) => {
    try {
      localStorage.removeItem(name);
    } catch {}
  },
};

export const useGuestCart = create<GuestCartState>()(
  persist(
    (set, get) => ({
      lines: [],
      add: (product, qty = 1) => {
        const { lines } = get();
        const existing = lines.find((l) => l.product_id === product.id);
        if (isDigitalType(product.type)) {
          if (existing) return "duplicate";
          if (lines.length >= GUEST_CART_MAX_LINES) return "full";
          set({ lines: [...lines, toLine(product, 1)] });
          return "added";
        }
        const cap = stockCap(product);
        const want = Math.max(1, qty) + (existing?.qty ?? 0);
        const next = Math.min(want, cap);
        if (next < 1 || (existing && next === existing.qty)) return "capped";
        if (!existing && lines.length >= GUEST_CART_MAX_LINES) return "full";
        set({
          lines: existing
            ? lines.map((l) => (l === existing ? { ...l, qty: next } : l))
            : [...lines, toLine(product, next)],
        });
        return next < want ? "capped" : "added";
      },
      setQty: (productId, qty, stock) =>
        set({
          lines: get().lines.map((l) => {
            if (l.product_id !== productId || isDigitalType(l.type)) return l;
            const cap = typeof stock === "number" ? stock : Infinity;
            return { ...l, qty: Math.max(1, Math.min(Math.floor(qty), cap)) };
          }),
        }),
      remove: (productId) => get().removeMany([productId]),
      removeMany: (ids) => set({ lines: get().lines.filter((l) => !ids.includes(l.product_id)) }),
      clear: () => set({ lines: [] }),
    }),
    { name: "abak-guest-cart", version: 1, storage, partialize: (s) => ({ lines: s.lines }) }
  )
);

function toLine(p: Product, qty: number): GuestLine {
  return {
    product_id: p.id,
    qty,
    name: p.name,
    type: p.type,
    unit_price: p.price,
    image_url: p.image_url ?? "",
  };
}
