"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import { studentsKeys } from "@/lib/hooks/students";
import type { User } from "@/lib/types";

export function useUpdateOwnPhoto() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (photo_url: string) =>
      authFetch<User>(`/auth/photo`, {
        method: "PATCH",
        body: JSON.stringify({ photo_url }),
      }),
    onSuccess: (data) => {
      const { token, refreshToken } = useAuthStore.getState();
      if (token && data) {
        useAuthStore.getState().setSession(token, refreshToken ?? "", data);
      }
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

export function useUpdateOwnAvatar() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (avatar_index: number | null) =>
      authFetch<User>(`/auth/avatar`, {
        method: "PATCH",
        body: JSON.stringify({ avatar_index }),
      }),
    onSuccess: (data) => {
      const { token, refreshToken } = useAuthStore.getState();
      if (token && data) {
        useAuthStore.getState().setSession(token, refreshToken ?? "", data);
      }
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
      qc.invalidateQueries({ queryKey: studentsKeys.profile() });
    },
  });
}
