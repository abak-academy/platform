"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import type { AdminSchoolInput, AdminSchoolUpdateInput, School } from "@/lib/types";

export interface AdminSchoolsParams {
  q?: string;
  status?: string;
  cursor?: string;
  limit?: number;
}

export interface AdminSchoolsResponse {
  data: School[];
  next_cursor?: string;
  total: number;
  active: number;
  students: number;
}

export const adminSchoolsKeys = {
  all: ["admin", "schools"] as const,
  list: (params?: AdminSchoolsParams) =>
    [
      ...adminSchoolsKeys.all,
      "list",
      params?.q ?? "",
      params?.status ?? "all",
      params?.cursor ?? "initial",
      params?.limit ?? 20,
    ] as const,
};

export function useAdminSchools(params?: AdminSchoolsParams) {
  const { q, status, cursor, limit } = params ?? {};
  return useQuery({
    queryKey: adminSchoolsKeys.list(params),
    queryFn: async () => {
      const search = new URLSearchParams();
      if (q) search.set("q", q);
      if (status) search.set("status", status);
      if (cursor) search.set("cursor", cursor);
      if (limit) search.set("limit", String(limit));
      const query = search.toString();
      const path = query ? `/admin/schools?${query}` : "/admin/schools";
      return authFetch<AdminSchoolsResponse>(path);
    },
  });
}

export function useCreateSchool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: AdminSchoolInput) =>
      authFetch<School>("/admin/schools", {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminSchoolsKeys.all });
    },
  });
}

export function useUpdateSchool() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, ...data }: { id: string } & AdminSchoolUpdateInput) =>
      authFetch<School>(`/admin/schools/${encodeURIComponent(id)}`, {
        method: "PUT",
        body: JSON.stringify(data),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminSchoolsKeys.all });
    },
  });
}

export function useChangeSchoolStatus() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      authFetch<School>(`/admin/schools/${encodeURIComponent(id)}`, {
        method: "PATCH",
        body: JSON.stringify({ status }),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: adminSchoolsKeys.all });
    },
  });
}
