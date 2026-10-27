"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  Affiliator,
  AffiliatorDashboard,
  AffiliatorOrderInput,
  AffiliatorOrderPreview,
  AffiliatorOrderRow,
  Order,
  Product,
  PromoCode,
} from "@/lib/types";

// Affiliator self-service reads and writes: GET/POST /affiliates/me/*.
export const affiliateKeys = {
  all: ["affiliate"] as const,
  me: () => [...affiliateKeys.all, "me"] as const,
  dashboard: (from?: string, to?: string) => [...affiliateKeys.all, "dashboard", from ?? "", to ?? ""] as const,
  orders: (status?: string) => [...affiliateKeys.all, "orders", status ?? "all"] as const,
  products: () => [...affiliateKeys.all, "products"] as const,
  promos: () => [...affiliateKeys.all, "promos"] as const,
};

export function useAffiliatorMe() {
  return useQuery({
    queryKey: affiliateKeys.me(),
    queryFn: () => authFetch<Affiliator>("/affiliates/me"),
  });
}

export function useAffiliatorDashboard(range: { from?: string; to?: string }) {
  return useQuery({
    queryKey: affiliateKeys.dashboard(range.from, range.to),
    queryFn: () => {
      const params = new URLSearchParams();
      if (range.from) params.set("from", range.from);
      if (range.to) params.set("to", range.to);
      const query = params.toString();
      return authFetch<AffiliatorDashboard>(query ? `/affiliates/me/dashboard?${query}` : "/affiliates/me/dashboard");
    },
  });
}

export function useAffiliatorOrders(opts?: { status?: string; cursor?: string; limit?: number }) {
  const { status, cursor, limit } = opts ?? {};
  return useQuery({
    queryKey: [...affiliateKeys.orders(status), cursor ?? "", limit ?? 20],
    queryFn: () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      const query = params.toString();
      return authFetch<{ data: AffiliatorOrderRow[]; next_cursor?: string }>(
        query ? `/affiliates/me/orders?${query}` : "/affiliates/me/orders",
      );
    },
  });
}

export function useAffiliatorProducts() {
  return useQuery({
    queryKey: affiliateKeys.products(),
    queryFn: async () => (await authFetch<{ data: Product[] }>("/affiliates/me/products")).data ?? [],
  });
}

export function useAffiliatorPromos() {
  return useQuery({
    queryKey: affiliateKeys.promos(),
    queryFn: async () => (await authFetch<{ data: PromoCode[] }>("/affiliates/me/promos")).data ?? [],
  });
}

export function usePreviewAffiliatorOrder() {
  return useMutation({
    mutationFn: (input: AffiliatorOrderInput) =>
      authFetch<AffiliatorOrderPreview>("/affiliates/me/orders/preview", {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

export function useCreateAffiliatorOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AffiliatorOrderInput) =>
      authFetch<Order>("/affiliates/me/orders", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: affiliateKeys.all });
    },
  });
}
