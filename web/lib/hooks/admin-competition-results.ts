"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { CompetitionResult, CompetitionResultPreview, CompetitionResultRow } from "@/lib/types";
import type { PresignedUpload } from "@/lib/hooks/admin-exam-grants";

export const competitionResultKeys = {
  all: ["admin", "competitionResults"] as const,
  detail: (productId: string) => [...competitionResultKeys.all, "detail", productId] as const,
  rows: (productId: string) => [...competitionResultKeys.all, "rows", productId] as const,
};

const base = (productId: string) => `/admin/products/${encodeURIComponent(productId)}/competition-result`;

export function useCompetitionResult(productId: string | undefined) {
  return useQuery({
    queryKey: competitionResultKeys.detail(productId ?? ""),
    enabled: Boolean(productId),
    queryFn: () => authFetch<CompetitionResult | null>(base(productId!)),
  });
}

export function useCompetitionResultRows(productId: string, enabled: boolean) {
  return useQuery({
    queryKey: competitionResultKeys.rows(productId),
    enabled,
    queryFn: () => authFetch<{ data: CompetitionResultRow[] }>(`${base(productId)}/results`),
  });
}

export function usePresignCompetitionResult(productId: string) {
  return useMutation({
    mutationFn: (filename: string) =>
      authFetch<PresignedUpload>(`${base(productId)}/presign`, {
        method: "POST",
        body: JSON.stringify({ filename }),
      }),
  });
}

export function usePreviewCompetitionResult(productId: string) {
  return useMutation({
    mutationFn: (key: string) =>
      authFetch<CompetitionResultPreview>(`${base(productId)}/preview`, {
        method: "POST",
        body: JSON.stringify({ key }),
      }),
  });
}

export function useApplyCompetitionResult(productId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (key: string) =>
      authFetch<CompetitionResult>(`${base(productId)}/apply`, {
        method: "POST",
        body: JSON.stringify({ key }),
      }),
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}

export function useSetCompetitionResultPublished(productId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (publish: boolean) =>
      authFetch<CompetitionResult>(`${base(productId)}/${publish ? "publish" : "unpublish"}`, { method: "POST" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}

export function useDeleteCompetitionResult(productId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () => authFetch<void>(base(productId), { method: "DELETE" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}
