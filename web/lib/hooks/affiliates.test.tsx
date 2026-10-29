import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import {
  useAffiliatorDashboard,
  useAffiliatorMe,
  useAffiliatorOrders,
  useAffiliatorProducts,
  useAffiliatorPromos,
  useCreateAffiliatorOrder,
  usePreviewAffiliatorOrder,
} from "./affiliates";

const authFetch = vi.fn();
vi.mock("@/lib/api", () => ({ authFetch: (...args: unknown[]) => authFetch(...args) }));

function wrapper() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return ({ children }: { children: ReactNode }) => <QueryClientProvider client={qc}>{children}</QueryClientProvider>;
}

describe("affiliate hooks", () => {
  beforeEach(() => authFetch.mockReset());

  it("reads the signed-in affiliator profile", async () => {
    authFetch.mockResolvedValue({ id: "a1" });
    const { result } = renderHook(() => useAffiliatorMe(), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.data).toEqual({ id: "a1" }));
    expect(authFetch).toHaveBeenCalledWith("/affiliates/me");
  });

  it("passes the period to the dashboard only when set", async () => {
    authFetch.mockResolvedValue({});
    const all = renderHook(() => useAffiliatorDashboard({}), { wrapper: wrapper() });
    await waitFor(() => expect(all.result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenLastCalledWith("/affiliates/me/dashboard");

    const ranged = renderHook(() => useAffiliatorDashboard({ from: "2026-10-01", to: "2026-10-31" }), { wrapper: wrapper() });
    await waitFor(() => expect(ranged.result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenLastCalledWith("/affiliates/me/dashboard?from=2026-10-01&to=2026-10-31");
  });

  it("builds the orders query from status, cursor and limit", async () => {
    authFetch.mockResolvedValue({ data: [] });
    const { result } = renderHook(() => useAffiliatorOrders({ status: "paid", cursor: "c1", limit: 20 }), { wrapper: wrapper() });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(authFetch).toHaveBeenCalledWith("/affiliates/me/orders?status=paid&cursor=c1&limit=20");
  });

  it("unwraps the product and promo lists", async () => {
    authFetch.mockResolvedValue({ data: [{ id: "p1" }] });
    const products = renderHook(() => useAffiliatorProducts(), { wrapper: wrapper() });
    const promos = renderHook(() => useAffiliatorPromos(), { wrapper: wrapper() });

    await waitFor(() => expect(products.result.current.data).toEqual([{ id: "p1" }]));
    await waitFor(() => expect(promos.result.current.data).toEqual([{ id: "p1" }]));
    expect(authFetch).toHaveBeenCalledWith("/affiliates/me/products");
    expect(authFetch).toHaveBeenCalledWith("/affiliates/me/promos");
  });

  it("posts the same body to preview and create", async () => {
    authFetch.mockResolvedValue({});
    const body = { school_id: "s1", product_id: "p1", student_ids: ["st1"], promo_code: "AFF10" };
    const preview = renderHook(() => usePreviewAffiliatorOrder(), { wrapper: wrapper() });
    const create = renderHook(() => useCreateAffiliatorOrder(), { wrapper: wrapper() });

    await act(async () => {
      await preview.result.current.mutateAsync(body);
      await create.result.current.mutateAsync(body);
    });

    expect(authFetch).toHaveBeenNthCalledWith(1, "/affiliates/me/orders/preview", { method: "POST", body: JSON.stringify(body) });
    expect(authFetch).toHaveBeenNthCalledWith(2, "/affiliates/me/orders", { method: "POST", body: JSON.stringify(body) });
  });
});
