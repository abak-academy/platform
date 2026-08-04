import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook, waitFor, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  useAdminOrders,
  useAdminOrder,
  useAdminOrderSummary,
  useConfirmOrder,
  useShipOrder,
  useShipOrderManual,
  useRefundOrder,
  useReconcileOrder,
  adminOrdersKeys,
  ORDERS_PAGE_SIZE,
} from "./admin-orders";
import type { Order, OrderSummary } from "@/lib/types";

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

  it("useAdminOrders fetches GET /admin/orders and returns the first page", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [sampleOrder] });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminOrders({ status: "all" }), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith(`/admin/orders?limit=${ORDERS_PAGE_SIZE}`);
    expect(result.current.data?.pages[0].data).toEqual([sampleOrder]);
  });

  it("useAdminOrders maps status filter to backend enum", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [] });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminOrders({ status: "pending" }), { wrapper });

    await waitFor(() =>
      expect(mockAuthFetch).toHaveBeenCalledWith(
        `/admin/orders?status=payment_pending&limit=${ORDERS_PAGE_SIZE}`
      )
    );
  });

  it("useAdminOrders sends status, q, from and to together", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [] });

    const { wrapper } = wrapperFactory();
    renderHook(
      () => useAdminOrders({ status: "paid", q: "budi santoso", from: "2026-07-01", to: "2026-07-31" }),
      { wrapper }
    );

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalledTimes(1));
    const params = new URLSearchParams(mockAuthFetch.mock.calls[0][0].split("?")[1]);
    expect(params.get("status")).toBe("paid");
    expect(params.get("q")).toBe("budi santoso");
    expect(params.get("from")).toBe("2026-07-01");
    expect(params.get("to")).toBe("2026-07-31");
  });

  // shipment_failed filters on shipment_status, not status — the order itself
  // is still "shipped", so no status param may be sent alongside it.
  it("useAdminOrders sends shipment=failed and no status for the shipment_failed filter", async () => {
    mockAuthFetch.mockResolvedValueOnce({ data: [] });

    const { wrapper } = wrapperFactory();
    renderHook(() => useAdminOrders({ status: "shipment_failed" }), { wrapper });

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalledTimes(1));
    const params = new URLSearchParams(mockAuthFetch.mock.calls[0][0].split("?")[1]);
    expect(params.get("shipment")).toBe("failed");
    expect(params.get("status")).toBeNull();
  });

  it("useAdminOrders pages forward with cursor and stops when next_cursor is empty", async () => {
    mockAuthFetch
      .mockResolvedValueOnce({ data: [sampleOrder], next_cursor: "cur-2" })
      .mockResolvedValueOnce({ data: [{ ...sampleOrder, id: "o2" }], next_cursor: "" });

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(() => useAdminOrders({ status: "all" }), { wrapper });

    await waitFor(() => expect(result.current.hasNextPage).toBe(true));

    await act(async () => {
      await result.current.fetchNextPage();
    });

    await waitFor(() => expect(result.current.hasNextPage).toBe(false));
    expect(mockAuthFetch).toHaveBeenLastCalledWith(
      `/admin/orders?limit=${ORDERS_PAGE_SIZE}&cursor=cur-2`
    );
    expect(result.current.data?.pages).toHaveLength(2);
  });

  it("useAdminOrders refetches when q changes", async () => {
    mockAuthFetch.mockResolvedValue({ data: [] });

    const { wrapper } = wrapperFactory();
    const { result, rerender } = renderHook(
      ({ q }: { q: string }) => useAdminOrders({ status: "all", q }),
      { wrapper, initialProps: { q: "budi" } }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockAuthFetch).toHaveBeenCalledTimes(1);

    rerender({ q: "siti" });

    await waitFor(() => expect(mockAuthFetch).toHaveBeenCalledTimes(2));
    expect(mockAuthFetch.mock.calls[1][0]).toContain("q=siti");
  });

  it("useAdminOrderSummary fetches GET /admin/orders/summary with the same filters", async () => {
    const summary: OrderSummary = {
      buckets: {
        needs_confirm: 3,
        ready_to_ship: 1,
        shipment_failed: 0,
        in_transit: 2,
        created_this_month: 9,
        completed_this_month: 4,
        total: 12,
      },
      top_products: [
        { product_id: "p1", name: "Buku A", product_type: "book", qty_sold: 7, order_count: 5 },
      ],
    };
    mockAuthFetch.mockResolvedValueOnce(summary);

    const { wrapper } = wrapperFactory();
    const { result } = renderHook(
      () => useAdminOrderSummary({ status: "all", q: "buku", from: "2026-07-01", to: "2026-07-31" }),
      { wrapper }
    );

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    expect(mockAuthFetch).toHaveBeenCalledWith(
      "/admin/orders/summary?q=buku&from=2026-07-01&to=2026-07-31"
    );
    expect(result.current.data).toEqual(summary);
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
