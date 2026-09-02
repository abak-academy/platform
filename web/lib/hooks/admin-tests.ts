"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import { adminBankQuestionsKeys } from "./admin-bank-questions";
import type {
  Test,
  TestDetail,
  QuestionListResponse,
  QuestionWithOptions,
  AdminCreateTestInput,
  AdminUpdateTestInput,
  AdminQuestionInput,
  AdminAttachQuestionsInput,
  AdminReorderQuestionsInput,
} from "@/lib/types";

export const adminTestsKeys = {
  all: ["admin", "tests"] as const,
  list: () => [...adminTestsKeys.all, "list"] as const,
  detail: (id: string) => [...adminTestsKeys.all, "detail", id] as const,
  questions: (testId: string) =>
    [...adminTestsKeys.all, "questions", testId] as const,
};

export interface AdminTestsFilters {
  subject?: string;
  topic?: string;
  q?: string;
  cursor?: string;
  limit?: number;
}

function buildListPath(filters?: AdminTestsFilters): string {
  if (!filters) return "/admin/tests";
  const params = new URLSearchParams();
  if (filters.subject) params.set("subject", filters.subject);
  if (filters.topic) params.set("topic", filters.topic);
  if (filters.q) params.set("q", filters.q);
  if (filters.cursor) params.set("cursor", filters.cursor);
  if (filters.limit !== undefined) params.set("limit", String(filters.limit));
  const query = params.toString();
  return query ? `/admin/tests?${query}` : "/admin/tests";
}

export function useAdminTests(filters?: AdminTestsFilters, enabled = true) {
  return useQuery({
    queryKey: [...adminTestsKeys.list(), filters ?? {}] as const,
    queryFn: () =>
      authFetch<{ data: Test[]; next_cursor?: string; total?: number }>(buildListPath(filters)),
    enabled,
  });
}

export function useTestDetail(id: string | undefined) {
  return useQuery({
    queryKey: adminTestsKeys.detail(id ?? ""),
    queryFn: () =>
      authFetch<TestDetail>(`/admin/tests/${encodeURIComponent(id!)}`),
    enabled: Boolean(id),
  });
}

export function useCreateTest() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminCreateTestInput) =>
      authFetch<Test>("/admin/tests", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.list() });
    },
  });
}

export function useUpdateTest(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminUpdateTestInput) =>
      authFetch<Test>(`/admin/tests/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.list() });
      qc.invalidateQueries({ queryKey: adminTestsKeys.detail(id) });
    },
  });
}

export function useDeleteTest(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      authFetch<void>(`/admin/tests/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.list() });
      qc.invalidateQueries({ queryKey: adminTestsKeys.detail(id) });
    },
  });
}

export function useTestQuestions(testId: string | undefined) {
  return useQuery({
    queryKey: adminTestsKeys.questions(testId ?? ""),
    queryFn: () =>
      authFetch<QuestionListResponse>(
        `/admin/tests/${encodeURIComponent(testId!)}/questions`,
      ),
    enabled: Boolean(testId),
  });
}

export function useSaveQuestion(testId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      question,
      input,
    }: {
      question?: string;
      input: AdminQuestionInput;
    }): Promise<QuestionWithOptions> => {
      if (question) {
        return authFetch<QuestionWithOptions>(
          `/admin/questions/${encodeURIComponent(question)}`,
          {
            method: "PATCH",
            body: JSON.stringify(input),
          },
        );
      }
      return authFetch<QuestionWithOptions>(
        `/admin/tests/${encodeURIComponent(testId)}/questions`,
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      );
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.questions(testId) });
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

export function useDeleteQuestion(testId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (questionId: string) =>
      authFetch<void>(`/admin/questions/${encodeURIComponent(questionId)}`, {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.questions(testId) });
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

export function useAttachQuestions(testId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminAttachQuestionsInput) =>
      authFetch<void>(
        `/admin/tests/${encodeURIComponent(testId)}/questions/attach`,
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.questions(testId) });
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

export function useDetachQuestion(testId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (questionId: string) =>
      authFetch<void>(
        `/admin/tests/${encodeURIComponent(testId)}/questions/${encodeURIComponent(questionId)}`,
        {
          method: "DELETE",
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.questions(testId) });
      qc.invalidateQueries({ queryKey: adminBankQuestionsKeys.lists() });
    },
  });
}

export function useReorderTestQuestions(testId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminReorderQuestionsInput) =>
      authFetch<void>(
        `/admin/tests/${encodeURIComponent(testId)}/questions/order`,
        {
          method: "PUT",
          body: JSON.stringify(input),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminTestsKeys.questions(testId) });
    },
  });
}
