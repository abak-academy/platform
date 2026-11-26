"use client";

import { useEffect, useRef } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { create } from "zustand";
import { apiFetch, ApiError, authFetch } from "@/lib/api";
import { ordersKeys } from "@/lib/hooks/orders";
import { useTranslation } from "@/lib/i18n";
import type { Order, Product } from "@/lib/types";
import { useCartStore } from "@/stores/cart";
import { useGuestCart, type GuestLine } from "@/stores/guest-cart";

export type MergeCode =
  | "added"
  | "already_in_cart"
  | "unavailable"
  | "out_of_stock"
  | "digital_conflict"
  | "unauthorized"
  | "failed";

export interface MergeOutcome {
  product_id: string;
  name: string;
  code: MergeCode;
  price_changed?: { from: number; to: number };
  error?: ApiError;
}

export interface MergeDeps {
  getCart: () => Promise<Order>;
  addItem: (orderId: string, body: { product_id: string; qty: number }) => Promise<Order>;
  isUnavailable?: (productId: string) => Promise<boolean>;
}

export interface MergeResult {
  outcomes: MergeOutcome[];
  removeIds: string[];
  stopped: null | "unauthorized" | "failed";
  cartCount: number;
}

export function outsideWindow(p: Product, now: number): boolean {
  const from = p.available_from ? Date.parse(p.available_from) : null;
  const until = p.available_until ? Date.parse(p.available_until) : null;
  return (from !== null && from > now) || (until !== null && until < now);
}

function classify(err: unknown): MergeCode {
  if (!(err instanceof ApiError)) return "failed";
  if (err.status === 401) return "unauthorized";
  if (err.status === 404 && err.code === "product_not_found") return "unavailable";
  if (err.status === 409 && err.code === "out_of_stock") return "out_of_stock";
  if (err.status === 409 && err.code === "digital_target_conflict") return "digital_conflict";
  if (err.status === 400 && err.code === "invalid_qty") return "already_in_cart";
  return "failed";
}

export async function mergeGuestCart(lines: GuestLine[], deps: MergeDeps): Promise<MergeResult> {
  const outcomes: MergeOutcome[] = [];
  const removeIds: string[] = [];
  let stopped: MergeResult["stopped"] = null;
  let cartCount = 0;

  let cart: Order;
  try {
    cart = await deps.getCart();
  } catch (err) {
    const code = classify(err);
    return { outcomes: [], removeIds, stopped: code === "unauthorized" ? code : "failed", cartCount };
  }
  const serverIds = new Set((cart.items ?? []).map((i) => i.product_id));
  cartCount = serverIds.size;

  for (const line of lines) {
    const base = { product_id: line.product_id, name: line.name };
    try {
      if (await deps.isUnavailable?.(line.product_id)) {
        outcomes.push({ ...base, code: "unavailable" });
        removeIds.push(line.product_id);
        continue;
      }
    } catch (err) {
      stopped = classify(err) === "unauthorized" ? "unauthorized" : "failed";
      break;
    }
    if (serverIds.has(line.product_id)) {
      outcomes.push({ ...base, code: "already_in_cart" });
      removeIds.push(line.product_id);
      continue;
    }
    try {
      const res = await deps.addItem(cart.id, { product_id: line.product_id, qty: line.qty });
      const to = res.items?.find((i) => i.product_id === line.product_id)?.unit_price;
      outcomes.push({
        ...base,
        code: "added",
        ...(to !== undefined && to !== line.unit_price ? { price_changed: { from: line.unit_price, to } } : {}),
      });
      removeIds.push(line.product_id);
      serverIds.add(line.product_id);
      cartCount += 1;
    } catch (err) {
      const code = classify(err);
      if (code === "unauthorized" || code === "failed") {
        stopped = code;
        break;
      }
      outcomes.push({ ...base, code, ...(code === "digital_conflict" ? { error: err as ApiError } : {}) });
      removeIds.push(line.product_id);
    }
  }
  return { outcomes, removeIds, stopped, cartCount };
}

interface MergeNoticeState {
  outcomes: MergeOutcome[];
  show: (outcomes: MergeOutcome[]) => void;
  dismiss: () => void;
}

export const useMergeNoticeStore = create<MergeNoticeState>()((set) => ({
  outcomes: [],
  show: (outcomes) => set({ outcomes }),
  dismiss: () => set({ outcomes: [] }),
}));

const realDeps: MergeDeps = {
  getCart: () => authFetch<Order>("/orders", { method: "POST" }),
  addItem: (orderId, body) =>
    authFetch<Order>(`/orders/${encodeURIComponent(orderId)}/items`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  isUnavailable: async (productId) => {
    try {
      // Relies on GET /products/:id staying unauthenticated: it 404s drafts and out-of-window products only for role "".
      await apiFetch<Product>(`/products/${encodeURIComponent(productId)}`);
      return false;
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) return true;
      throw err;
    }
  },
};

export function useGuestCartMerge(enabled: boolean) {
  const qc = useQueryClient();
  const { t } = useTranslation();
  const ran = useRef(false);

  useEffect(() => {
    if (!enabled || ran.current) return;
    const { lines, removeMany } = useGuestCart.getState();
    if (lines.length === 0) return;
    ran.current = true;
    void mergeGuestCart(lines, realDeps).then((res) => {
      removeMany(res.removeIds);
      qc.invalidateQueries({ queryKey: ordersKeys.all });
      if (res.cartCount > 0) useCartStore.getState().setCount(res.cartCount);
      const notable = res.outcomes.filter((o) => o.code !== "added" || o.price_changed);
      if (notable.length) useMergeNoticeStore.getState().show(notable);
      if (res.stopped === "failed") toast.error(t("gcart_sync_failed"));
    });
  }, [enabled, qc, t]);
}
