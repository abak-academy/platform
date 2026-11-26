"use client";

import { useMemo } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Product, ProductType } from "@/lib/types";

export const productsKeys = {
  all: ["products"] as const,
  list: (type?: ProductType, groupingId?: string) =>
    [...productsKeys.all, "list", type ?? "all", groupingId ?? "all"] as const,
  detail: (id: string) => [...productsKeys.all, "detail", id] as const,
};

const MAX_PRODUCT_PAGES = 10;

export function useProducts(type?: ProductType, groupingId?: string) {
  return useQuery({
    queryKey: productsKeys.list(type, groupingId),
    queryFn: async () => {
      const all: Product[] = [];
      let cursor: string | undefined;

      for (let page = 0; page < MAX_PRODUCT_PAGES; page++) {
        const params = new URLSearchParams();
        if (type) params.set("type", type);
        if (groupingId) params.set("grouping_id", groupingId);
        if (cursor) params.set("cursor", cursor);
        const qs = params.toString() ? `?${params.toString()}` : "";

        const res = await apiFetch<{ data: Product[]; next_cursor?: string }>(`/products${qs}`);
        all.push(...(res.data ?? []));

        if (!res.next_cursor) break;
        cursor = res.next_cursor;
      }

      return all;
    },
  });
}

export function useProduct(id: string) {
  return useQuery({
    queryKey: productsKeys.detail(id),
    queryFn: () => apiFetch<Product>(`/products/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}

// For a cart/order with a handful of distinct products, where each one's
// own detail (its agreement_checkboxes, in particular) is needed without a
// bulk-by-ids endpoint.
export function useProductsByIds(ids: string[]) {
  // ids.join: `ids` is typically a fresh array every render (mapped from
  // cart/order items), so this is the stable key both useQueries' query list
  // and the uniqueIds below are derived from.
  const idsKey = ids.filter(Boolean).join(",");
  const uniqueIds = useMemo(() => Array.from(new Set(idsKey ? idsKey.split(",") : [])), [idsKey]);
  const results = useQueries({
    queries: uniqueIds.map((id) => ({
      queryKey: productsKeys.detail(id),
      queryFn: () => apiFetch<Product>(`/products/${encodeURIComponent(id)}`),
    })),
  });
  // Keyed on each result's own product id (not array identity, which
  // useQueries never guarantees stable) — without this, a brand new Map
  // every render feeds a brand new "rows" array into AgreementChecklist's
  // effect on every render, which calls back into the parent's setState
  // with a brand new object every time and never settles.
  const dataSignature = results.map((r) => r.data?.id ?? "").join(",");
  const byId = useMemo(() => {
    const map = new Map<string, Product>();
    uniqueIds.forEach((id, i) => {
      const data = results[i]?.data;
      if (data) map.set(id, data);
    });
    return map;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [uniqueIds, dataSignature]);
  return { byId, isLoading: results.some((r) => r.isLoading) };
}