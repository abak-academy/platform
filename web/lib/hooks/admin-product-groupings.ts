"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, authFetch } from "@/lib/api";
import type {
  AdminCreateProductGroupingInput,
  AdminProductGrouping,
  AdminUpdateProductGroupingInput,
  AdminUpdateProductLevelInput,
  Product,
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
      const res = await authFetch<{ data: AdminProductGrouping[] }>("/admin/product-groupings");
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
      authFetch<AdminProductGrouping>("/admin/product-groupings", {
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
      authFetch<ProductGroupingDetail>(`/admin/product-groupings/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: (_data, { id }) => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.detail(id) });
    },
  });
}

// useUpdateEditionLevel saves an edition's jenjang and grade range; the level
// is display-only, so it goes through the plain product PATCH.
export function useUpdateEditionLevel(groupingId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, input }: { productId: string; input: AdminUpdateProductLevelInput }) =>
      authFetch<Product>(`/admin/products/${encodeURIComponent(productId)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.detail(groupingId) });
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
    onMutate: async (groupingIds) => {
      await qc.cancelQueries({ queryKey: adminProductGroupingsKeys.list() });
      const previous = qc.getQueryData<AdminProductGrouping[]>(adminProductGroupingsKeys.list());
      if (previous) {
        const byId = new Map(previous.map((g) => [g.id, g]));
        qc.setQueryData(
          adminProductGroupingsKeys.list(),
          groupingIds.flatMap((id, i) => (byId.has(id) ? [{ ...byId.get(id)!, display_order: i }] : [])),
        );
      }
      return { previous };
    },
    onError: (_err, _ids, ctx) => {
      if (ctx?.previous) qc.setQueryData(adminProductGroupingsKeys.list(), ctx.previous);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: adminProductGroupingsKeys.list() });
    },
  });
}

const MISSING_LABELS: Record<string, string> = {
  status: "Status Published",
  participant_prefix: "Prefiks nomor peserta",
  "id.tagline": "Tagline (Indonesia)",
  "id.story": "Cerita (Indonesia)",
  "en.tagline": "Tagline (English)",
  "en.story": "Cerita (English)",
};

export function groupingMissingLabel(key: string): string {
  const row = /^schedule\[(\d+)\]\.label_en$/.exec(key);
  if (row) return `Label EN jadwal ke-${Number(row[1]) + 1}`;
  return MISSING_LABELS[key] ?? key;
}

// missing is the 422 list of what keeps an event from being visible.
export function groupingErrorParts(e: unknown): { message: string; missing: string[]; code?: string } {
  if (e instanceof ApiError) {
    const raw = e.body?.missing;
    const missing = Array.isArray(raw) ? raw.filter((m): m is string => typeof m === "string") : [];
    return { message: e.message, missing, code: e.code };
  }
  return { message: e instanceof Error ? e.message : "Terjadi kesalahan", missing: [] };
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
