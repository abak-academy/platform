"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, authFetch } from "@/lib/api";
import type { ActivePromoCode, CheckoutResult, CourierRate, Order, PromoValidation } from "@/lib/types";

export const ordersKeys = {
  all: ["orders"] as const,
  list: () => [...ordersKeys.all, "list"] as const,
  cart: () => [...ordersKeys.all, "cart"] as const,
  detail: (id: string) => [...ordersKeys.all, "detail", id] as const,
  activePromos: () => [...ordersKeys.all, "active-promos"] as const,
};

// FR-14: additive listing next to the manual promo input — must never block
// manual entry, so callers read isError/isLoading and simply render nothing
// on failure rather than surfacing an error state.
export function useActivePromoCodes() {
  return useQuery({
    queryKey: ordersKeys.activePromos(),
    queryFn: async () => {
      const res = await authFetch<{ data: ActivePromoCode[] }>("/promo-codes/active");
      return res.data ?? [];
    },
    retry: false,
  });
}

export function useOrders() {
  return useQuery({
    queryKey: ordersKeys.list(),
    queryFn: async () => {
      const res = await authFetch<{ data: Order[]; next_cursor?: string }>(`/orders`);
      return res.data ?? [];
    },
  });
}

export function useOrder(id: string) {
  return useQuery({
    queryKey: ordersKeys.detail(id),
    queryFn: () => authFetch<Order>(`/orders/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}

export function useCart() {
  return useQuery({
    queryKey: ordersKeys.cart(),
    queryFn: () => authFetch<Order>(`/orders`, { method: "POST" }),
  });
}

interface AddToCartInput {
  productId: string;
  qty?: number;
  cartId?: string;
}

interface AddItemBody {
  product_id: string;
  qty: number;
}

export function useAddToCart() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({ productId, qty = 1, cartId }: AddToCartInput) => {
      let orderId = cartId;
      if (!orderId) {
        const cart = await authFetch<Order>("/orders", {
          method: "POST",
          body: JSON.stringify({ status: "cart" }),
        });
        orderId = cart.id;
      }
      return authFetch<Order>(`/orders/${encodeURIComponent(orderId!)}/items`, {
        method: "POST",
        body: JSON.stringify({ product_id: productId, qty } satisfies AddItemBody),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ordersKeys.cart() });
      qc.invalidateQueries({ queryKey: ordersKeys.list() });
    },
  });
}

interface RemoveCartItemInput {
  orderId: string;
  itemId: string;
}

export function useRemoveCartItem() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ orderId, itemId }: RemoveCartItemInput) =>
      authFetch<void>(`/orders/${encodeURIComponent(orderId)}/items/${encodeURIComponent(itemId)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ordersKeys.cart() });
      qc.invalidateQueries({ queryKey: ordersKeys.list() });
    },
  });
}

interface UpdateCartItemQtyInput {
  orderId: string;
  itemId: string;
  qty: number;
}

export function useUpdateCartItemQty() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ orderId, itemId, qty }: UpdateCartItemQtyInput) =>
      authFetch<void>(`/orders/${encodeURIComponent(orderId)}/items/${encodeURIComponent(itemId)}`, {
        method: "PATCH",
        body: JSON.stringify({ qty }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ordersKeys.cart() });
    },
  });
}

interface ValidatePromoInput {
  code: string;
  orderId?: string;
  subtotal?: number;
}

export function useValidatePromo() {
  return useMutation({
    mutationFn: (input: ValidatePromoInput) =>
      authFetch<PromoValidation>(`/promo-codes/validate`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

export function useCheckout(basePath?: string) {
  const qc = useQueryClient();
  const pathPrefix = basePath ?? "/orders";
  return useMutation({
    mutationFn: (orderId: string) =>
      authFetch<CheckoutResult>(`${pathPrefix}/${encodeURIComponent(orderId)}/checkout`, {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ordersKeys.cart() });
      qc.invalidateQueries({ queryKey: ordersKeys.list() });
    },
  });
}

export function useRetryPayment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (orderId: string) =>
      authFetch<CheckoutResult>(`/orders/${encodeURIComponent(orderId)}/retry`, {
        method: "POST",
        headers: { "Idempotency-Key": crypto.randomUUID() },
      }),
    onSuccess: (data, orderId) => {
      qc.invalidateQueries({ queryKey: ordersKeys.detail(orderId) });
      qc.invalidateQueries({ queryKey: ordersKeys.list() });
    },
  });
}

interface ShippingRatesInput {
  destination_postal_code: string;
  weight_grams: number;
}

export function useShippingRates() {
  return useMutation({
    mutationFn: (input: ShippingRatesInput) =>
      authFetch<{ rates: CourierRate[] }>(`/orders/shipping`, {
        method: "POST",
        body: JSON.stringify(input),
      }).then((res) => res.rates),
  });
}

interface PatchCartShippingAddress {
  penerima: string;
  telepon: string;
  alamat: string;
  kode_pos: string;
  provinsi_id: string;
  kota_id: string;
  kecamatan_id: string;
  // Region names snapshotted at checkout so admin can read one address without
  // three ID lookups, and a later rename cannot rewrite a historical order.
  provinsi?: string;
  kota?: string;
  kecamatan?: string;
  // Free text from the buyer ("titip di satpam"). shipping_address is a JSONB
  // passthrough the backend stores verbatim, so this needs no column of its own.
  catatan?: string;
}

interface PatchCartInput {
  orderId: string;
  // Absent when only the address is being saved — the buyer has not chosen a
  // courier yet. The backend keeps whatever the order already holds.
  courier?: string;
  service?: string;
  shipping_cost?: number;
  province_id: string;
  city_id: string;
  district_id: string;
  kode_pos: string | null;
  promo_code?: string;
  shipping_address?: PatchCartShippingAddress;
}

export function usePatchCart() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ orderId, courier, service, shipping_cost, province_id, city_id, district_id, kode_pos, promo_code, shipping_address }: PatchCartInput) => {
      const body: Record<string, unknown> = {
        courier,
        service,
        shipping_cost,
        province_id,
        city_id,
        district_id,
        kode_pos,
      };
      if (promo_code !== undefined) {
        body.promo_code = promo_code;
      }
      if (shipping_address !== undefined) {
        body.shipping_address = shipping_address;
      }
      return authFetch<void>(`/orders/${encodeURIComponent(orderId)}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ordersKeys.cart() });
    },
    // order_changed means the cart was mutated while this patch was being
    // computed, so the cached copy this form was built from is stale — refetch
    // it, or the buyer retries against figures the server already rejected.
    onError: (err) => {
      if (err instanceof ApiError && err.code === "order_changed") {
        qc.invalidateQueries({ queryKey: ordersKeys.cart() });
      }
    },
  });
}