import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor, act } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import * as api from "@/lib/api";
import {
  adminProductGroupingsKeys,
  useAdminProductGroupings,
  useAdminProductGrouping,
  useCreateProductGrouping,
  useUpdateProductGrouping,
  useDeleteProductGrouping,
  useReorderProductGroupings,
  useSetGroupingProducts,
} from "./admin-product-groupings";
import type { ProductGrouping, ProductGroupingDetail } from "@/lib/types";

function wrapperFactory() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}

afterEach(() => vi.restoreAllMocks());

describe("admin-product-groupings hooks", () => {
  it("useAdminProductGroupings fetches GET /admin/product-groupings", async () => {
    const groupings: ProductGrouping[] = [
      { id: "g1", name: "Paket UTBK", status: "draft", display_order: 0 },
    ];
    const fetchSpy = vi.spyOn(api, "authFetch").mockResolvedValueOnce({ data: groupings });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminProductGroupings(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchSpy).toHaveBeenCalledWith("/admin/product-groupings");
    expect(result.current.data).toEqual(groupings);
  });

  it("useAdminProductGrouping fetches GET /admin/product-groupings/:id", async () => {
    const detail: ProductGroupingDetail = {
      id: "g1",
      name: "Paket UTBK",
      status: "draft",
      display_order: 0,
      products: [],
    };
    const fetchSpy = vi.spyOn(api, "authFetch").mockResolvedValueOnce(detail);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminProductGrouping("g1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchSpy).toHaveBeenCalledWith("/admin/product-groupings/g1");
    expect(result.current.data).toEqual(detail);
  });

  it("useCreateProductGrouping posts to /admin/product-groupings and invalidates list", async () => {
    const created: ProductGrouping = { id: "g2", name: "Paket SNBT", status: "draft", display_order: 0 };
    vi.spyOn(api, "authFetch").mockResolvedValueOnce(created);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useCreateProductGrouping(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ name: "Paket SNBT" });
    });

    expect(api.authFetch).toHaveBeenCalledWith("/admin/product-groupings", {
      method: "POST",
      body: JSON.stringify({ name: "Paket SNBT" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminProductGroupingsKeys.list() });
  });

  it("useUpdateProductGrouping patches /admin/product-groupings/:id and invalidates list+detail", async () => {
    vi.spyOn(api, "authFetch").mockResolvedValueOnce({ id: "g1", name: "Paket UTBK 2", status: "published", display_order: 0 });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdateProductGrouping(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "g1", input: { status: "published" } });
    });

    expect(api.authFetch).toHaveBeenCalledWith("/admin/product-groupings/g1", {
      method: "PATCH",
      body: JSON.stringify({ status: "published" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminProductGroupingsKeys.list() });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminProductGroupingsKeys.detail("g1") });
  });

  it("useDeleteProductGrouping deletes /admin/product-groupings/:id and invalidates list", async () => {
    vi.spyOn(api, "authFetch").mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDeleteProductGrouping(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("g1");
    });

    expect(api.authFetch).toHaveBeenCalledWith("/admin/product-groupings/g1", { method: "DELETE" });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminProductGroupingsKeys.list() });
  });

  it("useReorderProductGroupings patches /admin/product-groupings/order with grouping_ids", async () => {
    vi.spyOn(api, "authFetch").mockResolvedValueOnce({ message: "product groupings reordered" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useReorderProductGroupings(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync(["g2", "g1"]);
    });

    expect(api.authFetch).toHaveBeenCalledWith("/admin/product-groupings/order", {
      method: "PATCH",
      body: JSON.stringify({ grouping_ids: ["g2", "g1"] }),
    });
  });

  it("useSetGroupingProducts puts /admin/product-groupings/:id/products with product_ids and invalidates detail", async () => {
    const detail: ProductGroupingDetail = { id: "g1", name: "Paket UTBK", status: "draft", display_order: 0, products: [] };
    vi.spyOn(api, "authFetch").mockResolvedValueOnce(detail);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useSetGroupingProducts(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "g1", productIds: ["p2", "p1"] });
    });

    expect(api.authFetch).toHaveBeenCalledWith("/admin/product-groupings/g1/products", {
      method: "PUT",
      body: JSON.stringify({ product_ids: ["p2", "p1"] }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminProductGroupingsKeys.detail("g1") });
  });
});
