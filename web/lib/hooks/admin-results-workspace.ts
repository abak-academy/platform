"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AdminResultDetail, ResultsWorkspaceResponse, ResultsWorkspaceAttempt } from "@/lib/types";

export const adminResultsWorkspaceKeys = {
  all: ["admin", "results_workspace"] as const,
  list: (examId: string, q?: string, schoolId?: string, cursor?: string, limit?: number) =>
    [...adminResultsWorkspaceKeys.all, "list", examId, q ?? "", schoolId ?? "", cursor ?? "initial", limit ?? 20] as const,
  attempts: (examId: string, registrationId: string) =>
    [...adminResultsWorkspaceKeys.all, "attempts", examId, registrationId] as const,
  detail: (examId: string, sessionId: string) =>
    [...adminResultsWorkspaceKeys.all, "detail", examId, sessionId] as const,
};

export function useResultsWorkspace(
  opts: { examId: string; q?: string; schoolId?: string; cursor?: string; limit?: number; enabled?: boolean },
) {
  const { examId, q, schoolId, cursor, limit, enabled } = opts;
  return useQuery({
    queryKey: adminResultsWorkspaceKeys.list(examId, q, schoolId, cursor, limit),
    enabled: enabled ?? true,
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (schoolId) params.set("school_id", schoolId);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      const query = params.toString();
      return authFetch<ResultsWorkspaceResponse>(
        `/admin/exams/${encodeURIComponent(examId)}/results-workspace${query ? `?${query}` : ""}`,
      );
    },
  });
}

export function useResultsWorkspaceAttempts(examId: string, registrationId: string) {
  return useQuery({
    queryKey: adminResultsWorkspaceKeys.attempts(examId, registrationId),
    queryFn: () =>
      authFetch<{ data: ResultsWorkspaceAttempt[] }>(
        `/admin/exams/${encodeURIComponent(examId)}/results-workspace/${encodeURIComponent(registrationId)}/attempts`,
      ),
    enabled: Boolean(examId) && Boolean(registrationId),
  });
}

export function useResultsWorkspaceDetail(examId: string, sessionId: string) {
  return useQuery({
    queryKey: adminResultsWorkspaceKeys.detail(examId, sessionId),
    queryFn: () =>
      authFetch<AdminResultDetail>(
        `/admin/exams/${encodeURIComponent(examId)}/results-workspace/sessions/${encodeURIComponent(sessionId)}`,
      ),
    enabled: Boolean(examId) && Boolean(sessionId),
  });
}
