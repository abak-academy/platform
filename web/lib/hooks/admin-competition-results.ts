"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { CompetitionResult, CompetitionResultPreview, CompetitionResultRow } from "@/lib/types";
import type { PresignedUpload } from "@/lib/hooks/admin-exam-grants";

export const competitionResultKeys = {
  all: ["admin", "competitionResults"] as const,
  detail: (examId: string) => [...competitionResultKeys.all, "detail", examId] as const,
  rows: (examId: string) => [...competitionResultKeys.all, "rows", examId] as const,
};

const base = (examId: string) => `/admin/exams/${encodeURIComponent(examId)}/competition-result`;

export function useCompetitionResult(examId: string | undefined) {
  return useQuery({
    queryKey: competitionResultKeys.detail(examId ?? ""),
    enabled: Boolean(examId),
    queryFn: async () => (await authFetch<{ data: CompetitionResult | null }>(base(examId!))).data,
  });
}

export function useCompetitionResultRows(examId: string, enabled: boolean) {
  return useQuery({
    queryKey: competitionResultKeys.rows(examId),
    enabled,
    queryFn: () => authFetch<{ data: CompetitionResultRow[] }>(`${base(examId)}/results`),
  });
}

export function usePresignCompetitionResult(examId: string) {
  return useMutation({
    mutationFn: (filename: string) =>
      authFetch<PresignedUpload>(`${base(examId)}/presign?filename=${encodeURIComponent(filename)}`, { method: "POST" }),
  });
}

export function usePreviewCompetitionResult(examId: string) {
  return useMutation({
    mutationFn: (key: string) =>
      authFetch<CompetitionResultPreview>(`${base(examId)}/preview`, {
        method: "POST",
        body: JSON.stringify({ key }),
      }),
  });
}

export function useApplyCompetitionResult(examId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (key: string) =>
      (
        await authFetch<{ data: CompetitionResult }>(`${base(examId)}/apply`, {
          method: "POST",
          body: JSON.stringify({ key }),
        })
      ).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}

export function useSetCompetitionResultPublished(examId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (publish: boolean) =>
      (await authFetch<{ data: CompetitionResult }>(`${base(examId)}/${publish ? "publish" : "unpublish"}`, { method: "POST" })).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}

export function useDeleteCompetitionResult(examId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => authFetch<void>(base(examId), { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}
