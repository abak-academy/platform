"use client";

import { useEffect } from "react";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { Product, AdminCreateProductInput, AdminUpdateProductInput } from "@/lib/types";

export const adminProductsKeys = {
  all: ["admin", "products"] as const,
  list: () => [...adminProductsKeys.all, "list"] as const,
};

// The list endpoint pages at 20 rows and hands back a cursor. Requesting only
// the first page dropped everything past row 20 with no way to reach it, and
// because the product table orders by a random uuid primary key the 20 that did
// arrive were an arbitrary subset, not the newest.
//
// Every page is pulled here rather than behind a "load more" button because the
// products page narrows by type in the browser: filtering a partial list would
// quietly under-report, and a role could see an empty page while owning rows
// further down.
export function useAdminProducts() {
  const query = useInfiniteQuery({
    queryKey: adminProductsKeys.list(),
    initialPageParam: "",
    queryFn: async ({ pageParam }) => {
      const path = pageParam
        ? `/admin/products?cursor=${encodeURIComponent(pageParam)}`
        : "/admin/products";
      return authFetch<{ data: Product[]; next_cursor?: string }>(path);
    },
    // The server omits the cursor on the final page, which is what ends this.
    getNextPageParam: (last) => last.next_cursor || undefined,
  });

  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query;
  useEffect(() => {
    if (hasNextPage && !isFetchingNextPage) fetchNextPage();
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  return {
    ...query,
    data: query.data?.pages.flatMap((page) => page.data ?? []),
  };
}

export function useCreateProduct() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminCreateProductInput) =>
      authFetch<Product>("/admin/products", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductsKeys.list() });
    },
  });
}

export function useUpdateProduct() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, input }: { id: string; input: AdminUpdateProductInput }) =>
      authFetch<Product>(`/admin/products/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductsKeys.list() });
    },
  });
}

export function usePublishProduct() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/products/${encodeURIComponent(id)}/publish`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductsKeys.list() });
    },
  });
}

export function useDeleteProduct() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<void>(`/admin/products/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminProductsKeys.list() });
    },
  });
}
