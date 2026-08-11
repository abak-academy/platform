"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { authFetch } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
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
