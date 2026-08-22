"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  AdminStudent,
  StudentCredentials,
  StudentRegistrationInput,
  StudentRegistrationResult,
} from "@/lib/types";

export const adminStudentsKeys = {
  all: ["admin", "students"] as const,
  list: (status?: string, q?: string, cursor?: string, limit?: number, schoolId?: string, examId?: string, jenjang?: string, grade?: string) =>
    [...adminStudentsKeys.all, "list", status ?? "all", q ?? "", cursor ?? "initial", limit ?? 20, schoolId ?? "", examId ?? "", jenjang ?? "", grade ?? ""] as const,
};

export function useAdminStudents(
  opts?: { status?: string; q?: string; cursor?: string; limit?: number; schoolId?: string; examId?: string; jenjang?: string; grade?: string; enabled?: boolean }
) {
  const { status, q, cursor, limit, schoolId, examId, jenjang, grade, enabled } = opts ?? {};
  return useQuery({
    queryKey: adminStudentsKeys.list(status, q, cursor, limit, schoolId, examId, jenjang, grade),
    enabled: enabled ?? true,
    queryFn: async () => {
      const params = new URLSearchParams();
      if (status) params.set("status", status);
      if (q) params.set("q", q);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      if (schoolId) params.set("school_id", schoolId);
      if (examId) params.set("exam_id", examId);
      if (jenjang) params.set("jenjang", jenjang);
      if (grade) params.set("grade", grade);
      const query = params.toString();
      const path = query ? `/admin/students?${query}` : "/admin/students";
      return authFetch<{ data: AdminStudent[]; next_cursor?: string }>(path);
    },
  });
}

// scopeQuery returns the ?school_id= suffix for super_admin acting on a chosen
// school, or "" for admin_school (whose scope rides on the JWT).
function scopeQuery(schoolId?: string): string {
  if (!schoolId) return "";
  return `?school_id=${encodeURIComponent(schoolId)}`;
}

export function useRegisterStudent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ input, schoolId }: { input: StudentRegistrationInput; schoolId?: string }) =>
      authFetch<StudentRegistrationResult>(`/admin/students${scopeQuery(schoolId)}`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminStudentsKeys.all });
    },
  });
}

export function useChangeStudentStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status, schoolId }: { id: string; status: string; schoolId?: string }) =>
      authFetch<{ message: string }>(
        `/admin/students/${encodeURIComponent(id)}${scopeQuery(schoolId)}`,
        {
          method: "PATCH",
          body: JSON.stringify({ status }),
        }
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminStudentsKeys.all });
    },
  });
}

export function useReissueStudentCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, schoolId }: { id: string; schoolId?: string }) =>
      authFetch<StudentCredentials>(
        `/admin/students/${encodeURIComponent(id)}/credentials${scopeQuery(schoolId)}`
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminStudentsKeys.all });
    },
  });
}
