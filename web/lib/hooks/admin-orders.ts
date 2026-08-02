"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { Order, AdminOrderFilterStatus } from "@/lib/types";

export const adminOrdersKeys = {
  all: ["admin", "orders"] as const,
  list: (status?: AdminOrderFilterStatus) =>
    status && status !== "all"
      ? ([...adminOrdersKeys.all, "list", status] as const)
      : ([...adminOrdersKeys.all, "list"] as const),
  detail: (id: string) => [...adminOrdersKeys.all, "detail", id] as const,
};

const FILTER_STATUS_MAP: Record<Exclude<AdminOrderFilterStatus, "all">, Order["status"]> = {
  pending: "payment_pending",
  paid: "paid",
  processing: "processing",
  shipped: "shipped",
  failed: "payment_expired",
  refunded: "cancelled",
};

function statusQueryParam(status?: AdminOrderFilterStatus): string | undefined {
  if (!status || status === "all") return undefined;
  return FILTER_STATUS_MAP[status];
}

function idempotencyKey(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

export function useAdminOrders(status?: AdminOrderFilterStatus) {
  return useQuery({
    queryKey: adminOrdersKeys.list(status),
    queryFn: async () => {
      const params = new URLSearchParams();
      const statusParam = statusQueryParam(status);
      if (statusParam) {
        params.set("status", statusParam);
      }
      const query = params.toString();
      const path = query ? `/admin/orders?${query}` : "/admin/orders";
      const res = await authFetch<{ data: Order[]; next_cursor?: string }>(path);
      return res.data ?? [];
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

export function useShipOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/ship`, {
        method: "POST",
      }),
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

export function useRefundOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<{ message: string }>(`/admin/orders/${encodeURIComponent(id)}/refund`, {
        method: "POST",
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
