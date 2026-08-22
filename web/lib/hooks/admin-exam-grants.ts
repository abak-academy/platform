"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { CrossSchoolStudent } from "@/lib/types";

// ── Query keys ────────────────────────────────────────────────────────────

export const examGrantKeys = {
  all: ["admin", "examGrants"] as const,
  search: (q?: string, schoolId?: string, jenjang?: string, grade?: string, examId?: string, cursor?: string, limit?: number) =>
    [...examGrantKeys.all, "search", q ?? "", schoolId ?? "", jenjang ?? "", grade ?? "", examId ?? "", cursor ?? "initial", limit ?? 20] as const,
};

// ── Types ─────────────────────────────────────────────────────────────────

export interface SearchStudentsAcrossSchoolsOpts {
  q?: string;
  schoolId?: string;
  jenjang?: string;
  grade?: string;
  examId?: string;
  cursor?: string;
  limit?: number;
  enabled?: boolean;
}

export interface GrantExamAccessInput {
  exam_id: string;
  student_ids: string[];
}

export interface GrantExamAccessResponse {
  granted_count: number;
  granted_students: Array<{
    id: string;
    name: string;
    username: string;
  }>;
}

export interface PresignedUpload {
  url: string;
  method: string;
  key: string;
}

export interface EnqueueBulkResult {
  job_id: string;
}

// ── Hooks ─────────────────────────────────────────────────────────────────

/**
 * Search students across all schools (super_admin only).
 * GET /admin/exam-grants/students/search
 * All filter params are optional.
 */
export function useSearchStudentsAcrossSchools(opts?: SearchStudentsAcrossSchoolsOpts) {
  const { q, schoolId, jenjang, grade, examId, cursor, limit, enabled } = opts ?? {};
  return useQuery({
    queryKey: examGrantKeys.search(q, schoolId, jenjang, grade, examId, cursor, limit),
    enabled: enabled ?? true,
    queryFn: async () => {
      const params = new URLSearchParams();
      if (q) params.set("q", q);
      if (schoolId) params.set("school_id", schoolId);
      if (jenjang) params.set("jenjang", jenjang);
      if (grade) params.set("grade", grade);
      if (examId) params.set("exam_id", examId);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      const query = params.toString();
      const path = query ? `/admin/exam-grants/students/search?${query}` : "/admin/exam-grants/students/search";
      return authFetch<{ data: CrossSchoolStudent[]; next_cursor?: string }>(path);
    },
  });
}

/**
 * Grant free exam access to selected students (super_admin only).
 * POST /admin/exam-grants
 */
export function useGrantExamAccess() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: GrantExamAccessInput) =>
      authFetch<GrantExamAccessResponse>("/admin/exam-grants", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: examGrantKeys.all });
    },
  });
}

/**
 * Request a presigned MinIO PUT URL for a super_admin exam-grant bulk CSV
 * upload (super_admin only).
 * POST /admin/exam-grants/bulk/presign?exam_id=&filename=&content_type=
 */
export function usePresignExamGrantBulkUpload(examId: string) {
  return useMutation({
    mutationFn: ({ filename, contentType }: { filename: string; contentType: string }) => {
      const qs = new URLSearchParams({
        exam_id: examId,
        filename,
        content_type: contentType,
      }).toString();
      return authFetch<PresignedUpload>(`/admin/exam-grants/bulk/presign?${qs}`, {
        method: "POST",
      });
    },
  });
}

/**
 * Enqueue an exam-grant-bulk job for an already-uploaded CSV (super_admin only).
 * POST /admin/exam-grants/bulk {exam_id, file_key} -> {job_id}
 */
export function useEnqueueExamGrantBulk() {
  return useMutation({
    mutationFn: ({ examId, fileKey }: { examId: string; fileKey: string }) =>
      authFetch<EnqueueBulkResult>("/admin/exam-grants/bulk", {
        method: "POST",
        body: JSON.stringify({ exam_id: examId, file_key: fileKey }),
      }),
  });
}
