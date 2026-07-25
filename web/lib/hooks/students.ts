"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import type { Dashboard, School, User } from "@/lib/types";

export const studentsKeys = {
  all: ["students"] as const,
  dashboard: () => [...studentsKeys.all, "dashboard"] as const,
  profile: () => [...studentsKeys.all, "profile"] as const,
};

export function useDashboard() {
  return useQuery({
    queryKey: studentsKeys.dashboard(),
    queryFn: () => authFetch<Dashboard>(`/students/dashboard`),
  });
}

export function useProfile() {
  return useQuery({
    queryKey: studentsKeys.profile(),
    queryFn: () => authFetch<User>(`/students/profile`),
  });
}

export interface UpdateProfileInput {
  name?: string;
  email?: string;
  username?: string;
  phone?: string;
  nis?: string;
  grade?: string | number;
  school_id?: string;
  unlisted_school_name?: string;
  target_exam?: string;
  address?: string;
  dob?: string;
  gender?: string;
  jenjang?: string;
  provinsi_id?: string;
  kota_id?: string;
  kecamatan_id?: string;
  kode_pos?: string;
}

export function useSchools() {
  return useQuery({
    queryKey: [...studentsKeys.all, "schools"],
    queryFn: () => authFetch<School[]>(`/schools`),
  });
}

export interface PresignUploadInput {
  filename: string;
  content_type: string;
  kind?: string;
}

export interface PresignUploadResponse {
  url: string;
  method: "PUT";
  key: string;
  fields?: Record<string, string>;
}

export function usePresignUpload() {
  return useMutation({
    mutationFn: ({ filename, content_type, kind }: PresignUploadInput) =>
      authFetch<PresignUploadResponse>(
        `/uploads/presign?filename=${encodeURIComponent(filename)}&content_type=${encodeURIComponent(content_type)}${kind ? `&kind=${encodeURIComponent(kind)}` : ""}`,
        { method: "POST" }
      ),
  });
}

export function useUpdatePhoto() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (photo_url: string) =>
      authFetch<User>(`/students/photo`, {
        method: "PATCH",
        body: JSON.stringify({ photo_url }),
      }),
    onSuccess: (data) => {
      const { token, refreshToken } = useAuthStore.getState();
      if (token && data) {
        useAuthStore.getState().setSession(token, refreshToken ?? "", data);
      }
      qc.invalidateQueries({ queryKey: studentsKeys.profile() });
    },
  });
}

export function useUpdateProfile() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (input: UpdateProfileInput) =>
      authFetch<User>(`/students/profile`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: studentsKeys.profile() });
    },
  });
}

export interface ChangePasswordInput {
  current_password: string;
  new_password: string;
}

export function useChangePassword() {
  return useMutation({
    mutationFn: (input: ChangePasswordInput) =>
      authFetch<void>(`/auth/password/change`, {
        method: "PATCH",
        body: JSON.stringify(input),
      }),
  });
}