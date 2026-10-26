"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch, authFetch } from "@/lib/api";
import type {
  AffiliateAdjustmentPage,
  AffiliateBindingPage,
  AffiliateCommissionPage,
  AffiliateLedgerPage,
  AffiliatePayout,
  AffiliatePayoutPage,
  AffiliateProfile,
  AffiliateStats,
  AffiliateValidation,
  AffiliateViolation,
  AffiliateViolationPage,
} from "@/lib/types";

export interface AffiliateListParams {
  page?: number;
  per_page?: number;
  q?: string;
  type?: string;
  status?: string;
  product_type?: string;
  from?: string;
  to?: string;
  order?: "asc" | "desc";
}

const LIST_PARAM_ORDER = ["page", "per_page", "q", "type", "status", "product_type", "from", "to", "order"] as const;

function listQuery(params?: AffiliateListParams): string {
  const qs = new URLSearchParams();
  for (const key of LIST_PARAM_ORDER) {
    const value = params?.[key];
    if (value !== undefined && value !== "") qs.set(key, String(value));
  }
  const s = qs.toString();
  return s ? `?${s}` : "";
}

export const affiliateKeys = {
  all: ["affiliate"] as const,
  me: () => [...affiliateKeys.all, "me"] as const,
  bindings: (p?: AffiliateListParams) => [...affiliateKeys.all, "bindings", p ?? null] as const,
  commissions: (p?: AffiliateListParams) => [...affiliateKeys.all, "commissions", p ?? null] as const,
  ledger: (p?: AffiliateListParams) => [...affiliateKeys.all, "ledger", p ?? null] as const,
  payouts: (p?: AffiliateListParams) => [...affiliateKeys.all, "payouts", p ?? null] as const,
  adjustments: (p?: AffiliateListParams) => [...affiliateKeys.all, "adjustments", p ?? null] as const,
  violations: (p?: AffiliateListParams) => [...affiliateKeys.all, "violations", p ?? null] as const,
  stats: (range?: AffiliateStatsRange) => [...affiliateKeys.all, "stats", range ?? null] as const,
};

export function useAffiliateMe() {
  return useQuery({
    queryKey: affiliateKeys.me(),
    queryFn: () => authFetch<AffiliateProfile>("/affiliates/me"),
  });
}

export function useUpdateAffiliateBank() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { bank_name: string; bank_account_number: string; bank_account_holder: string }) =>
      authFetch<AffiliateProfile>("/affiliates/me/bank", {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: affiliateKeys.me() });
    },
  });
}

export function useAffiliateBindings(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.bindings(params),
    queryFn: () => authFetch<AffiliateBindingPage>(`/affiliates/me/bindings${listQuery(params)}`),
  });
}

export function useAffiliateCommissions(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.commissions(params),
    queryFn: () => authFetch<AffiliateCommissionPage>(`/affiliates/me/commissions${listQuery(params)}`),
  });
}

export function useAffiliateLedger(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.ledger(params),
    queryFn: () => authFetch<AffiliateLedgerPage>(`/affiliates/me/ledger${listQuery(params)}`),
  });
}

export function useAffiliatePayouts(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.payouts(params),
    queryFn: () => authFetch<AffiliatePayoutPage>(`/affiliates/me/payouts${listQuery(params)}`),
  });
}

export function useAffiliateAdjustments(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.adjustments(params),
    queryFn: () => authFetch<AffiliateAdjustmentPage>(`/affiliates/me/adjustments${listQuery(params)}`),
  });
}

export function useAffiliateViolations(params?: AffiliateListParams) {
  return useQuery({
    queryKey: affiliateKeys.violations(params),
    queryFn: () => authFetch<AffiliateViolationPage>(`/affiliates/me/violations${listQuery(params)}`),
  });
}

export interface AffiliateStatsRange {
  from?: string;
  to?: string;
}

export function useAffiliateStats(range?: AffiliateStatsRange) {
  return useQuery({
    queryKey: affiliateKeys.stats(range),
    queryFn: () => {
      const qs = new URLSearchParams();
      if (range?.from) qs.set("from", range.from);
      if (range?.to) qs.set("to", range.to);
      const s = qs.toString();
      return authFetch<AffiliateStats>(`/affiliates/me/stats${s ? `?${s}` : ""}`);
    },
    placeholderData: keepPreviousData,
  });
}

export function useRequestAffiliatePayout() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: { amount?: number }) =>
      authFetch<AffiliatePayout>("/affiliates/me/payouts", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: affiliateKeys.all });
    },
  });
}

export function useAppealViolation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, text }: { id: string; text: string }) =>
      authFetch<AffiliateViolation>(`/affiliates/me/violations/${encodeURIComponent(id)}/appeal`, {
        method: "POST",
        body: JSON.stringify({ text }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...affiliateKeys.all, "violations"] });
    },
  });
}

export function useValidateAffiliateCode() {
  return useMutation({
    mutationFn: (code: string) =>
      apiFetch<AffiliateValidation>(`/affiliates/public/validate?code=${encodeURIComponent(code)}`),
  });
}
