"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { API_BASE, ApiError, authFetch } from "@/lib/api";
import type { PeriodRange } from "@/components/admin/PeriodBar";
import type { EnqueueBulkResult, PresignedUpload } from "@/lib/hooks/admin-students-bulk";
import type {
  Affiliator,
  AffiliatorDetail,
  AffiliatorInput,
  AffiliatorPage,
  AffiliateReportName,
  PromotableAdmin,
  ReportPage,
} from "@/lib/types";

export { putFileToPresignedURL } from "@/lib/hooks/admin-students-bulk";

// Super-admin affiliator management: /admin/system/affiliators* and the reports.
export const adminAffiliatorsKeys = {
  all: ["admin", "affiliators"] as const,
  list: (p: ListParams) => [...adminAffiliatorsKeys.all, "list", p.status ?? "", p.q ?? "", p.cursor ?? "", p.limit ?? 20] as const,
  promotable: (p: ListParams) => [...adminAffiliatorsKeys.all, "promotable", p.q ?? "", p.cursor ?? "", p.limit ?? 20] as const,
  detail: (id: string) => [...adminAffiliatorsKeys.all, "detail", id] as const,
  report: (name: string, p: ReportParams) => [...adminAffiliatorsKeys.all, "report", name, p] as const,
};

export interface ListParams {
  status?: string;
  q?: string;
  cursor?: string;
  limit?: number;
}

function listQuery(p: ListParams): string {
  const params = new URLSearchParams();
  if (p.status) params.set("status", p.status);
  if (p.q) params.set("q", p.q);
  if (p.cursor) params.set("cursor", p.cursor);
  params.set("limit", String(p.limit ?? 20));
  return params.toString();
}

export function useAdminAffiliators(p: ListParams) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.list(p),
    queryFn: () => authFetch<AffiliatorPage<Affiliator>>(`/admin/system/affiliators?${listQuery(p)}`),
    placeholderData: keepPreviousData,
  });
}

export function usePromotableAdmins(p: Omit<ListParams, "status">) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.promotable(p),
    queryFn: () => authFetch<AffiliatorPage<PromotableAdmin>>(`/admin/system/affiliators/promotable?${listQuery(p)}`),
    placeholderData: keepPreviousData,
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

/** POST /admin/system/affiliators/bulk/presign?filename=&content_type= */
export function usePresignAffiliatorBulkUpload() {
  return useMutation({
    mutationFn: ({ filename, contentType }: { filename: string; contentType: string }) => {
      const qs = new URLSearchParams({ filename, content_type: contentType }).toString();
      return authFetch<PresignedUpload>(`/admin/system/affiliators/bulk/presign?${qs}`, { method: "POST" });
    },
  });
}

/** POST /admin/system/affiliators/bulk {file_key} -> {job_id} */
export function useEnqueueAffiliatorBulkImport() {
  return useMutation({
    mutationFn: ({ fileKey }: { fileKey: string }) =>
      authFetch<EnqueueBulkResult>("/admin/system/affiliators/bulk", {
        method: "POST",
        body: JSON.stringify({ file_key: fileKey }),
      }),
  });
}

export interface ReportParams extends PeriodRange {
  q?: string;
  inactive?: boolean;
  limit?: number;
  offset?: number;
}

function reportQuery(p: ReportParams, paged: boolean): string {
  const params = new URLSearchParams();
  if (p.from) params.set("from", p.from);
  if (p.to) params.set("to", p.to);
  if (p.q) params.set("q", p.q);
  if (p.inactive) params.set("inactive", "true");
  if (paged) {
    params.set("limit", String(p.limit ?? 20));
    params.set("offset", String(p.offset ?? 0));
  }
  return params.toString();
}

/** One paged report: GET /admin/system/affiliate-reports/:name. */
export function useAffiliateReport<T>(name: Exclude<AffiliateReportName, "trend">, p: ReportParams) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.report(name, p),
    queryFn: () => authFetch<ReportPage<T>>(`/admin/system/affiliate-reports/${name}?${reportQuery(p, true)}`),
    placeholderData: keepPreviousData,
  });
}

/** The monthly trend is not paged: GET /admin/system/affiliate-reports/trend. */
export function useAffiliateTrend<T>(p: PeriodRange) {
  return useQuery({
    queryKey: adminAffiliatorsKeys.report("trend", p),
    queryFn: async () => (await authFetch<{ data: T[] }>(`/admin/system/affiliate-reports/trend?${reportQuery(p, false)}`)).data ?? [],
  });
}

/** Downloads a whole report for the period as CSV, ignoring paging. */
export async function downloadAffiliateReportCsv(name: AffiliateReportName, p: ReportParams): Promise<void> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;
  const res = await fetch(`${API_BASE}/admin/system/affiliate-reports/${name}/export?${reportQuery(p, false)}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new ApiError(`HTTP_${res.status}`, `Failed to download report: ${res.status}`, res.status);
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = `laporan-affiliator-${name}-${p.from || "semua-waktu"}_${p.to || "semua-waktu"}.csv`;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
