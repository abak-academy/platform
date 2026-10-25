"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { API_BASE, ApiError, authFetch } from "@/lib/api";
import type {
  AffiliateAdminDetail,
  AffiliateAdminListPage,
  AffiliateAdminPayout,
  AffiliateAdminPayoutPage,
  AffiliateAdjustment,
  AffiliateAppealRow,
  AffiliateFlagRow,
  AffiliateJobResult,
  AffiliateLiabilityReport,
  AffiliateProductOverride,
  AffiliateReportSummary,
  AffiliateSettingsInput,
  AffiliateSettingsView,
  AffiliateViolation,
} from "@/lib/types";

export const adminAffiliateKeys = {
  all: ["admin", "affiliates"] as const,
  list: (p: AdminAffiliateListParams) => [...adminAffiliateKeys.all, "list", p] as const,
  detail: (id: string) => [...adminAffiliateKeys.all, "detail", id] as const,
  payouts: (status: string) => [...adminAffiliateKeys.all, "payouts", status] as const,
  settings: () => [...adminAffiliateKeys.all, "settings"] as const,
  overrides: () => [...adminAffiliateKeys.all, "overrides"] as const,
  flags: (status: string) => [...adminAffiliateKeys.all, "flags", status] as const,
  appeals: (status: string) => [...adminAffiliateKeys.all, "appeals", status] as const,
  report: (from: string, to: string) => [...adminAffiliateKeys.all, "report", from, to] as const,
  liability: () => [...adminAffiliateKeys.all, "liability"] as const,
};

export interface AdminAffiliateListParams {
  status?: string;
  level?: string;
  q?: string;
  page?: number;
}

function qs(params: Record<string, string | number | undefined>): string {
  const sp = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") sp.set(key, String(value));
  }
  const s = sp.toString();
  return s ? `?${s}` : "";
}

const post = (body: unknown) => ({ method: "POST", body: JSON.stringify(body) });

export function useAdminAffiliates(params: AdminAffiliateListParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.list(params),
    queryFn: () =>
      authFetch<AffiliateAdminListPage>(
        `/admin/affiliates${qs({ status: params.status, level: params.level, q: params.q, page: params.page })}`,
      ),
  });
}

export function useAdminAffiliate(id: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.detail(id),
    queryFn: () => authFetch<AffiliateAdminDetail>(`/admin/affiliates/${encodeURIComponent(id)}`),
  });
}

function useAffiliateAction<TInput, TResult = unknown>(path: (input: TInput) => string, init: (input: TInput) => RequestInit) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: TInput) => authFetch<TResult>(path(input), init(input)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliateKeys.all });
    },
  });
}

export function useDecideAffiliateApproval() {
  return useAffiliateAction<{ id: string; decision: "approve" | "reject"; reason?: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/approval`,
    ({ decision, reason }) => post({ decision, reason: reason ?? "" }),
  );
}

export function useChangeAffiliateStatus() {
  return useAffiliateAction<{ id: string; status: string; reason: string; days?: number }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/status`,
    ({ status, reason, days }) => ({ method: "PATCH", body: JSON.stringify({ status, reason, days }) }),
  );
}

export function useFreezeAffiliateBalance() {
  return useAffiliateAction<{ id: string; days: number; reason: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/freeze`,
    ({ days, reason }) => post({ days, reason }),
  );
}

export function useUnfreezeAffiliateBalance() {
  return useAffiliateAction<{ id: string; reason: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/unfreeze`,
    ({ reason }) => post({ reason }),
  );
}

export function useChangeAffiliateLevel() {
  return useAffiliateAction<{ id: string; level: string; lock_until?: string; reason: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/level`,
    ({ level, lock_until, reason }) => post({ level, lock_until: lock_until ?? "", reason }),
  );
}

export function useCreateAffiliateAdjustment() {
  return useAffiliateAction<
    { id: string; category: string; amount: number; reason: string; note?: string },
    AffiliateAdjustment
  >(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/adjustments`,
    ({ category, amount, reason, note }) => post({ category, amount, reason, note }),
  );
}

export function useAdminAffiliatePayouts(status: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.payouts(status),
    queryFn: () => authFetch<AffiliateAdminPayoutPage>(`/admin/affiliate-payouts${qs({ status })}`),
  });
}

