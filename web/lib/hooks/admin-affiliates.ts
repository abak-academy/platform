"use client";

import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { API_BASE, ApiError, authFetch } from "@/lib/api";
import type {
  AffiliateCommissionRuleType,
  AffiliateAdminDetail,
  AffiliateAdminListPage,
  AffiliateAdminPayout,
  AffiliateAdminPayoutPage,
  AffiliateAdjustment,
  AffiliateCommission,
  AffiliateJobResult,
  AffiliateLiabilityReport,
  AffiliateOverrideProduct,
  AffiliateProductOverride,
  AffiliateReportOrder,
  AffiliateReportSummary,
  AffiliateSettingsInput,
  AffiliateSettingsView,
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
  overrides: (p?: AdminQueueParams) => [...adminAffiliateKeys.all, "overrides", p ?? null] as const,
  overrideProducts: (q: string) => [...adminAffiliateKeys.all, "override-products", q] as const,
  report: (p: AdminReportParams) => [...adminAffiliateKeys.all, "report", p] as const,
  reportOrders: (p: AdminQueueParams) => [...adminAffiliateKeys.all, "report-orders", p] as const,
  commissions: (id: string, p: AdminQueueParams) => [...adminAffiliateKeys.all, "commissions", id, p] as const,
  liability: () => [...adminAffiliateKeys.all, "liability"] as const,
};

export interface AdminAffiliateListParams {
  status?: string;
  level?: string;
  q?: string;
  page?: number;
  per_page?: number;
}

// The admin queues share one query: search, status, date range and paging. An empty status means every status.
export interface AdminQueueParams {
  q?: string;
  status?: string;
  type?: string;
  from?: string;
  to?: string;
  page?: number;
  per_page?: number;
}

function queueQuery(p: AdminQueueParams): string {
  return qs({ q: p.q, status: p.status || "all", type: p.type, from: p.from, to: p.to, page: p.page, per_page: p.per_page });
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
        `/admin/affiliates${qs({ status: params.status, level: params.level, q: params.q, page: params.page, per_page: params.per_page })}`,
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
  return useAffiliateAction<{ id: string; status: string; reason: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/status`,
    ({ status, reason }) => ({ method: "PATCH", body: JSON.stringify({ status, reason }) }),
  );
}

export function useChangeAffiliateLevel() {
  return useAffiliateAction<{ id: string; level: string; reason: string }>(
    ({ id }) => `/admin/affiliates/${encodeURIComponent(id)}/level`,
    ({ level, reason }) => post({ level, reason }),
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
export async function downloadAffiliateCsv(
  kind: "payouts" | "commissions" | "report-orders",
  params: AdminQueueParams = {},
  affiliatorId?: string,
): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const path =
    kind === "commissions"
      ? `/admin/affiliates/${encodeURIComponent(affiliatorId ?? "")}/commissions/export`
      : kind === "report-orders"
        ? "/admin/affiliate-reports/orders/export"
        : `/admin/affiliate-${kind}/export`;
  const res = await fetch(`${API_BASE}${path}${queueQuery({ ...params, page: undefined, per_page: undefined })}`, {
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

function invalidateOverrides(qc: QueryClient) {
  qc.invalidateQueries({ queryKey: [...adminAffiliateKeys.all, "overrides"] });
  qc.invalidateQueries({ queryKey: [...adminAffiliateKeys.all, "override-products"] });
}

// The overrides set so far, searchable by product name and paged.
export function useAdminAffiliateOverrides(params: { q?: string; page?: number; per_page?: number } = {}) {
  return useQuery({
    queryKey: adminAffiliateKeys.overrides(params),
    queryFn: () =>
      authFetch<AdminQueuePage<AffiliateProductOverride>>(
        `/admin/affiliate-product-overrides${qs({ q: params.q, page: params.page, per_page: params.per_page })}`,
      ),
  });
}

// Products matching a name, for the picker that replaces typing a product id.
export function useAffiliateOverrideProducts(q: string) {
  return useQuery({
    queryKey: adminAffiliateKeys.overrideProducts(q),
    queryFn: async () => {
      const res = await authFetch<{ data: AffiliateOverrideProduct[] }>(
        `/admin/affiliate-product-overrides/products${qs({ q, limit: 8 })}`,
      );
      return res.data ?? [];
    },
  });
}

export function useUpsertAffiliateOverride() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, type, value }: { productId: string; type: AffiliateCommissionRuleType; value: number }) =>
      authFetch<AffiliateProductOverride>(`/admin/affiliate-product-overrides/${encodeURIComponent(productId)}`, {
        method: "PUT",
        body: JSON.stringify({ type, value }),
      }),
    onSuccess: () => {
      invalidateOverrides(qc);
    },
  });
}

export function useDeleteAffiliateOverride() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (productId: string) =>
      authFetch<void>(`/admin/affiliate-product-overrides/${encodeURIComponent(productId)}`, { method: "DELETE" }),
    onSuccess: () => {
      invalidateOverrides(qc);
    },
  });
}

export interface AdminReportParams {
  from: string;
  to: string;
  q?: string;
  level?: string;
  page?: number;
  per_page?: number;
}

export function useAffiliateReportSummary(params: AdminReportParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.report(params),
    enabled: params.from !== "" && params.to !== "",
    queryFn: () => authFetch<AffiliateReportSummary>(`/admin/affiliate-reports/summary${qs({ ...params })}`),
  });
}

// Every affiliator's commissions with the order behind each, for the report's detail tab.
export function useAdminAffiliateReportOrders(params: AdminQueueParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.reportOrders(params),
    queryFn: () => authFetch<AdminQueuePage<AffiliateReportOrder>>(`/admin/affiliate-reports/orders${queueQuery(params)}`),
  });
}

// The orders behind one affiliator's commissions; status "" means every status.
export function useAdminAffiliateCommissions(id: string, params: AdminQueueParams) {
  return useQuery({
    queryKey: adminAffiliateKeys.commissions(id, params),
    enabled: id !== "",
    queryFn: () =>
      authFetch<AdminQueuePage<AffiliateCommission>>(`/admin/affiliates/${encodeURIComponent(id)}/commissions${queueQuery(params)}`),
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
