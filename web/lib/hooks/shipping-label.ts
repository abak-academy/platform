"use client";

import { API_BASE, ApiError } from "@/lib/api";

// Dedicated download helper for the Task 14 packing-slip endpoint so
// admin-orders.ts (its existing mutations) does not need another touch.
export async function downloadShippingLabel(orderId: string): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;

  const res = await fetch(`${API_BASE}/admin/orders/${encodeURIComponent(orderId)}/label`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) {
    throw new ApiError(
      `HTTP_${res.status}`,
      `Failed to fetch shipping label: ${res.status}`,
      res.status,
    );
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `resi-${orderId}.pdf`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
