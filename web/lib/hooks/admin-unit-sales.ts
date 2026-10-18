"use client";

import { useQuery } from "@tanstack/react-query";
import { API_BASE, ApiError, authFetch } from "@/lib/api";
import type { PeriodRange } from "@/components/admin/PeriodBar";
import type { UnitSalesProductType, UnitSalesResponse } from "@/lib/types";

function rangeParams(range: PeriodRange) {
  const params = new URLSearchParams();
  if (range.from) params.set("from", range.from);
  if (range.to) params.set("to", range.to);
  return params;
}

export function useAdminUnitSales(range: PeriodRange) {
  return useQuery({
    queryKey: ["admin", "orders", "unit-sales", range],
    queryFn: () => {
      const query = rangeParams(range).toString();
      return authFetch<UnitSalesResponse>(`/admin/orders/unit-sales${query ? `?${query}` : ""}`);
    },
  });
}

export async function downloadUnitSalesCsv(type: UnitSalesProductType, range: PeriodRange): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const params = new URLSearchParams({ type });
  rangeParams(range).forEach((value, key) => params.set(key, value));
  const res = await fetch(`${API_BASE}/admin/orders/unit-sales/export?${params}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new ApiError(`HTTP_${res.status}`, `Failed to download unit sales: ${res.status}`, res.status);
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = `unit-terjual-${type}-${range.from || "semua-waktu"}_${range.to || "semua-waktu"}.csv`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
