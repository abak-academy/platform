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
    queryFn: async () => (await authFetch<{ data: CompetitionResult | null }>(base(productId!))).data,
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
      authFetch<PresignedUpload>(`${base(productId)}/presign?filename=${encodeURIComponent(filename)}`, { method: "POST" }),
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
    mutationFn: async (key: string) =>
      (
        await authFetch<{ data: CompetitionResult }>(`${base(productId)}/apply`, {
          method: "POST",
          body: JSON.stringify({ key }),
        })
      ).data,
    onSuccess: () => qc.invalidateQueries({ queryKey: competitionResultKeys.all }),
  });
}

export function useSetCompetitionResultPublished(productId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (publish: boolean) =>
      (await authFetch<{ data: CompetitionResult }>(`${base(productId)}/${publish ? "publish" : "unpublish"}`, { method: "POST" })).data,
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
