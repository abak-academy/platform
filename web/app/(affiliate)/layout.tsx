"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { useResolvedRole } from "@/lib/hooks/use-capability";
import { redirectForRole } from "@/lib/auth-redirect";
import { AppShell } from "@/components/shell/AppShell";
import "../(admin)/admin-theme.css";

export default function AffiliateLayout({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token);
  const router = useRouter();
  const { role, hydrated, meIsError } = useResolvedRole();

  useEffect(() => {
    if (!hydrated) return;
    if (!token || meIsError) {
      router.replace("/login");
      return;
    }
    if (role && role !== "affiliator") {
      router.replace(redirectForRole(role));
    }
  }, [hydrated, token, meIsError, role, router]);

  if (!hydrated || !token || role !== "affiliator") {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return <AppShell role="affiliator">{children}</AppShell>;
}
