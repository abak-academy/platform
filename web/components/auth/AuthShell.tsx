"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/stores/auth";
import { BrandPanel } from "@/components/auth/BrandPanel";

export function AuthShell({
  children,
  variant = "student",
}: {
  children: React.ReactNode;
  variant?: "student" | "admin";
}) {
  const token = useAuthStore((s) => s.token);
  const router = useRouter();

  useEffect(() => {
    if (token) router.replace("/");
  }, [token, router]);

  if (token) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        Memuat…
      </div>
    );
  }

  return (
    <div className="grid min-h-screen grid-cols-1 bg-surface min-[900px]:grid-cols-[minmax(380px,1fr)_minmax(0,1.1fr)]">
      <BrandPanel variant={variant} />
      <main id="main" className="flex flex-col bg-surface p-4 min-[900px]:p-6">
        <div className="m-auto w-[min(460px,100%)] py-6">{children}</div>
      </main>
    </div>
  );
}
