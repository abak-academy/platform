"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { QuestionBundleState, QuestionBundleTemplate, QuestionBundleVariant } from "@/lib/types";

export const questionBundleKeys = {
  all: ["admin", "question-bundles"] as const,
  test: (testId: string, variant: QuestionBundleVariant) =>
    [...questionBundleKeys.all, "test", testId, variant] as const,
};

function testPath(testId: string, variant: QuestionBundleVariant): string {
  return `/admin/tests/${encodeURIComponent(testId)}/question-bundles/${variant}`;
}

async function serializeQuestionBundleTemplate(): Promise<QuestionBundleTemplate> {
  const response = await fetch("/api/admin/question-bundle-template", { method: "POST" });
  if (!response.ok) throw new Error("Gagal menyiapkan template PDF.");
  const body = (await response.json()) as { template: QuestionBundleTemplate };
  return body.template;
}

export function useQuestionBundleState(
  testId: string,
  variant: QuestionBundleVariant,
  enabled: boolean,
) {
  return useQuery({
    queryKey: questionBundleKeys.test(testId, variant),
    queryFn: () => authFetch<QuestionBundleState>(testPath(testId, variant)),
    enabled,
    refetchInterval: (query) => query.state.data?.status === "queued" ? 2000 : false,
  });
}

export function useRequestQuestionBundle(testId: string, variant: QuestionBundleVariant) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const template = await serializeQuestionBundleTemplate();
      return authFetch<QuestionBundleState>(testPath(testId, variant), {
        method: "POST",
        body: JSON.stringify({ template }),
      });
    },
    onSuccess: (state) => queryClient.setQueryData(questionBundleKeys.test(testId, variant), state),
  });
}

export function useQuestionBundleDownload(testId: string, variant: QuestionBundleVariant) {
  return useMutation({
    mutationFn: () => authFetch<{ url: string }>(`${testPath(testId, variant)}/download`),
  });
}
