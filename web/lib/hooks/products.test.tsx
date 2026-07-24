import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useProducts } from "./products";
import * as api from "@/lib/api";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => vi.restoreAllMocks());

describe("useProducts", () => {
  it("follows next_cursor so products past the first page are not dropped", async () => {
    const fetchSpy = vi.spyOn(api, "apiFetch").mockImplementation(async (path: string) => {
      if (path.includes("cursor=p2")) {
        return { data: [{ id: "b", type: "medal", name: "Medali", price: 1 }] };
      }
      return {
        data: [{ id: "a", type: "book", name: "Buku", price: 1 }],
        next_cursor: "p2",
      };
    });

    const { result } = renderHook(() => useProducts(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((p) => p.id)).toEqual(["a", "b"]);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it("stops after ten pages so a bad cursor cannot loop forever", async () => {
    const fetchSpy = vi.spyOn(api, "apiFetch").mockResolvedValue({
      data: [{ id: "x", type: "book", name: "Buku", price: 1 }],
      next_cursor: "always",
    } as any);

    const { result } = renderHook(() => useProducts(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchSpy).toHaveBeenCalledTimes(10);
  });
});
