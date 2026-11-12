import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import * as api from "@/lib/api";
import { useProductGroupings } from "./product-groupings";
import type { ProductGrouping } from "@/lib/types";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => vi.restoreAllMocks());

describe("useProductGroupings", () => {
  it("fetches GET /product-groupings and returns the data array", async () => {
    const groupings: ProductGrouping[] = [
      { id: "g1", name: "Paket UTBK", status: "published", display_order: 0 },
    ];
    const fetchSpy = vi.spyOn(api, "apiFetch").mockResolvedValueOnce({ data: groupings });

    const { result } = renderHook(() => useProductGroupings(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchSpy).toHaveBeenCalledWith("/product-groupings");
    expect(result.current.data).toEqual(groupings);
  });

  it("returns an empty array when the server sends no data field", async () => {
    vi.spyOn(api, "apiFetch").mockResolvedValueOnce({});

    const { result } = renderHook(() => useProductGroupings(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
  });
});
