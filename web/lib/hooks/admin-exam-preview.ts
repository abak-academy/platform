"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { RegistrationExam } from "@/lib/types";

export function useExamPreview(id: string | undefined) {
  return useQuery({
    queryKey: ["admin", "exams", "preview", id ?? ""],
    queryFn: () => authFetch<RegistrationExam>(`/admin/exams/${encodeURIComponent(id!)}/preview`),
    enabled: Boolean(id),
  });
}
