"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { useResolvedRole } from "@/lib/hooks/use-capability";
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
  const { role: effectiveRole, hydrated, meIsError } = useResolvedRole();

  useEffect(() => {
    if (!hydrated) return;
    // These fire inside /admin, so the visitor is here for the admin app —
    // send them to its own sign-in rather than the student one.
    if (!token) {
      router.replace("/admin/login");
      return;
    }
    if (meIsError) {
      router.replace("/admin/login");
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
