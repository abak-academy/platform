"use client";

import { API_BASE, ApiError } from "@/lib/api";

// Dedicated download helper for the Task 14 packing-slip endpoint so
// admin-orders.ts (its existing mutations) does not need another touch.
export async function downloadShippingLabel(orderId: string, size?: LabelSize): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;

  const res = await fetch(`${API_BASE}/admin/orders/${encodeURIComponent(orderId)}/label${size ? `?size=${size}` : ""}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) {
    throw new ApiError(
      `HTTP_${res.status}`,
      `Failed to fetch shipping label: ${res.status}`,
      res.status,
    );
  }

  saveShippingLabel(await res.blob(), `resi-${orderId}.pdf`);
}

function saveShippingLabel(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export type LabelSize = "100x150" | "roll80";
export type LabelNotPrintable = { order_id: string; reason: "not_found" | "no_tracking_number" | "shipment_failed" };

export async function downloadShippingLabels(orderIds: string[], size: LabelSize): Promise<LabelNotPrintable[] | null> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const res = await fetch(`${API_BASE}/admin/orders/labels`, {
    method: "POST",
    headers: { "Content-Type": "application/json", ...(token ? { Authorization: `Bearer ${token}` } : {}) },
    body: JSON.stringify({ order_ids: orderIds, size }),
  });
  if (res.status === 409) {
    const body = await res.json();
    if (body.code === "labels_not_printable" && Array.isArray(body.details)) return body.details;
  }
  if (!res.ok) throw new ApiError(`HTTP_${res.status}`, `Failed to fetch shipping labels: ${res.status}`, res.status);
  saveShippingLabel(await res.blob(), "resi-bulk.pdf");
  return null;
}
