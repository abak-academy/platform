"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { ExternalResultRow, ExternalRound, ExternalRoundPreview } from "@/lib/types";
import type { PresignedUpload } from "@/lib/hooks/admin-exam-grants";

export const externalRoundKeys = {
  all: ["admin", "externalRounds"] as const,
  list: (examId: string) => [...externalRoundKeys.all, "list", examId] as const,
  results: (roundId: string) => [...externalRoundKeys.all, "results", roundId] as const,
};

export function useExternalRounds(examId: string | undefined) {
  return useQuery({
    queryKey: externalRoundKeys.list(examId ?? ""),
    enabled: Boolean(examId),
    queryFn: () => authFetch<{ data: ExternalRound[] }>(`/admin/exams/${encodeURIComponent(examId!)}/external-rounds`),
  });
}

export function useExternalRoundResults(roundId: string | null) {
  return useQuery({
    queryKey: externalRoundKeys.results(roundId ?? ""),
    enabled: Boolean(roundId),
    queryFn: () => authFetch<{ data: ExternalResultRow[] }>(`/admin/external-rounds/${roundId}/results`),
  });
}

export function usePresignExternalRound(examId: string) {
  return useMutation({
    mutationFn: (filename: string) =>
      authFetch<PresignedUpload>(
        `/admin/exams/${encodeURIComponent(examId)}/external-rounds/presign?filename=${encodeURIComponent(filename)}`,
        { method: "POST" },
      ),
  });
}

export interface ExternalRoundFileInput {
  key: string;
  round_label: string;
}

export function usePreviewExternalRound(examId: string) {
  return useMutation({
    mutationFn: (input: ExternalRoundFileInput) =>
      authFetch<ExternalRoundPreview>(`/admin/exams/${encodeURIComponent(examId)}/external-rounds/preview`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

export function useApplyExternalRound(examId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: ExternalRoundFileInput) =>
      authFetch<{ data: ExternalRound }>(`/admin/exams/${encodeURIComponent(examId)}/external-rounds/apply`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: externalRoundKeys.all }),
  });
}

export function useSetExternalRoundPublished() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ roundId, publish }: { roundId: string; publish: boolean }) =>
      authFetch<{ data: ExternalRound }>(`/admin/external-rounds/${roundId}/${publish ? "publish" : "unpublish"}`, {
        method: "POST",
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: externalRoundKeys.all }),
  });
}

export function useDeleteExternalRound() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (roundId: string) => authFetch<void>(`/admin/external-rounds/${roundId}`, { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: externalRoundKeys.all }),
  });
}
