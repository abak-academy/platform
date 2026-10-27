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

interface AdminQueuePage<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

export const adminAffiliateKeys = {
  all: ["admin", "affiliates"] as const,
  list: (p: AdminAffiliateListParams) => [...adminAffiliateKeys.all, "list", p] as const,
  detail: (id: string) => [...adminAffiliateKeys.all, "detail", id] as const,
  payouts: (p: AdminQueueParams) => [...adminAffiliateKeys.all, "payouts", p] as const,
  settings: () => [...adminAffiliateKeys.all, "settings"] as const,
  overrides: () => [...adminAffiliateKeys.all, "overrides"] as const,
  flags: (p: AdminQueueParams) => [...adminAffiliateKeys.all, "flags", p] as const,
  appeals: (p: AdminQueueParams) => [...adminAffiliateKeys.all, "appeals", p] as const,
  report: (from: string, to: string) => [...adminAffiliateKeys.all, "report", from, to] as const,
  liability: () => [...adminAffiliateKeys.all, "liability"] as const,
};

export interface AdminAffiliateListParams {
  status?: string;
  level?: string;
  q?: string;
  page?: number;
}

// The payout, flag and appeal queues share one query: search, status, date
// range and paging. An empty status means every status.
export interface AdminQueueParams {
  q?: string;
  status?: string;
  from?: string;
  to?: string;
  page?: number;
  per_page?: number;
}

function queueQuery(p: AdminQueueParams): string {
  return qs({ q: p.q, status: p.status || "all", from: p.from, to: p.to, page: p.page, per_page: p.per_page });
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

export function useCreateAffiliateViolation() {
  return useAffiliateAction<
    { id: string; category: string; description: string; sanction: string; days?: number; level?: string },
    AffiliateViolation
  >(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/violations`,
    ({ category, description, sanction, days, level }) => post({ category, description, sanction, days, level }),
  );
}

export function useAdminAffiliatePayouts(params: AdminQueueParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.payouts(params),
    queryFn: () => authFetch<AffiliateAdminPayoutPage>(`/admin/affiliate-payouts${queueQuery(params)}`),
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

// Downloads the CSV behind a queue, honouring the filters currently applied.
export async function downloadAffiliateCsv(kind: "payouts" | "flags" | "appeals", params: AdminQueueParams = {}): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const res = await fetch(`${API_BASE}/admin/affiliate-${kind}/export${queueQuery({ ...params, page: undefined, per_page: undefined })}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new ApiError(`HTTP_${res.status}`, `Failed to download ${kind}: ${res.status}`, res.status);
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = `affiliate-${kind}.csv`;
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

export function useAdminAffiliateFlags(params: AdminQueueParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.flags(params),
    queryFn: () => authFetch<AdminQueuePage<AffiliateFlagRow>>(`/admin/affiliate-flags${queueQuery(params)}`),
  });
}

export function useReviewAffiliateFlag() {
  return useAffiliateAction<{ id: string; status: string; note?: string }, AffiliateFlagRow>(
    ({ id }) => `/admin/affiliate-flags/${encodeURIComponent(id)}/review`,
    ({ status, note }) => post({ status, note }),
  );
}

export function useAdminAffiliateAppeals(params: AdminQueueParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.appeals(params),
    queryFn: () => authFetch<AdminQueuePage<AffiliateAppealRow>>(`/admin/affiliate-appeals${queueQuery(params)}`),
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
    enabled: from !== "" && to !== "",
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
