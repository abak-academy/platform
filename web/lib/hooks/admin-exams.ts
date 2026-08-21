"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { API_BASE, authFetch, ApiError } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { examKeys } from "@/lib/hooks/exam";
import type {
  ExamListItem,
  ExamDetail,
  CreateExamPayload,
  UpdateExamPayload,
  GradingSessionItem,
  GradingEssayItem,
  ExamLeaderboardEntry,
  ExamAnalytics,
  CertificateDesign,
  CertificateDesignInput,
  CertificateLayout,
  ExamRosterEntry,
} from "@/lib/types";

export const adminExamsKeys = {
  all: ["adminExams"] as const,
  lists: () => [...adminExamsKeys.all, "list"] as const,
  list: (filter: AdminExamsFilters | undefined) =>
    [...adminExamsKeys.lists(), filter ?? {}] as const,
  details: () => [...adminExamsKeys.all, "detail"] as const,
  detail: (id: string) => [...adminExamsKeys.details(), id] as const,
  gradingLists: () => [...adminExamsKeys.all, "grading"] as const,
  grading: (examId: string) => [...adminExamsKeys.gradingLists(), examId] as const,
  sessionEssays: (sessionId: string) =>
    [...adminExamsKeys.all, "sessionEssays", sessionId] as const,
  leaderboardLists: () => [...adminExamsKeys.all, "leaderboard"] as const,
  leaderboard: (examId: string, filter?: AdminExamsFilters) =>
    [...adminExamsKeys.leaderboardLists(), examId, filter ?? {}] as const,
  certificateDesign: (examId: string) =>
    [...adminExamsKeys.detail(examId), "certificate-design"] as const,
  rosters: () => [...adminExamsKeys.all, "roster"] as const,
  roster: (examId: string, filter?: ExamRosterFilters) =>
    [...adminExamsKeys.rosters(), examId, filter ?? {}] as const,
};

export interface GradeEssayInput {
  question_id: string;
  score: number;
  comment?: string;
}

export interface AdminExamsFilters {
  cursor?: string;
  limit?: number;
  q?: string;
  status?: string;
}

export interface ExamRosterFilters {
  cursor?: string;
  limit?: number;
  sort?: "asc" | "desc";
}

function buildListPath(filters?: AdminExamsFilters): string {
  if (!filters) return "/admin/exams";
  const params = new URLSearchParams();
  if (filters.cursor) params.set("cursor", filters.cursor);
  if (filters.limit !== undefined) params.set("limit", String(filters.limit));
  if (filters.q) params.set("q", filters.q);
  if (filters.status) params.set("status", filters.status);
  const query = params.toString();
  return query ? `/admin/exams?${query}` : "/admin/exams";
}

export function useExams(filter?: AdminExamsFilters) {
  return useQuery({
    queryKey: adminExamsKeys.list(filter),
    queryFn: () =>
      authFetch<{ data: ExamListItem[]; next_cursor?: string }>(buildListPath(filter)),
  });
}

export function useExam(id: string | undefined) {
  return useQuery({
    queryKey: adminExamsKeys.detail(id ?? ""),
    queryFn: () =>
      authFetch<ExamDetail>(`/admin/exams/${encodeURIComponent(id!)}`),
    enabled: Boolean(id),
  });
}

export function useCreateExam() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CreateExamPayload) =>
      authFetch<ExamDetail>("/admin/exams", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminExamsKeys.details() });
    },
  });
}

