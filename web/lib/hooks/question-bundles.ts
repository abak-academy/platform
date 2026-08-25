"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { QuestionBundle } from "@/lib/types";

export const questionBundleKeys = {
  all: ["admin", "question-bundles"] as const,
  detail: (id: string) => [...questionBundleKeys.all, id] as const,
};

export type QuestionBundleScope = "exam" | "test";

export interface CreateQuestionBundleInput {
  include_answer_key: boolean;
}

function createPath(scope: QuestionBundleScope, scopeId: string): string {
  const encoded = encodeURIComponent(scopeId);
  return scope === "exam"
    ? `/admin/exams/${encoded}/question-bundle`
    : `/admin/tests/${encoded}/question-bundle`;
}

export function useCreateQuestionBundle(scope: QuestionBundleScope, scopeId: string) {
  return useMutation({
    mutationFn: (input: CreateQuestionBundleInput) =>
      authFetch<QuestionBundle>(createPath(scope, scopeId), {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

export function useQuestionBundle(
  bundleId: string | undefined,
  enabled: boolean,
  refetchInterval: number | false | ((query: { state: { data?: QuestionBundle } }) => number | false) = false,
) {
  return useQuery({
    queryKey: questionBundleKeys.detail(bundleId ?? ""),
    queryFn: () => authFetch<QuestionBundle>(`/admin/question-bundles/${encodeURIComponent(bundleId!)}`),
    enabled: Boolean(bundleId) && enabled,
    refetchInterval,
  });
}

export function useQuestionBundleDownload() {
  return useMutation({
    mutationFn: (bundleId: string) =>
      authFetch<{ url: string }>(`/admin/question-bundles/${encodeURIComponent(bundleId)}/download`),
  });
}
