"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { Product, ProductType } from "@/lib/types";

export const productsKeys = {
  all: ["products"] as const,
  list: (type?: ProductType) => [...productsKeys.all, "list", type ?? "all"] as const,
  detail: (id: string) => [...productsKeys.all, "detail", id] as const,
};

const MAX_PRODUCT_PAGES = 10;

export function useProducts(type?: ProductType) {
  return useQuery({
    queryKey: productsKeys.list(type),
    queryFn: async () => {
      const all: Product[] = [];
      let cursor: string | undefined;

      for (let page = 0; page < MAX_PRODUCT_PAGES; page++) {
        const params = new URLSearchParams();
        if (type) params.set("type", type);
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