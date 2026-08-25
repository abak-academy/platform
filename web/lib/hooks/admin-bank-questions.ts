"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch, authFetchMultipart, API_BASE, ApiError } from "@/lib/api";
import { questionBundleKeys } from "./question-bundles";
import type {
  AdminQuestionImportResponse,
  AdminQuestionInput,
  BankQuestionListResponse,
  QuestionFormat,
  QuestionWithOptions,
} from "@/lib/types";

export const adminBankQuestionsKeys = {
  all: ["admin", "bank-questions"] as const,
  lists: () => [...adminBankQuestionsKeys.all, "list"] as const,
  list: (filters: BankQuestionsFilters | undefined) =>
    [...adminBankQuestionsKeys.lists(), filters ?? {}] as const,
  detail: (id: string) => [...adminBankQuestionsKeys.all, "detail", id] as const,
};

export interface BankQuestionsFilters {
  format?: QuestionFormat;
  topic_id?: string;
  search?: string;
  cursor?: string;
  page?: number;
  limit?: number;
}

function buildListPath(filters?: BankQuestionsFilters): string {
  const params = new URLSearchParams();
  if (filters?.format) params.set("format", filters.format);
  if (filters?.topic_id) params.set("topic_id", filters.topic_id);
  if (filters?.search) params.set("search", filters.search);
  if (filters?.cursor) params.set("cursor", filters.cursor);
  if (filters?.page !== undefined) params.set("page", String(filters.page));
  if (filters?.limit !== undefined) params.set("limit", String(filters.limit));
  const query = params.toString();
  return query ? `/admin/questions?${query}` : "/admin/questions";
}

export function useBankQuestions(filters?: BankQuestionsFilters) {
  return useQuery({
    queryKey: adminBankQuestionsKeys.list(filters),
    queryFn: () => authFetch<BankQuestionListResponse>(buildListPath(filters)),
  });
}

export function useCreateBankQuestion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminQuestionInput) =>
      authFetch<QuestionWithOptions>("/admin/questions", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

export function useUpdateBankQuestion(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminQuestionInput) =>
      authFetch<QuestionWithOptions>(`/admin/questions/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.detail(id) });
      qc.invalidateQueries({ queryKey: questionBundleKeys.all });
    },
  });
}

export function useDeleteBankQuestion() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      authFetch<void>(`/admin/questions/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
      qc.invalidateQueries({ queryKey: questionBundleKeys.all });
    },
  });
}

export function useImportBankQuestions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (file: File) => {
      const formData = new FormData();
      formData.set("file", file);
      return authFetchMultipart<AdminQuestionImportResponse>("/admin/questions/import", {
        method: "POST",
        body: formData,
      });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

// The template CSV is generated server-side from the same header list the
// parser requires (FR-11), so the client can't build it from a literal.
export function useDownloadQuestionImportTemplate() {
  return useMutation({
    mutationFn: async (): Promise<Blob> => {
      const { useAuthStore } = await import("@/stores/auth");
      const token = useAuthStore.getState().token;
      const res = await fetch(`${API_BASE}/admin/questions/import-template`, {
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });
      if (!res.ok) {
        throw new ApiError(
          `HTTP_${res.status}`,
          `Failed to download import template: ${res.status}`,
          res.status,
        );
      }
      return res.blob();
    },
  });
}
