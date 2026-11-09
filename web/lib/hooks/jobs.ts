"use client";

import { useQuery } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { JobStatus } from "@/lib/types";

const TERMINAL_STATUSES = new Set(["succeeded", "failed"]);

const POLL_INTERVAL_MS = 2000;

export function useJobStatus(
  jobId: string | null | undefined,
  opts?: { enabled?: boolean; basePath?: string },
) {
  const enabled = (opts?.enabled ?? true) && Boolean(jobId);
  return useQuery({
    queryKey: ["admin", "job", jobId] as const,
    enabled,
    refetchInterval: (query) => {
      const data = query.state.data as JobStatus | undefined;
      if (data && TERMINAL_STATUSES.has(data.status)) return false;
      return POLL_INTERVAL_MS;
    },
    queryFn: () => authFetch<JobStatus>(`${opts?.basePath ?? "/admin/jobs"}/${jobId}`),
  });
}
