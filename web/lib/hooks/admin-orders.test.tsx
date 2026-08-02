import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminOrders,
  useAdminOrder,
  useConfirmOrder,
  useShipOrder,
  useShipOrderManual,
  useRefundOrder,
  useReconcileOrder,
  adminOrdersKeys,
} from "./admin-orders";
import type { Order } from "@/lib/types";

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

const sampleOrder: Order = {
  id: "o1",
  student_id: "s1",
  status: "payment_pending",
  subtotal: 100000,
  discount: 0,
  shipping_cost: 15000,
  total: 115000,
  items: [{ id: "i1", order_id: "o1", product_id: "p1", product_type: "book", name: "Buku A", unit_price: 100000, qty: 1, jumlah: 100000 }],
};

describe("admin-orders hooks", () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });

  it("useAdminOrders fetches GET /admin/orders and returns data", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [sampleOrder] });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminOrders(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders");
    expect(result.current.data).toEqual([sampleOrder]);
  });

  it("useAdminOrders maps status filter to backend enum", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [] });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminOrders("pending"), { wrapper });

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders?status=payment_pending"));
  });

  it("useAdminOrder fetches GET /admin/orders/:id", async () => {
    mockAuthFetch.mockResolvedValueOnce(sampleOrder);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminOrder("o1"), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders/o1");
    expect(result.current.data).toEqual(sampleOrder);
  });

  it("useConfirmOrder posts to /admin/orders/:id/confirm with idempotency key and the proof key", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "order confirmed" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useConfirmOrder(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "o1", paymentProofUrl: "payment_proof/admin1/proof.jpg" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/orders/o1/confirm",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
        body: JSON.stringify({ payment_proof_url: "payment_proof/admin1/proof.jpg" }),
      })
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: adminOrdersKeys.all });
  });

  it("useShipOrder posts to /admin/orders/:id/ship with no tracking_number (auto-book)", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "order shipped" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useShipOrder(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("o1");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders/o1/ship", { method: "POST" });
    const [, init] = mockAuthFetch.mock.calls[0];
    expect(init).not.toHaveProperty("body");
    expect(spy).toHaveBeenCalledWith({ queryKey: adminOrdersKeys.all });
  });

  it("useShipOrder surfaces the server's error message verbatim on rejection", async () => {
    mockAuthFetch.mockRejectedValueOnce(new Error("order has no persisted courier code"));

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useShipOrder(), { wrapper });

    await expect(
      act(async () => {
        await result.current.mutateAsync("o1");
      })
    ).rejects.toThrow("order has no persisted courier code");
  });

  it("useShipOrderManual posts tracking_number to /admin/orders/:id/ship-manual", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "order shipped" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useShipOrderManual(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "o1", trackingNumber: "JNE-123" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders/o1/ship-manual", {
      method: "POST",
      body: JSON.stringify({ tracking_number: "JNE-123" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminOrdersKeys.all });
  });

  // The refund is a manual bank transfer, so the receipt travels with the
  // request — the backend rejects the call without one.
  it("useRefundOrder posts to /admin/orders/:id/refund with the transfer receipt", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "order refunded" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useRefundOrder(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync({ id: "o1", refundProofUrl: "refund_proof/admin-1/trf.jpg" });
    });

    expect(mockAuthFetch).toHaveBeenCalledWith("/admin/orders/o1/refund", {
      method: "POST",
      body: JSON.stringify({ refund_proof_url: "refund_proof/admin-1/trf.jpg" }),
    });
    expect(spy).toHaveBeenCalledWith({ queryKey: adminOrdersKeys.all });
  });

  it("useReconcileOrder posts to /admin/orders/:id/reconcile with idempotency key", async () => {
    mockAuthFetch.mockResolvedValueOnce({ message: "order reconciled" });

    const { wrapper, queryClient } = wrapperFactory();
    const spy = vi.spyOn(queryClient, "invalidateQueries");
    const { result } = renderHook(() => useReconcileOrder(), { wrapper });

    await act(async () => {
      await result.current.mutateAsync("o1");
    });

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/orders/o1/reconcile",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ "Idempotency-Key": expect.any(String) }),
      })
    );
    expect(spy).toHaveBeenCalledWith({ queryKey: adminOrdersKeys.all });
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
