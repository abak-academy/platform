"use client";

import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type {
  PartnerAccess,
  PartnerCredentials,
  PartnerDashboard,
  PartnerMe,
  PartnerOrder,
  PartnerOrderPage,
  PartnerOrderPreview,
  PartnerProduct,
  PartnerSchool,
  PartnerStudentPage,
  PartnerStudentRegistration,
  PartnerVoucher,
} from "@/lib/types";

export const partnerKeys = {
  all: ["partner"] as const,
  me: () => [...partnerKeys.all, "me"] as const,
  dashboard: () => [...partnerKeys.all, "dashboard"] as const,
  students: (p: PartnerStudentParams) => [...partnerKeys.all, "students", p] as const,
  products: () => [...partnerKeys.all, "products"] as const,
  vouchers: () => [...partnerKeys.all, "vouchers"] as const,
  orders: () => [...partnerKeys.all, "orders"] as const,
  order: (id: string) => [...partnerKeys.all, "order", id] as const,
};

export function usePartnerMe(enabled = true) {
  return useQuery({ queryKey: partnerKeys.me(), enabled, queryFn: () => authFetch<PartnerMe>("/partners/me") });
}

export function usePartnerDashboard() {
  return useQuery({ queryKey: partnerKeys.dashboard(), queryFn: () => authFetch<PartnerDashboard>("/partners/me/dashboard") });
}

export interface PartnerStudentParams {
  q?: string;
  registeredByMe?: boolean;
  schoolId?: string;
  status?: string;
}

function studentQuery(p: PartnerStudentParams, cursor: string): string {
  const qs = new URLSearchParams();
  if (p.q) qs.set("q", p.q);
  if (p.registeredByMe) qs.set("registered_by_me", "true");
  if (p.schoolId) qs.set("school_id", p.schoolId);
  if (p.status) qs.set("status", p.status);
  if (cursor) qs.set("cursor", cursor);
  qs.set("limit", "50");
  return `?${qs.toString()}`;
}

export function usePartnerStudents(params: PartnerStudentParams) {
  return useInfiniteQuery({
    queryKey: partnerKeys.students(params),
    initialPageParam: "",
    queryFn: ({ pageParam }) => authFetch<PartnerStudentPage>(`/partners/me/students${studentQuery(params, pageParam)}`),
    getNextPageParam: (last) => last.next_cursor || undefined,
    placeholderData: keepPreviousData,
  });
}

export interface RegisterPartnerStudentInput {
  name: string;
  jenjang: string;
  school_id?: string;
  email?: string;
  grade?: number;
}

export function useRegisterPartnerStudent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: RegisterPartnerStudentInput) =>
      authFetch<PartnerStudentRegistration>("/partners/me/students", { method: "POST", body: JSON.stringify(input) }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: [...partnerKeys.all, "students"] });
      qc.invalidateQueries({ queryKey: partnerKeys.dashboard() });
    },
  });
}

export function useReissuePartnerCredentials() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (studentId: string) =>
      authFetch<PartnerCredentials>(`/partners/me/students/${studentId}/credentials`, { method: "POST", body: "{}" }),
    onSuccess: () => qc.invalidateQueries({ queryKey: [...partnerKeys.all, "students"] }),
  });
}

export function usePartnerProducts() {
  return useQuery({ queryKey: partnerKeys.products(), queryFn: () => authFetch<{ data: PartnerProduct[] }>("/partners/me/products") });
}

export function usePartnerVouchers() {
  return useQuery({ queryKey: partnerKeys.vouchers(), queryFn: () => authFetch<{ data: PartnerVoucher[] }>("/partners/me/vouchers") });
}

export interface PartnerOrderInput {
  product_id: string;
  student_ids: string[];
  promo_code?: string;
}

export function usePreviewPartnerOrder() {
  return useMutation({
    mutationFn: (input: PartnerOrderInput) =>
      authFetch<PartnerOrderPreview>("/partners/me/orders/preview", { method: "POST", body: JSON.stringify(input) }),
  });
}

export function useCreatePartnerOrder() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: PartnerOrderInput) =>
      authFetch<PartnerOrder>("/partners/me/orders", { method: "POST", body: JSON.stringify(input) }),
    onSuccess: () => qc.invalidateQueries({ queryKey: partnerKeys.orders() }),
  });
}

export function usePartnerOrders(status = "") {
  return useInfiniteQuery({
    queryKey: [...partnerKeys.orders(), status],
    initialPageParam: "",
    queryFn: ({ pageParam }) => {
      const qs = new URLSearchParams({ limit: "20" });
      if (status) qs.set("status", status);
      if (pageParam) qs.set("cursor", pageParam);
      return authFetch<PartnerOrderPage>(`/partners/me/orders?${qs.toString()}`);
    },
    getNextPageParam: (last) => last.next_cursor || undefined,
    placeholderData: keepPreviousData,
  });
}

export function usePartnerOrder(id: string) {
  return useQuery({
    queryKey: partnerKeys.order(id),
    queryFn: () => authFetch<PartnerOrder>(`/partners/me/orders/${encodeURIComponent(id)}`),
  });
}

export function usePartnerSchools() {
  return useQuery({
    queryKey: [...partnerKeys.all, "schools"],
    queryFn: () => authFetch<{ data: PartnerSchool[] }>("/partners/me/schools"),
  });
}

export async function fetchAllPartnerPages<T>(path: string, params: Record<string, string> = {}): Promise<T[]> {
  const rows: T[] = [];
  let cursor = "";
  do {
    const qs = new URLSearchParams({ ...params, limit: "100" });
    if (cursor) qs.set("cursor", cursor);
    const page = await authFetch<{ data: T[]; next_cursor?: string }>(`${path}?${qs.toString()}`);
    rows.push(...page.data);
    cursor = page.next_cursor ?? "";
  } while (cursor);
  return rows;
}

export function usePartnerProductAccess(productId: string | null) {
  return useQuery({
    queryKey: [...partnerKeys.all, "access", productId],
    enabled: Boolean(productId),
    queryFn: () => authFetch<PartnerAccess>(`/admin/products/${productId}/partner-access`),
  });
}

export function useSetPartnerProductAccess() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ productId, access }: { productId: string; access: PartnerAccess }) =>
      authFetch<PartnerAccess>(`/admin/products/${productId}/partner-access`, { method: "PUT", body: JSON.stringify(access) }),
    onSuccess: (_data, vars) => qc.invalidateQueries({ queryKey: [...partnerKeys.all, "access", vars.productId] }),
  });
}
