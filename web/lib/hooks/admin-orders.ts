"use client";

import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  Order,
  AdminOrderFilterStatus,
  AdminOrderQuery,
  OrderSummary,
  OrderTracking,
} from "@/lib/types";

export const ORDERS_PAGE_SIZE = 20;

export const adminOrdersKeys = {
  all: ["admin", "orders"] as const,
  // The whole query object is part of the key, so any filter change starts a
  // fresh page 1 rather than appending to the previous filter's pages.
  list: (query: AdminOrderQuery) => [...adminOrdersKeys.all, "list", query] as const,
  // Keyed without status on purpose — see useAdminOrderSummary.
  summary: (scope: { q?: string; from?: string; to?: string }) =>
    [...adminOrdersKeys.all, "summary", scope] as const,
  detail: (id: string) => [...adminOrdersKeys.all, "detail", id] as const,
};

const FILTER_STATUS_MAP: Record<
  Exclude<AdminOrderFilterStatus, "all">,
  Order["status"]
> = {
  pending: "payment_pending",
  paid: "paid",
  processing: "processing",
  shipped: "shipped",
  failed: "payment_expired",
  cancelled: "cancelled",
};

function statusQueryParam(status?: AdminOrderFilterStatus): string | undefined {
  if (!status || status === "all") return undefined;
  return FILTER_STATUS_MAP[status];
}

function orderQueryParams(query: AdminOrderQuery): URLSearchParams {
  const params = new URLSearchParams();
  // Mutually exclusive: a queue is its own filter, not another status value.
  if (query.queue) {
    params.set("queue", query.queue);
  } else {
    const statusParam = statusQueryParam(query.status);
    if (statusParam) {
      params.set("status", statusParam);
    }
  }
  if (query.q) {
    params.set("q", query.q);
  }
  if (query.from) {
    params.set("from", query.from);
  }
  if (query.to) {
    params.set("to", query.to);
  }
  return params;
}

function idempotencyKey(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

export function useAdminOrders(query: AdminOrderQuery) {
  return useInfiniteQuery({
    queryKey: adminOrdersKeys.list(query),
    initialPageParam: "",
    queryFn: ({ pageParam }) => {
      const params = orderQueryParams(query);
      params.set("limit", String(ORDERS_PAGE_SIZE));
      if (pageParam) {
        params.set("cursor", pageParam);
      }
      return authFetch<{ data: Order[]; next_cursor?: string }>(`/admin/orders?${params.toString()}`);
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
  });
}

// Scoped to the search and date range, never to the status.
//
// The buckets ARE the per-status breakdown, and the toolbar renders a chip
// count from each. Passing the selected status through would filter the counts
// by the very chip they label: pick "Perlu konfirmasi" and every other chip
// drops to zero. Search and dates do narrow them, so the numbers still describe
// the rows on screen.
export function useAdminOrderSummary(query: AdminOrderQuery) {
  const scope = { q: query.q, from: query.from, to: query.to };
  return useQuery({
    queryKey: adminOrdersKeys.summary(scope),
    queryFn: () => {
      const params = new URLSearchParams();
      if (scope.q) params.set("q", scope.q);
      if (scope.from) params.set("from", scope.from);
      if (scope.to) params.set("to", scope.to);
      const qs = params.toString();
      return authFetch<OrderSummary>(
        qs ? `/admin/orders/summary?${qs}` : "/admin/orders/summary",
      );
    },
  });
}

export function useAdminOrder(id: string) {
  return useQuery({
    queryKey: adminOrdersKeys.detail(id),
    queryFn: () => authFetch<Order>(`/admin/orders/${encodeURIComponent(id)}`),
    enabled: Boolean(id),
  });
}

export function useConfirmOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, paymentProofUrl }: { id: string; paymentProofUrl: string }) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/confirm`, {
        method: "POST",
        headers: { "Idempotency-Key": idempotencyKey() },
        body: JSON.stringify({ payment_proof_url: paymentProofUrl }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

// Payment proofs are not served by the unauthenticated /files/* proxy — a bank
// transfer receipt must not be readable by anyone holding the key. The backend
// mints a short-lived link per request for an already-authorised admin, so the
// URL is fetched at click time rather than rendered into the page.
export function useFetchPaymentProofURL() {
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ url: string }>(`/admin/orders/${encodeURIComponent(id)}/payment-proof`),
  });
}

export function useFetchRefundProofURL() {
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ url: string }>(`/admin/orders/${encodeURIComponent(id)}/refund-proof`),
  });
}

export interface ShipOrderVars {
  id: string;
  // Both optional and sent together or not at all — a half-filled schedule
  // books an immediate pickup rather than failing the whole action.
  deliveryDate?: string;
  deliveryTime?: string;
}

export function useShipOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (vars: string | ShipOrderVars) => {
      const { id, deliveryDate, deliveryTime } =
        typeof vars === "string" ? ({ id: vars } as ShipOrderVars) : vars;
      const path = `/admin/orders/${encodeURIComponent(id)}/ship`;
      // No schedule means no body at all, exactly as before scheduling
      // existed — an immediate pickup is the unchanged default path and
      // should look identical on the wire.
      if (!deliveryDate || !deliveryTime) {
        return authFetch<{ message: string }>(path, { method: "POST" });
      }
      return authFetch<{ message: string }>(path, {
        method: "POST",
        body: JSON.stringify({ delivery_date: deliveryDate, delivery_time: deliveryTime }),
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useRefreshShipment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(
        `/admin/orders/${encodeURIComponent(id)}/shipment/refresh`,
        { method: "POST" },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useCancelShipment() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      authFetch<{ message: string }>(
        `/admin/orders/${encodeURIComponent(id)}/shipment/cancel`,
        { method: "POST", body: JSON.stringify({ reason }) },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useShipOrderManual() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, trackingNumber }: { id: string; trackingNumber: string }) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/ship-manual`, {
        method: "POST",
        body: JSON.stringify({ tracking_number: trackingNumber }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

// The refund itself is a manual bank transfer — the backend rejects the call
// without a receipt, so the key is required here rather than optional.
export function useRefundOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, refundProofUrl }: { id: string; refundProofUrl: string }) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/refund`, {
        method: "POST",
        body: JSON.stringify({ refund_proof_url: refundProofUrl }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useReconcileOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/reconcile`, {
        method: "POST",
        headers: { "Idempotency-Key": idempotencyKey() },
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useCompleteOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/complete`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminOrdersKeys.all });
    },
  });
}

export function useOrderTracking(orderId: string | null) {
  return useQuery({
    queryKey: [...adminOrdersKeys.all, "tracking", orderId],
    queryFn: () =>
      authFetch<OrderTracking>(`/admin/orders/${encodeURIComponent(orderId!)}/tracking`),
    enabled: Boolean(orderId),
    // The courier's scan log is the point of opening this; a cached one from
    // an earlier visit would answer the wrong question.
    staleTime: 0,
  });
}
