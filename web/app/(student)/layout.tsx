"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { useProfile } from "@/lib/hooks/students";
import { isProfileComplete } from "@/lib/profile";
import { AppShell } from "@/components/shell/AppShell";
import { ADMIN_ROLES } from "@/lib/nav-config";
import type { UserRole } from "@/lib/nav-config";

const SNAP_SRC =
  process.env.NEXT_PUBLIC_MIDTRANS_SNAP_URL ??
  "https://app.sandbox.midtrans.com/snap/snap.js";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

export default function StudentLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);
  const router = useRouter();
  const [hydrated, setHydrated] = useState(false);

  const role = user?.role as UserRole | undefined;

  // Durable Google-only completeness gate — re-evaluated each session from DB-truth.
  const {
    data: profile,
    isLoading: profileLoading,
    isFetching: profileFetching,
    isError: profileError,
  } = useProfile();
  // Only a Google student's shell depends on completeness read from DB truth, so
  // only theirs must distrust a cached profile mid-refetch. Applying that to
  // everyone tore the shell down — and with it every page's local state — on any
  // background refetch: saving an address from the cart threw the buyer back to
  // a blank form.
  const profilePending =
    profileLoading || profileError || (profileFetching && profile?.auth_provider === "google");

  useEffect(() => {
    setHydrated(true);
  }, []);

  useEffect(() => {
    if (!hydrated) return;
    if (!token) {
      router.replace("/login");
      return;
    }
    if (role && ADMIN_ROLES.includes(role)) {
      router.replace("/admin");
    }
  }, [hydrated, token, role, router]);

  // Google-only gate: incomplete Google students go to /complete-profile.
  // Password students with NULL school_id are never redirected.
  useEffect(() => {
    if (!hydrated || !token || profilePending) return;
    if (
      profile &&
      profile.auth_provider === "google" &&
      !isProfileComplete(profile)
    ) {
      router.replace("/complete-profile");
    }
  }, [hydrated, token, profilePending, profile, router]);

  // Load Midtrans Snap JS with client key from backend (DB-sourced).
  useEffect(() => {
    if (!hydrated || !token) return;
    if (document.querySelector('script[src*="snap.js"]')) return;

    let cancelled = false;

    fetch(`${API_BASE}/config/payment-client-key`)
      .then((res) => (res.ok ? res.json() : Promise.reject(res)))
      .then((data: { client_key: string }) => {
        if (cancelled || !data.client_key) return;
        const script = document.createElement("script");
        script.src = SNAP_SRC;
        script.setAttribute("data-client-key", data.client_key);
        script.async = true;
        document.head.appendChild(script);
      })
      .catch(() => {
        // Fall back to build-time env var if API unavailable
        const fallbackKey =
          process.env.NEXT_PUBLIC_MIDTRANS_CLIENT_KEY ?? "";
        if (cancelled || !fallbackKey) return;
        const script = document.createElement("script");
        script.src = SNAP_SRC;
        script.setAttribute("data-client-key", fallbackKey);
        script.async = true;
        document.head.appendChild(script);
      });

    return () => {
      cancelled = true;
    };
  }, [hydrated, token]);

  // Show loading while profile resolves (gate needs it) or during no-token/admin checks.
  if (!hydrated || !token || (role && ADMIN_ROLES.includes(role)) || profilePending) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  // Don't render shell for gated Google students (gate redirects above).
  if (profile && profile.auth_provider === "google" && !isProfileComplete(profile)) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return <AppShell role="student">{children}</AppShell>;
}
