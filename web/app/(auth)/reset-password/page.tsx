"use client";

import * as React from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Lock, Eye, EyeOff, Loader2, AlertCircle, CheckCircle2 } from "lucide-react";

import { useResetPassword } from "@/lib/hooks/auth";
import { ApiError } from "@/lib/api";
import { OtpInput } from "@/components/auth/OtpInput";
import { PasswordStrength } from "@/components/auth/PasswordStrength";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

const MIN_PASSWORD_LEN = 8;

function ResetPasswordForm() {
  const { t } = useTranslation();
  const token = useSearchParams().get("token") ?? "";
  const reset = useResetPassword();
  const [code, setCode] = React.useState<string[]>(Array(6).fill(""));
  const [password, setPassword] = React.useState("");
  const [showPw, setShowPw] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [done, setDone] = React.useState(false);

  const otp = code.join("");

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password.length < MIN_PASSWORD_LEN) {
      setError(t("reset_password_too_short"));
      return;
    }
    try {
      await reset.mutateAsync({ token, otp, new_password: password });
      setDone(true);
    } catch (err) {
      if (err instanceof ApiError && err.code === "reset_invalid") setError(t("reset_invalid"));
      else if (err instanceof ApiError && err.code === "rate_limited") setError(t("forgot_rate_limited"));
      else if (err instanceof ApiError && err.code === "invalid_request") setError(t("reset_password_too_short"));
      else setError(t("reset_failed"));
    }
  };

  const requestNew = (
    <Link href="/forgot-password" className="font-bold text-success transition-colors hover:text-success/80">
      {t("reset_request_new")}
    </Link>
  );

  if (!token) {
    return (
      <div className="w-full max-w-[372px]">
        <h2 className="font-serif text-[27px] font-bold leading-tight text-ink-900">{t("reset_title")}</h2>
        <p role="alert" className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("reset_missing_token")}</p>
        <p className="mt-6 text-[13px]">{requestNew}</p>
      </div>
    );
  }

  if (done) {
    return (
      <div className="w-full max-w-[372px]">
        <CheckCircle2 size={32} className="mb-4 text-success" />
        <h2 className="font-serif text-[27px] font-bold leading-tight text-ink-900">{t("reset_done_title")}</h2>
        <p className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("reset_done_body")}</p>
        <Button asChild className="mt-6 h-12 w-full text-[15px]">
          <Link href="/login">{t("login_submit")}</Link>
        </Button>
      </div>
    );
  }

  return (
    <div className="w-full max-w-[372px]">
      <div className="mb-7">
        <h2 className="font-serif text-[27px] font-bold leading-tight text-ink-900">{t("reset_title")}</h2>
        <p className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("reset_subtitle")}</p>
      </div>

      <form onSubmit={onSubmit} noValidate>
        <p className="mb-1.5 text-[12.5px] font-semibold text-ink-600">{t("reset_code_label")}</p>
        <OtpInput value={code} onChange={setCode} hasError={!!error} disabled={reset.isPending} />

        <div className="mt-5">
          <Label htmlFor="new-password" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("reset_password_label")}
          </Label>
          <div className="relative">
            <Input
              id="new-password"
              type={showPw ? "text" : "password"}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="••••••••"
              autoComplete="new-password"
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
          <PasswordStrength password={password} />
        </div>

        {error && (
          <div
            role="alert"
            className="mt-3 flex items-center gap-2 rounded-[10px] border border-danger/20 bg-danger-bg px-3 py-2.5 text-[12.5px] text-danger"
          >
            <AlertCircle size={14} className="shrink-0" />
            <span>{error}</span>
          </div>
        )}

        <Button
          type="submit"
          disabled={reset.isPending || otp.length !== 6 || !password}
          className="mt-5 h-12 w-full text-[15px]"
        >
          {reset.isPending ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("reset_submitting")}
            </span>
          ) : (
            t("reset_submit")
          )}
        </Button>
      </form>
      <p className="mt-6 text-center text-[13px]">{requestNew}</p>
    </div>
  );
}

export default function ResetPasswordPage() {
  return (
    <React.Suspense fallback={<div className="w-full max-w-[372px]" />}>
      <ResetPasswordForm />
    </React.Suspense>
  );
}