export function useProcessAffiliatePayout() {
  return useAffiliateAction<string, AffiliateAdminPayout>(
    (id) => `/admin/affiliate-payouts/${encodeURIComponent(id)}/process`,
    () => ({ method: "POST" }),
  );
}

export function useDecideAffiliatePayout() {
  return useAffiliateAction<
    { id: string; decision: "paid" | "reject"; proof_url?: string; reason?: string },
    AffiliateAdminPayout
  >(
    ({ id }) => `/admin/affiliate-payouts/${encodeURIComponent(id)}/decide`,
    ({ decision, proof_url, reason }) => post({ decision, proof_url: proof_url ?? "", reason: reason ?? "" }),
  );
}

export async function downloadAffiliatePayoutsCsv(): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const res = await fetch(`${API_BASE}/admin/affiliate-payouts/export`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new ApiError(`HTTP_${res.status}`, `Failed to download payouts: ${res.status}`, res.status);
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = "affiliate-payouts.csv";
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}

export function useAdminAffiliateSettings() {
  return useQuery({
    queryKey: adminAffiliateKeys.settings(),
    queryFn: () => authFetch<AffiliateSettingsView>("/admin/affiliate-settings"),
  });
}

export function useUpdateAffiliateSettings() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AffiliateSettingsInput) =>
      authFetch<AffiliateSettingsView>("/admin/affiliate-settings", {
        method: "PUT",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliateKeys.settings() });
    },
  });
}

export function useAdminAffiliateOverrides() {
  return useQuery({
    queryKey: adminAffiliateKeys.overrides(),
    queryFn: async () => {
      const res = await authFetch<{ data: AffiliateProductOverride[] }>("/admin/affiliate-product-overrides");
      return res.data ?? [];
    },
  });
}

export function useUpsertAffiliateOverride() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, percentage }: { productId: string; percentage: number }) =>
      authFetch<AffiliateProductOverride>(`/admin/affiliate-product-overrides/${encodeURIComponent(productId)}`, {
        method: "PUT",
        body: JSON.stringify({ percentage }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliateKeys.overrides() });
    },
  });
}

export function useDeleteAffiliateOverride() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (productId: string) =>
      authFetch<void>(`/admin/affiliate-product-overrides/${encodeURIComponent(productId)}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliateKeys.overrides() });
    },
  });
}

export function useAdminAffiliateFlags(status: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.flags(status),
    queryFn: async () => {
      const res = await authFetch<{ data: AffiliateFlagRow[] }>(`/admin/affiliate-flags${qs({ status })}`);
      return res.data ?? [];
    },
  });
}

export function useReviewAffiliateFlag() {
  return useAffiliateAction<{ id: string; status: string; note?: string }, AffiliateFlagRow>(
    ({ id }) => `/admin/affiliate-flags/${encodeURIComponent(id)}/review`,
    ({ status, note }) => post({ status, note }),
  );
}

export function useAdminAffiliateAppeals(status: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.appeals(status),
    queryFn: async () => {
      const res = await authFetch<{ data: AffiliateAppealRow[] }>(`/admin/affiliate-appeals${qs({ status })}`);
      return res.data ?? [];
    },
  });
}

export function useDecideAffiliateAppeal() {
  return useAffiliateAction<{ id: string; outcome: "upheld" | "rejected"; note?: string }, AffiliateViolation>(
    ({ id }) => `/admin/affiliate-appeals/${encodeURIComponent(id)}/decide`,
    ({ outcome, note }) => post({ outcome, note }),
  );
}

export function useAffiliateReportSummary(from: string, to: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.report(from, to),
    queryFn: () => authFetch<AffiliateReportSummary>(`/admin/affiliate-reports/summary${qs({ from, to })}`),
  });
}

export function useAffiliateLiabilityReport() {
  return useQuery({
    queryKey: adminAffiliateKeys.liability(),
    queryFn: () => authFetch<AffiliateLiabilityReport>("/admin/affiliate-reports/liability"),
  });
}

export function useRunAffiliateJob() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { scope: string; quarter?: string }) =>
      authFetch<AffiliateJobResult>("/admin/affiliate-jobs/run", post(input)),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminAffiliateKeys.all });
    },
  });
}
