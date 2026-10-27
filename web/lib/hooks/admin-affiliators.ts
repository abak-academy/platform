"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { Affiliator, AffiliatorBulkResult, AffiliatorDetail, AffiliatorInput } from "@/lib/types";

// Super-admin affiliator management: /admin/system/affiliators*.
export const adminAffiliatorsKeys = {
  all: ["admin", "affiliators"] as const,
  list: (status?: string) => [...adminAffiliatorsKeys.all, "list", status ?? "all"] as const,
  detail: (id: string) => [...adminAffiliatorsKeys.all, "detail", id] as const,
};

export function useAdminAffiliators(status?: string) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.list(status),
    queryFn: async () => {
      const params = new URLSearchParams({ limit: "100" });
      if (status) params.set("status", status);
      const res = await authFetch<{ data: Affiliator[]; next_cursor?: string }>(`/admin/system/affiliators?${params}`);
      return res.data ?? [];
    },
  });
}

export function useAdminAffiliator(id: string | null) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.detail(id ?? ""),
    enabled: Boolean(id),
    queryFn: () => authFetch<AffiliatorDetail>(`/admin/system/affiliators/${encodeURIComponent(id ?? "")}`),
  });
}

function useAffiliatorMutation<TInput, TResult>(fn: (input: TInput) => Promise<TResult>) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliatorsKeys.all });
      qc.invalidateQueries({ queryKey: ["admin", "accounts"] });
    },
  });
}

export function useCreateAffiliator() {
  return useAffiliatorMutation((input: AffiliatorInput) =>
    authFetch<Affiliator>("/admin/system/affiliators", { method: "POST", body: JSON.stringify(input) }),
  );
}

export function useBulkCreateAffiliators() {
  return useAffiliatorMutation((rows: AffiliatorInput[]) =>
    authFetch<{ results: AffiliatorBulkResult[] }>("/admin/system/affiliators/bulk", {
      method: "POST",
      body: JSON.stringify({ rows }),
    }),
  );
}

export function usePromoteAffiliator() {
  return useAffiliatorMutation(({ id, school_ids }: { id: string; school_ids: string[] }) =>
    authFetch<{ message: string }>(`/admin/system/affiliators/${encodeURIComponent(id)}/promote`, {
      method: "POST",
      body: JSON.stringify({ school_ids }),
    }),
  );
}

export function useSetAffiliatorSchools() {
  return useAffiliatorMutation(({ id, school_ids }: { id: string; school_ids: string[] }) =>
    authFetch<{ message: string }>(`/admin/system/affiliators/${encodeURIComponent(id)}/schools`, {
      method: "PUT",
      body: JSON.stringify({ school_ids }),
    }),
  );
}

export function useSetAffiliatorStatus() {
  return useAffiliatorMutation(({ id, status }: { id: string; status: "active" | "deactivated" }) =>
    authFetch<{ message: string }>(`/admin/system/accounts/${encodeURIComponent(id)}/status`, {
      method: "PATCH",
      body: JSON.stringify({ status }),
    }),
  );
}
