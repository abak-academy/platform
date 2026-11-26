"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { User, Lock, Eye, EyeOff, Loader2, AlertCircle } from "lucide-react";

import { useLogin } from "@/lib/hooks/auth";
import { useAffiliateProgramStatus } from "@/lib/hooks/affiliate";
import { redirectForRole } from "@/lib/auth-redirect";
import { safeNext } from "@/lib/safe-next";
import { GoogleSignInButton, googleSignInAvailable } from "@/components/auth/GoogleSignInButton";
import { ApiError } from "@/lib/api";
import { loginErrorMessage } from "@/lib/auth-error";
import { adoptResendBudget } from "@/lib/otp-budget";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

function LoginForm() {
  const router = useRouter();
  const next = useSearchParams().get("next");
  const nextQuery = next ? `?next=${encodeURIComponent(next)}` : "";
  const { t } = useTranslation();
  const login = useLogin();
  const affiliateStatus = useAffiliateProgramStatus();
  const [identifier, setIdentifier] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [showPw, setShowPw] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const data = await login.mutateAsync({ identifier, password });
      router.push(safeNext(next, data.user?.role) ?? redirectForRole(data.user?.role));
    } catch (err) {
      if (err instanceof ApiError && err.code === "verification_pending") {
        const pendingToken = err.body?.pending_token;
        if (typeof window !== "undefined" && typeof pendingToken === "string") {
          sessionStorage.setItem("abak-pending-token", pendingToken);
          // Delivery/budget flags for the OTP page's banners and countdown.
          sessionStorage.setItem("abak-email-failed", err.body?.email_failed ? "1" : "0");
          sessionStorage.setItem("abak-otp-active", err.body?.otp_active ? "1" : "0");
          adoptResendBudget(Number(err.body?.resend_after) || 60);
        }
        router.push(`/otp?id=${encodeURIComponent(identifier)}${next ? `&next=${encodeURIComponent(next)}` : ""}`);
        return;
      }
      setError(loginErrorMessage(err, t));
    }
  };

  const loading = login.isPending;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h1 className="font-[family-name:var(--font-display)] text-[36px] font-extrabold leading-[1.05] tracking-[-0.04em] text-ink-900">{t("login_heading")}</h1>
        <p className="text-ink-600">{t("login_intro")}</p>
      </div>

      <form onSubmit={onSubmit} noValidate>
        <div className="mb-4">
          <Label htmlFor="identifier" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("login_identifier_label")}
          </Label>
          <div className="relative">
            <Input
              id="identifier"
              value={identifier}
              onChange={(e) => setIdentifier(e.target.value)}
              placeholder={t("login_identifier_placeholder")}
              autoComplete="username"
              required
              className="h-11 pl-10"
            />
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              <User size={16} />
            </span>
          </div>
        </div>

        <div className="mb-2.5">
          <Label htmlFor="password" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("login_password_label")}
          </Label>
          <div className="relative">
            <Input
              id="password"
              type={showPw ? "text" : "password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              autoComplete="current-password"
              required
              className="h-11 pl-10 pr-11"
            />
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              <Lock size={16} />
            </span>
            <button
              type="button"
              onClick={() => setShowPw((p) => !p)}
              aria-label={showPw ? t("auth_hide_password") : t("auth_show_password")}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
            >
              {showPw ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
        </div>

        {error && (
          <div
            role="alert"
            className="mb-4 mt-3 flex items-center gap-2 rounded-[10px] border border-danger/20 bg-danger-bg px-3 py-2.5 text-[12.5px] text-danger"
          >
            <AlertCircle size={14} className="shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <Button
          type="submit"
          disabled={loading}
          className="mt-5 h-12 w-full text-[15px]"
        >
          {loading ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("login_submitting")}
            </span>
          ) : (
            t("login_submit")
          )}
        </Button>
      </form>

      <p className="mt-6 text-center text-[13px] text-ink-600">
        {t("login_no_account")}{" "}
        <button
          type="button"
          onClick={() => router.push(`/register${nextQuery}`)}
          className="font-bold text-success transition-colors hover:text-success/80"
        >
          {t("login_sign_up_link")}
        </button>
      </p>
      {(() => {
        const affiliateOpen = affiliateStatus.data?.enabled === true;
        const partnerOpen = affiliateStatus.data?.partner_enabled === true;
        if (!affiliateOpen && !partnerOpen) return null;
        const promptKey = affiliateOpen ? "login_affiliate_prompt" : "login_partner_prompt";
        const linkKey = affiliateOpen ? "login_affiliate_link" : "login_partner_link";
        return (
          <p className="mt-2 text-center text-[13px] text-ink-600">
            {t(promptKey)}{" "}
            <button
              type="button"
              onClick={() => router.push("/affiliate-register")}
              className="font-bold text-success transition-colors hover:text-success/80"
            >
              {t(linkKey)}
            </button>
          </p>
        );
      })()}

      {googleSignInAvailable && (
        <>
          <div className="flex items-center gap-3 text-xs text-ink-500">
            <div className="h-px flex-1 bg-line" />
            <span>{t("auth_or_divider")}</span>
            <div className="h-px flex-1 bg-line" />
          </div>
          <GoogleSignInButton text="signin_with" next={next} />
        </>
      )}
    </div>
  );
}

export default function LoginPage() {
  return (
    <React.Suspense fallback={<div />}>
      <LoginForm />
    </React.Suspense>
  );
}
