"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { BrandPanel } from "@/components/auth/BrandPanel";

export default function AuthLayout({ children }: { children: React.ReactNode }) {
  const token = useAuthStore((s) => s.token);
  const router = useRouter();
  const pathname = usePathname();
  // A reset link must work even in a browser still signed in (e.g. an admin-triggered reset).
  const redirectSignedIn = !!token && pathname !== "/reset-password";

  useEffect(() => {
    if (redirectSignedIn) router.replace("/");
  }, [redirectSignedIn, router]);

  if (redirectSignedIn) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return (
    <div className="flex min-h-screen">
      <BrandPanel mode="login" />
      <div className="flex flex-1 items-center justify-center bg-surface px-6 py-12">
        {children}
      </div>
    </div>
  );
}