import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminPromoCodes,
  useCreatePromoCode,
  useUpdatePromoCode,
  useDeletePromoCode,
  adminPromosKeys,
} from "./admin-promos";
import type { PromoCode } from "@/lib/types";

const mockAuthFetch = vi.fn();

vi.mock("@/lib/api", () => ({
  authFetch: (...args: Parameters<typeof mockAuthFetch>) => mockAuthFetch(...args),
  ApiError: class extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

describe("admin-promos hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  afterEach(() => {
    vi.clearAllTimers();
  });

  it("useAdminPromoCodes fetches GET /admin/promo-codes and returns data", async () => {
    const promos: PromoCode[] = [
      {
        id: "promo-1",
        code: "DISKON10",
        discount_percent: 10,
        used_count: 3,
        max_uses: 100,
        expires_at: "2026-12-31T00:00:00Z",
      },
    ];
    mockAuthFetch.mockResolvedValueOnce({ data: promos });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminPromoCodes(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes");
    expect(result.current.data).toEqual(promos);
  });

  it("useCreatePromoCode posts to /admin/promo-codes and invalidates list", async () => {
    const promo: PromoCode = {
      id: "promo-2",
      code: "DISKON20",
      discount_amount: 20000,
      used_count: 0,
    };
    mockAuthFetch.mockResolvedValueOnce(promo);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useCreatePromoCode(), { wrapper });

    const input = {
      code: "DISKON20",
      discount_amount: 20000,
      max_uses: 50,
      expires_at: "2026-12-31T00:00:00Z",
    };

    await act(async () => {
      await result.current.mutateAsync(input);
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes", {
      method: "POST",
      body: JSON.stringify(input),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminPromosKeys.list() });
  });

  it("useUpdatePromoCode puts /admin/promo-codes/:id and invalidates list", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "promo code updated" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useUpdatePromoCode(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({
        id: "promo-1",
        input: { max_uses: 200, expires_at: "2027-01-01T00:00:00Z" },
      });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes/promo-1", {
      method: "PUT",
      body: JSON.stringify({ max_uses: 200, expires_at: "2027-01-01T00:00:00Z" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminPromosKeys.list() });
  });

  // FR-13: is_public must reach the wire on both create and update, not be
  // dropped somewhere between the modal and the fetch call.
  it("useCreatePromoCode forwards is_public in the POST body", async () => {
    const promo: PromoCode = {
      id: "promo-3",
      code: "PUBLIKPROMO",
      discount_percent: 10,
      used_count: 0,
      is_public: true,
    };
    mockAuthFetch.mockResolvedValueOnce(promo);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useCreatePromoCode(), { wrapper });

    const input = { code: "PUBLIKPROMO", discount_percent: 10, is_public: true };

    await act(async () => {
      await result.current.mutateAsync(input);
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes", {
      method: "POST",
      body: JSON.stringify(input),
    });
  });

  it("useUpdatePromoCode forwards is_public: false in the PUT body", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "promo code updated" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useUpdatePromoCode(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "promo-1", input: { is_public: false } });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes/promo-1", {
      method: "PUT",
      body: JSON.stringify({ is_public: false }),
    });
  });

  it("useDeletePromoCode deletes /admin/promo-codes/:id and invalidates list", async () => {
    mockAuthFetch.mockResolvedValueOnce(undefined);

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useDeletePromoCode(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("promo-1");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/promo-codes/promo-1", {
      method: "DELETE",
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminPromosKeys.list() });
  });
});

function wrapperFactory() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return {
    wrapper: ({ children }: { children: React.ReactNode }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
    queryClient,
  };
}
