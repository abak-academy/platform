"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AdminRevenue } from "@/lib/types";

export interface RevenueRange {
  from?: string;
  to?: string;
}

export const adminRevenueKeys = {
  all: ["admin", "revenue"] as const,
  list: (range: RevenueRange = {}) =>
    range.from || range.to
      ? ([...adminRevenueKeys.all, "list", range] as const)
      : ([...adminRevenueKeys.all, "list"] as const),
};

export function useAdminRevenue(range: RevenueRange = {}) {
  return useQuery({
    queryKey: adminRevenueKeys.list(range),
    queryFn: async () => {
      const params = new URLSearchParams();
      if (range.from) {
        params.set("from", range.from);
      }
      if (range.to) {
        params.set("to", range.to);
      }
      const query = params.toString();
      return authFetch<AdminRevenue>(query ? `/admin/revenue?${query}` : "/admin/revenue");
    },
  });
}
