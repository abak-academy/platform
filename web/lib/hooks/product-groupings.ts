"use client";

import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api";
import type { ProductGrouping } from "@/lib/types";

export const productGroupingsKeys = {
  all: ["product-groupings"] as const,
  list: () => [...productGroupingsKeys.all, "list"] as const,
};

// Published groupings only — used to populate the catalog's grouping filter.
// Browsing a grouping's products goes through useProducts(type, groupingId)
// rather than a dedicated detail fetch.
export function useProductGroupings() {
  return useQuery({
    queryKey: productGroupingsKeys.list(),
    queryFn: async () => {
      const res = await apiFetch<{ data: ProductGrouping[] }>("/product-groupings");
      return res.data ?? [];
    },
  });
}
