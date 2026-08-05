"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { useResolvedAdminRole } from "@/lib/hooks/use-capability";
import { AppShell } from "@/components/shell/AppShell";
import { ADMIN_ROLES } from "@/lib/nav-config";
import "./admin-theme.css";

export default function AdminLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const router = useRouter();
  const token = useAuthStore((s) => s.token);
  const { role: effectiveRole, hydrated, meIsError } = useResolvedAdminRole();

  useEffect(() => {
    if (!hydrated) return;
    if (!token) {
      router.replace("/login");
      return;
    }
    if (meIsError) {
      router.replace("/login");
      return;
    }
    if (effectiveRole && !ADMIN_ROLES.includes(effectiveRole)) {
      router.replace("/");
    }
  }, [hydrated, token, effectiveRole, meIsError, router]);

  if (!hydrated || !token || !effectiveRole || !ADMIN_ROLES.includes(effectiveRole)) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return <AppShell role={effectiveRole}>{children}</AppShell>;
}
