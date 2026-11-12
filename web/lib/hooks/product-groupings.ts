"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { ProductGrouping, ProductGroupingDetail } from "@/lib/types";

export const productGroupingsKeys = {
  all: ["product-groupings"] as const,
  list: () => [...productGroupingsKeys.all, "list"] as const,
  detail: (id: string) => [...productGroupingsKeys.all, "detail", id] as const,
};

export function useProductGroupings() {
  return useQuery({
    queryKey: productGroupingsKeys.list(),
    queryFn: async () => {
      const res = await apiFetch<{ data: ProductGrouping[] }>("/product-groupings");
      return res.data ?? [];
    },
  });
}

export function useProductGrouping(id: string) {
  return useQuery({
    queryKey: productGroupingsKeys.detail(id),
    queryFn: () => apiFetch<ProductGroupingDetail>(`/product-groupings/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}