export function useUpdateExam(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateExamPayload) =>
      authFetch<ExamDetail>(`/admin/exams/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminExamsKeys.detail(id) });
    },
  });
}

export function useSetCertificateEnabled(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      authFetch<ExamDetail>(`/admin/exams/${encodeURIComponent(id)}/certificate-enabled`, {
        method: "PATCH",
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminExamsKeys.detail(id) });
      qc.invalidateQueries({ queryKey: adminExamsKeys.certificateDesign(id) });
    },
  });
}

export function useSetCardEnabled(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (enabled: boolean) =>
      authFetch<ExamDetail>(`/admin/exams/${encodeURIComponent(id)}/card-enabled`, {
        method: "PATCH",
        body: JSON.stringify({ enabled }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminExamsKeys.detail(id) });
    },
  });
}

export function useCertificateDesign(examId: string | undefined) {
  return useQuery({
    queryKey: adminExamsKeys.certificateDesign(examId ?? ""),
    queryFn: () =>
      authFetch<CertificateDesign>(
        `/admin/exams/${encodeURIComponent(examId!)}/certificate-design`,
      ),
    enabled: Boolean(examId),
  });
}

// serializeCertificateTemplate asks the Next.js server (app/api/admin/certificate-template)
// to render layout to self-contained HTML with {{token}} placeholders (async
// redesign 2026-08-02) — the FE remains the single source of markup, but the
// actual react-dom/server call only runs in a Node context, not the browser
// bundle. Called once per design save, right before the PUT that persists it.
export async function serializeCertificateTemplate(layout: CertificateLayout): Promise<string> {
  const res = await fetch("/api/admin/certificate-template", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ layout }),
  });
  if (!res.ok) {
    throw new ApiError(`HTTP_${res.status}`, `Failed to serialize certificate template: ${res.status}`, res.status);
  }
  const data = (await res.json()) as { html: string };
  return data.html;
}

export function useUpdateCertificateDesign(examId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: CertificateDesignInput) =>
      authFetch<CertificateDesign>(
        `/admin/exams/${encodeURIComponent(examId)}/certificate-design`,
        {
          method: "PUT",
          body: JSON.stringify(input),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.certificateDesign(examId) });
      qc.invalidateQueries({ queryKey: adminExamsKeys.detail(examId) });
    },
  });
}

export function usePresignCertificateAsset(examId: string) {
  return useMutation({
    mutationFn: ({ filename, content_type }: { filename: string; content_type: string }) =>
      authFetch<{ url: string; method: "PUT"; key: string }>(
        `/admin/exams/${encodeURIComponent(examId)}/certificate-assets/presign?filename=${encodeURIComponent(filename)}&content_type=${encodeURIComponent(content_type)}`,
        { method: "POST" },
      ),
  });
}

export function useReplaceExamTests(id: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (testIds: string[]) =>
      authFetch<void>(`/admin/exams/${encodeURIComponent(id)}/tests`, {
        method: "PUT",
        body: JSON.stringify(testIds),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.lists() });
      qc.invalidateQueries({ queryKey: adminExamsKeys.detail(id) });
    },
  });
}

export function useGradingSessions(examId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: adminExamsKeys.grading(examId ?? ""),
    queryFn: () =>
      authFetch<{ data: GradingSessionItem[] }>(
        `/admin/exams/${encodeURIComponent(examId!)}/grading`,
      ),
    enabled: Boolean(examId) && enabled,
  });
}

export function useSessionEssays(sessionId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: adminExamsKeys.sessionEssays(sessionId ?? ""),
    queryFn: () =>
      authFetch<{ data: GradingEssayItem[] }>(
        `/admin/sessions/${encodeURIComponent(sessionId!)}/essays`,
      ),
    enabled: Boolean(sessionId) && enabled,
  });
}

export function useGradeEssay(sessionId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: GradeEssayInput) =>
      authFetch<{ status: string; score: number }>(
        `/admin/sessions/${encodeURIComponent(sessionId)}/grade`,
        {
          method: "POST",
          body: JSON.stringify(input),
        },
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminExamsKeys.sessionEssays(sessionId) });
      qc.invalidateQueries({ queryKey: adminExamsKeys.gradingLists() });
      qc.invalidateQueries({ queryKey: examKeys.result(sessionId) });
    },
  });
}

export function useExamLeaderboard(
  examId: string | undefined,
  filter?: AdminExamsFilters,
  enabled = true,
) {
  return useQuery({
    queryKey: adminExamsKeys.leaderboard(examId ?? "", filter),
    queryFn: () => {
      const base = `/admin/exams/${encodeURIComponent(examId!)}/leaderboard`;
      if (!filter) return authFetch<{ data: ExamLeaderboardEntry[]; next_cursor?: string }>(base);
      const params = new URLSearchParams();
      if (filter.cursor) params.set("cursor", filter.cursor);
      if (filter.limit !== undefined) params.set("limit", String(filter.limit));
      return authFetch<{ data: ExamLeaderboardEntry[]; next_cursor?: string }>(`${base}?${params.toString()}`);
    },
    enabled: Boolean(examId) && enabled,
  });
}

export function useExamRoster(
  examId: string | undefined,
  filter?: ExamRosterFilters,
  enabled = true,
) {
  return useQuery({
    queryKey: adminExamsKeys.roster(examId ?? "", filter),
    queryFn: () => {
      const params = new URLSearchParams();
      if (filter?.cursor) params.set("cursor", filter.cursor);
      if (filter?.limit !== undefined) params.set("limit", String(filter.limit));
      if (filter?.sort) params.set("sort", filter.sort);
      const query = params.toString();
      return authFetch<{ data: ExamRosterEntry[]; next_cursor?: string }>(
        `/admin/exams/${encodeURIComponent(examId!)}/registrations${query ? `?${query}` : ""}`,
      );
    },
    enabled: Boolean(examId) && enabled,
  });
}

export async function exportExamRoster(examId: string): Promise<void> {
  const token = useAuthStore.getState().token;
  const res = await fetch(
    `${API_BASE}/admin/exams/${encodeURIComponent(examId)}/registrations/export`,
    { headers: token ? { Authorization: `Bearer ${token}` } : {} },
  );
  if (!res.ok) {
    throw new ApiError(
      `HTTP_${res.status}`,
      `Failed to export exam roster: ${res.status}`,
      res.status,
    );
  }
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = "roster.csv";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function useExamAnalytics(examId: string | undefined, enabled = true) {
  return useQuery({
    queryKey: [...adminExamsKeys.all, "analytics", examId ?? ""] as const,
    queryFn: () =>
      authFetch<ExamAnalytics>(
        `/admin/exams/${encodeURIComponent(examId!)}/analytics`,
      ),
    enabled: Boolean(examId) && enabled,
  });
}

// fetchCertificatePreview renders a preview PDF from an already-serialized
// document (async redesign 2026-08-02): html comes from rendering
// CertificateDocument with the editor's own live preview values baked in
// (mirrors lib/certificate-studio's previewContent, the same values the
// on-screen canvas already shows) — the backend is a thin pass-through to
// Gotenberg, never a stored PDF, never a second render engine.
export async function fetchCertificatePreview(examId: string, html: string): Promise<Blob> {
  const token = useAuthStore.getState().token;
  const res = await fetch(
    `${API_BASE}/admin/exams/${encodeURIComponent(examId)}/certificate-preview`,
    {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      body: JSON.stringify({ html }),
    },
  );
  if (!res.ok) {
    throw new ApiError(
      `HTTP_${res.status}`,
      `Failed to fetch certificate preview: ${res.status}`,
      res.status,
    );
  }
  return res.blob();
}
