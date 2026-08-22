"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AssessmentResponse, AssessmentAttempt } from "@/lib/types";

export const adminAssessmentKeys = {
  all: ["admin", "assessment"] as const,
  list: (examId: string, q?: string, schoolId?: string, cursor?: string, limit?: number) =>
    [...adminAssessmentKeys.all, "list", examId, q ?? "", schoolId ?? "", cursor ?? "initial", limit ?? 20] as const,
  attempts: (examId: string, registrationId: string) =>
    [...adminAssessmentKeys.all, "attempts", examId, registrationId] as const,
};

export function useAssessment(
  opts: { examId: string; q?: string; schoolId?: string; cursor?: string; limit?: number; enabled?: boolean },
) {
  const { examId, q, schoolId, cursor, limit, enabled } = opts;
  return useQuery({
    queryKey: adminAssessmentKeys.list(examId, q, schoolId, cursor, limit),
    enabled: enabled ?? true,
    queryFn: () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (schoolId) params.set("school_id", schoolId);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      const query = params.toString();
      return authFetch<AssessmentResponse>(
        `/admin/exams/${encodeURIComponent(examId)}/assessment${query ? `?${query}` : ""}`,
      );
    },
  });
}

export function useAssessmentAttempts(examId: string, registrationId: string) {
  return useQuery({
    queryKey: adminAssessmentKeys.attempts(examId, registrationId),
    queryFn: () =>
      authFetch<{ data: AssessmentAttempt[] }>(
        `/admin/exams/${encodeURIComponent(examId)}/assessment/${encodeURIComponent(registrationId)}/attempts`,
      ),
    enabled: Boolean(examId) && Boolean(registrationId),
  });
}
