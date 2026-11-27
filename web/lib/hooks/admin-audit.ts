"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AuditLogPage } from "@/lib/types";

export interface AuditLogFilters {
  actor_id?: string;
  from?: string;
  to?: string;
  target_type?: string;
  q?: string;
  page?: number;
  limit?: number;
}

export const adminAuditKeys = {
  all: ["admin", "audit"] as const,
  list: (filters?: AuditLogFilters) => {
    if (!filters) return [...adminAuditKeys.all, "list"] as const;
    const parts: string[] = [];
    if (filters.actor_id) parts.push(`actor=${filters.actor_id}`);
    if (filters.from) parts.push(`from=${filters.from}`);
    if (filters.to) parts.push(`to=${filters.to}`);
    if (filters.target_type) parts.push(`type=${filters.target_type}`);
    if (filters.q) parts.push(`q=${filters.q}`);
    if (filters.page) parts.push(`page=${filters.page}`);
    if (filters.limit) parts.push(`limit=${filters.limit}`);
    const key = parts.length ? parts.join("&") : "all";
    return [...adminAuditKeys.all, "list", key] as const;
  },
};

export function useAdminAuditLog(filters?: AuditLogFilters) {
  return useQuery({
    queryKey: adminAuditKeys.list(filters),
    placeholderData: keepPreviousData,
    queryFn: async () => {
      const params = new URLSearchParams();
      if (filters?.actor_id) params.set("actor_id", filters.actor_id);
      if (filters?.from) params.set("from", filters.from);
      if (filters?.to) params.set("to", filters.to);
      if (filters?.target_type) params.set("target_type", filters.target_type);
      if (filters?.q) params.set("q", filters.q);
      if (filters?.page) params.set("page", String(filters.page));
      if (filters?.limit) params.set("limit", String(filters.limit));
      const query = params.toString();
      const path = query ? `/admin/system/audit?${query}` : "/admin/system/audit";
      const res = await authFetch<AuditLogPage>(path);
      return { ...res, data: res.data ?? [] };
    },
  });
}
