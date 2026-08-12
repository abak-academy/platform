"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AdminDashboard, ExamDashboard } from "@/lib/types";
import type { PeriodRange } from "@/components/admin/PeriodBar";

export function useAdminDashboard(range: PeriodRange) {
  const params = new URLSearchParams();
  if (range.from) params.set("from", range.from);
  if (range.to) params.set("to", range.to);
  const qs = params.toString();

  return useQuery({
    queryKey: ["admin", "dashboard", range.from ?? null, range.to ?? null],
    queryFn: () => authFetch<AdminDashboard>(`/admin/dashboard${qs ? `?${qs}` : ""}`),
    staleTime: 60 * 1000,
  });
}

export function useExamDashboard() {
  return useQuery({
    queryKey: ["admin", "dashboard", "exam"],
    queryFn: () => authFetch<ExamDashboard>("/admin/dashboard/exam"),
    staleTime: 30 * 1000,
  });
}
