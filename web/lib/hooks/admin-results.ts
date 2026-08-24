"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch, API_BASE } from "@/lib/api";
import type { AdminResultRow, AdminResultDetail } from "@/lib/types";

export const adminResultsKeys = {
  all: ["admin", "results"] as const,
  list: (examId: string, q?: string, cursor?: string, limit?: number, schoolId?: string) =>
    [...adminResultsKeys.all, "list", examId, q ?? "", cursor ?? "initial", limit ?? 20, schoolId ?? ""] as const,
  detail: (sessionId: string, schoolId?: string) =>
    [...adminResultsKeys.all, "detail", sessionId, schoolId ?? ""] as const,
};

export function useAdminResults(
  opts: { examId: string; q?: string; cursor?: string; limit?: number; schoolId?: string; enabled?: boolean },
) {
  const { examId, q, cursor, limit, schoolId, enabled } = opts;
  return useQuery({
    queryKey: adminResultsKeys.list(examId, q, cursor, limit, schoolId),
    enabled: enabled ?? true,
    queryFn: async () => {
      const params = new URLSearchParams();
      params.set("exam_id", examId);
      if (q) params.set("q", q);
      if (cursor) params.set("cursor", cursor);
      if (limit) params.set("limit", String(limit));
      if (schoolId) params.set("school_id", schoolId);
      const query = params.toString();
      return authFetch<{ data: AdminResultRow[]; next_cursor?: string }>(
        `/admin/results?${query}`,
      );
    },
  });
}

export function useAdminResultDetail(sessionId: string, schoolId?: string) {
  return useQuery({
    queryKey: adminResultsKeys.detail(sessionId, schoolId),
    queryFn: () => {
      const qs = schoolId ? `?school_id=${encodeURIComponent(schoolId)}` : "";
      return authFetch<AdminResultDetail>(
        `/admin/results/${encodeURIComponent(sessionId)}${qs}`,
      );
    },
    enabled: Boolean(sessionId),
  });
}

export async function exportAdminResults(
  examId: string,
  schoolId?: string,
  q?: string,
  attempts: "latest" | "all" = "latest",
): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;

  const params = new URLSearchParams();
  params.set("exam_id", examId);
  if (schoolId) params.set("school_id", schoolId);
  if (q) params.set("q", q);
  if (attempts === "all") params.set("attempts", "all");
  const query = params.toString();

  const res = await fetch(`${API_BASE}/admin/results/export?${query}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });

  if (!res.ok) {
    throw new Error(`Export failed (${res.status})`);
  }

  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filenameFromContentDisposition(res.headers.get("Content-Disposition")) ?? "results.csv";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

function filenameFromContentDisposition(header: string | null): string | null {
  if (!header) return null;
  const match = /filename="?([^";]+)"?/i.exec(header);
  return match?.[1] ?? null;
}
