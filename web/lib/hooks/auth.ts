"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, apiFetch, authFetch } from "@/lib/api";
import { useAuthStore } from "@/stores/auth";
import type { LoginResponse, User } from "@/lib/types";

export interface LoginInput {
  identifier: string;
  password: string;
}

const LOGIN_RETRY_DELAYS_MS = [1000, 2000];

// Full jitter (https://aws.amazon.com/blogs/architecture/timeouts-retries-and-backoff-with-jitter/)
// de-synchronizes a retry herd; it does not raise server throughput.
function fullJitterDelayMs(baseMs: number) {
  return Math.random() * baseMs;
}

export function useLogin() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({
    mutationFn: async (input: LoginInput) => {
      for (let attempt = 0; ; attempt++) {
        try {
          return await apiFetch<LoginResponse>(`/auth/login`, {
            method: "POST",
            body: JSON.stringify(input),
          });
        } catch (err) {
          const canRetry =
            err instanceof ApiError &&
            err.status === 429 &&
            err.code === "rate_limited" &&
            attempt < LOGIN_RETRY_DELAYS_MS.length;
          if (!canRetry) throw err;
          await new Promise((resolve) =>
            setTimeout(resolve, fullJitterDelayMs(LOGIN_RETRY_DELAYS_MS[attempt])),
          );
        }
      }
    },
    onSuccess: (data) => {
      if (data.access_token && data.user) {
        setSession(data.access_token, data.refresh_token ?? "", data.user);
      }
    },
  });
}

export interface RegisterInput {
  name: string;
  email: string;
  password: string;
}

export interface RegisterResponse {
  otp_required: boolean;
  pending_token: string;
}

export function useRegister() {
  return useMutation({
    mutationFn: (input: RegisterInput) =>
      apiFetch<RegisterResponse>(`/auth/register`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
  });
}

export interface VerifyOtpInput {
  identifier: string;
  code: string;
  pending_token?: string;
}

export function useVerifyOtp() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({
    mutationFn: (input: VerifyOtpInput) =>
      apiFetch<LoginResponse>(`/auth/otp/verify`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: (data) => {
      if (data.access_token && data.user) {
        setSession(data.access_token, data.refresh_token ?? "", data.user);
      }
    },
  });
}

export function useLogout() {
  const clear = useAuthStore((s) => s.clear);
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => authFetch<void>(`/auth/logout`, { method: "POST" }),
    onSettled: () => {
      clear();
      // Drop all cached queries so the next user in this tab can't read the
      // previous session's data (e.g. the durable profile gate reading a stale
      // Google profile for a password user within staleTime).
      queryClient.clear();
    },
  });
}

export interface GoogleLoginInput {
  id_token: string;
}

export function useGoogleLogin() {
  const setSession = useAuthStore((s) => s.setSession);
  return useMutation({
    mutationFn: (input: GoogleLoginInput) =>
      apiFetch<LoginResponse>(`/auth/google`, {
        method: "POST",
        body: JSON.stringify(input),
      }),
    onSuccess: (data) => {
      if (data.access_token && data.user) {
        setSession(data.access_token, data.refresh_token ?? "", data.user);
      }
    },
  });
}

export function useMe(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: () => authFetch<User>(`/auth/me`),
    enabled: options?.enabled ?? true,
    staleTime: 5 * 60 * 1000,
  });
}