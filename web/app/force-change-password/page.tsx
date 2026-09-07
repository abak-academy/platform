"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { KeyRound } from "lucide-react";

import { useAuthStore } from "@/stores/auth";
import { redirectForRole } from "@/lib/auth-redirect";
import { ChangePasswordForm } from "@/components/profile/ChangePasswordForm";
import { useTranslation } from "@/lib/i18n";

// Landing spot for a session flagged must_change_password (admin-issued
// temporary credential). The API already blocks every other route for this
// token, so the page only needs to host the change form. Deliberately outside
// the (auth) layout, which bounces authenticated users back to the app.
export default function ForceChangePasswordPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.token);
  const user = useAuthStore((s) => s.user);

  useEffect(() => {
    if (!token) router.replace("/login");
  }, [token, router]);

  if (!token) {
    return (
      <div className="flex min-h-screen items-center justify-center text-ink-500">
        {t("force_change_password_loading")}
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-surface px-6 py-12">
      <div className="w-full max-w-[420px] rounded-[18px] border border-line bg-white p-8 shadow-[0_10px_30px_rgba(20,16,62,0.08)]">
        <div className="mb-6 flex items-start gap-3">
          <span className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary">
            <KeyRound className="size-4.5" />
          </span>
          <div>
            <h1 className="font-serif text-[22px] font-bold leading-tight text-ink-900">
              {t("force_change_password_title")}
            </h1>
            <p className="mt-1.5 text-[13px] leading-[1.55] text-ink-600">
              {t("force_change_password_lede")}
            </p>
          </div>
        </div>

        <ChangePasswordForm
          onDone={() => router.replace(redirectForRole(user?.role))}
        />
      </div>
    </div>
  );
}
