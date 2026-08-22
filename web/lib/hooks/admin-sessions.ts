"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  ExamMonitorAvailable,
  SessionMonitorResponse,
  SessionViolationLog,
  SubmitResult,
} from "@/lib/types";

export const adminSessionsKeys = {
  all: ["adminSessions"] as const,
  monitorLists: () => [...adminSessionsKeys.all, "monitor"] as const,
  monitor: (examId: string) => [...adminSessionsKeys.monitorLists(), examId] as const,
  monitorAvailable: () => [...adminSessionsKeys.all, "monitor-available"] as const,
  violationLists: () => [...adminSessionsKeys.all, "violations"] as const,
  violations: (sessionId: string) => [...adminSessionsKeys.violationLists(), sessionId] as const,
};

export function useSessionMonitor(examId?: string) {
  return useQuery({
    queryKey: adminSessionsKeys.monitor(examId ?? ""),
    queryFn: () => {
      const params = new URLSearchParams({ exam_id: examId! });
      return authFetch<SessionMonitorResponse>(`/admin/sessions/monitor?${params.toString()}`);
    },
    enabled: Boolean(examId),
    refetchInterval: 15_000,
  });
}

export function useAvailableExamsForMonitor() {
  return useQuery({
    queryKey: adminSessionsKeys.monitorAvailable(),
    queryFn: () => authFetch<{ data: ExamMonitorAvailable[] }>("/admin/sessions/monitor/available"),
    refetchInterval: 15_000,
  });
}

export function useSessionViolations(sessionId?: string) {
  return useQuery({
    queryKey: adminSessionsKeys.violations(sessionId ?? ""),
    queryFn: () =>
      authFetch<{ data: SessionViolationLog[] }>(
        `/admin/sessions/${encodeURIComponent(sessionId!)}/violations`,
      ),
    enabled: Boolean(sessionId),
  });
}

export function useReopenSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      sessionId,
      extend_minutes,
    }: {
      sessionId: string;
      extend_minutes: number;
    }) =>
      authFetch<{ status: string }>(
        `/admin/sessions/${encodeURIComponent(sessionId)}/reopen`,
        { method: "POST", body: JSON.stringify({ extend_minutes }) },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminSessionsKeys.monitorLists() });
    },
  });
}

export function useForceSubmitSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) =>
      authFetch<SubmitResult>(
        `/admin/sessions/${encodeURIComponent(sessionId)}/force-submit`,
        { method: "POST" },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminSessionsKeys.monitorLists() });
    },
  });
}
