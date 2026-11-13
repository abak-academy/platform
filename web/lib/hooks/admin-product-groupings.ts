"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  AdminCreateProductGroupingInput,
  AdminUpdateProductGroupingInput,
  ProductGrouping,
  ProductGroupingDetail,
} from "@/lib/types";

export const adminProductGroupingsKeys = {
  all: ["admin", "product-groupings"] as const,
  list: () => [...adminProductGroupingsKeys.all, "list"] as const,
  detail: (id: string) => [...adminProductGroupingsKeys.all, "detail", id] as const,
};

export function useAdminProductGroupings() {
  return useQuery({
    queryKey: adminProductGroupingsKeys.list(),
    queryFn: async () => {
      const res = await authFetch<{ data: ProductGrouping[] }>("/admin/product-groupings");
      return res.data ?? [];
    },
  });
}

export function useAdminProductGrouping(id: string) {
  return useQuery({
    queryKey: adminProductGroupingsKeys.detail(id),
    queryFn: () => authFetch<ProductGroupingDetail>(`/admin/product-groupings/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}

export function useCreateProductGrouping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminCreateProductGroupingInput) =>
      authFetch<ProductGrouping>("/admin/product-groupings", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
    },
  });
}

export function useUpdateProductGrouping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: AdminUpdateProductGroupingInput }) =>
      authFetch<ProductGrouping>(`/admin/product-groupings/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.detail(id) });
    },
  });
}

export function useDeleteProductGrouping() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<void>(`/admin/product-groupings/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
    },
  });
}

export function useReorderProductGroupings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (groupingIds: string[]) =>
      authFetch<{ message: string }>("/admin/product-groupings/order", {
        method: "PATCH",
        body: JSON.stringify({ grouping_ids: groupingIds }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
    },
  });
}

// useSetGroupingProducts replaces a grouping's member products in one call:
// the given order becomes their display order, and any current member left
// out is unassigned.
export function useSetGroupingProducts() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, productIds }: { id: string; productIds: string[] }) =>
      authFetch<ProductGroupingDetail>(`/admin/product-groupings/${encodeURIComponent(id)}/products`, {
        method: "PUT",
        body: JSON.stringify({ product_ids: productIds }),
      }),
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.detail(id) });
    },
  });
}
