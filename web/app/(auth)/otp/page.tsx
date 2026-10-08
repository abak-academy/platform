"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { ArrowLeft, ShieldCheck, Loader2, AlertCircle, Info } from "lucide-react";

import { useVerifyOtp, useSendOtp } from "@/lib/hooks/auth";
import { redirectForRole } from "@/lib/auth-redirect";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { OtpInput } from "@/components/auth/OtpInput";
import { useTranslation, type I18nKey } from "@/lib/i18n";

function maskContact(contact: string): string {
  if (!contact) return "•••••";
  if (contact.includes("@")) {
    return contact.replace(/(.{2}).*(@.*)/, "$1•••$2");
  }
  return contact.replace(/(\+?\d{2,4})\d+(\d{3})/, "$1•••••$2");
}

// The resend endpoint's errors carry user-facing Indonesian copy except a
// failed delivery, whose raw message is technical — map both to friendly text.
// A stale pending token (otp_invalid) never reaches the user as-is: the page
// sends the email alongside the token, so the server resolves it either way.
function resendErrorMessage(err: unknown, t: (key: I18nKey) => string): string {
  if (!(err instanceof ApiError)) return t("otp_resend_failed");
  if (err.code === "email_delivery_failed") return t("otp_send_failed");
  return t("otp_resend_failed");
}

function OtpForm() {
  const router = useRouter();
  const { t } = useTranslation();
  const params = useSearchParams();
  const identifier = params.get("id") ?? "";

  const verify = useVerifyOtp();
  const sendOtp = useSendOtp();
  const [digits, setDigits] = React.useState<string[]>(Array(6).fill(""));
  const [error, setError] = React.useState<string | null>(null);
  const [banner, setBanner] = React.useState<string | null>(null);
  const [resendIn, setResendIn] = React.useState<number>(60);

  // Flags written by the login/register pages describe how this challenge was
  // armed: a failed delivery, a live code (no send), and the server's own
  // resend budget. Read once on mount — re-running it on every render (t's
  // identity churns) would clobber the countdown adopted from a 429.
  React.useEffect(() => {
    if (typeof window === "undefined") return;
    const failed = sessionStorage.getItem("abak-email-failed") === "1";
    const active = sessionStorage.getItem("abak-otp-active") === "1";
    // Absent = no server budget known (default 60s floor); "0" means free now.
    const raw = sessionStorage.getItem("abak-resend-after");
    const stored = raw === null ? 60 : Math.max(0, Math.ceil(Number(raw) || 0));
    if (failed) setBanner(t("otp_email_failed_banner"));
    else if (active) setBanner(t("otp_active_banner"));
    setResendIn(stored);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  React.useEffect(() => {
    if (resendIn <= 0) return;
    const timer = setInterval(() => setResendIn((s) => (s > 0 ? s - 1 : 0)), 1000);
    return () => clearInterval(timer);
  }, [resendIn]);

  const code = digits.join("");
  const allFilled = digits.every((d) => d !== "");

  const onVerify = async () => {
    if (code.length < 6) {
      setError(t("otp_required"));
      return;
    }
    setError(null);
    try {
      const pending_token =
        typeof window !== "undefined" ? sessionStorage.getItem("abak-pending-token") ?? "" : "";
      const data = await verify.mutateAsync({ identifier, code, pending_token });
      if (typeof window !== "undefined") {
        sessionStorage.removeItem("abak-pending-token");
        sessionStorage.removeItem("abak-email-failed");
        sessionStorage.removeItem("abak-otp-active");
        sessionStorage.removeItem("abak-resend-after");
      }
      router.push(redirectForRole(data.user?.role));
    } catch (err) {
      const msg =
        err instanceof ApiError ? err.message : t("otp_invalid");
      setError(msg);
    }
  };

  const onResend = async () => {
    if (resendIn > 0 || sendOtp.isPending) return;
    setError(null);
    const pending_token =
      typeof window !== "undefined" ? sessionStorage.getItem("abak-pending-token") ?? "" : "";
    try {
      const data = await sendOtp.mutateAsync({ pending_token, email: identifier });
      // A successful resend re-arms the pending token server-side; keep the
      // freshest one so the next verify works even after the old expired.
      if (data.pending_token && typeof window !== "undefined") {
        sessionStorage.setItem("abak-pending-token", data.pending_token);
      }
      setBanner(t("otp_resent"));
      const serverBudget = Number(data.resend_after);
      setResendIn(Number.isFinite(serverBudget) && serverBudget > 0 ? Math.ceil(serverBudget) : 60);
    } catch (err) {
      if (err instanceof ApiError && err.retryAfter) setResendIn(err.retryAfter);
      setError(resendErrorMessage(err, t));
    }
  };

  const loading = verify.isPending;

  return (
    <div className="w-full max-w-[372px]">
      <button
        type="button"
        onClick={() => router.push("/login")}
        className="mb-8 flex items-center gap-1.5 text-[13px] font-semibold text-ink-400 transition-colors hover:text-ink-600"
      >
        <ArrowLeft size={15} />
        {t("auth_back")}
      </button>

      <div className="mb-5 flex h-14 w-14 items-center justify-center rounded-[14px] bg-[linear-gradient(135deg,#EEF0FD,#DDE1FB)]">
        <ShieldCheck size={26} className="text-brand-600" />
      </div>

      <h2 className="font-serif text-[26px] font-bold leading-tight tracking-[-0.01em] text-ink-900">
        {t("otp_title")}
      </h2>
      <p className="mt-2.5 mb-7 text-[13.5px] leading-[1.6] text-ink-500">
        {t("otp_subtitle_prefix")}
        <strong className="text-ink-700">{maskContact(identifier)}</strong>
        {t("otp_subtitle_suffix")}
      </p>

      {banner && (
        <div
          role="status"
          className="mb-4 flex items-start gap-2 rounded-lg border border-warn/30 bg-warn-bg px-3 py-2.5 text-[12.5px] leading-[1.5] text-ink-700"
        >
          <Info size={14} className="mt-0.5 shrink-0 text-warn" />
          <span>{banner}</span>
        </div>
      )}

      <OtpInput
        value={digits}
        onChange={setDigits}
        hasError={!!error}
        disabled={loading}
      />

      {error && (
        <div
          role="alert"
          className="mt-4 flex items-center gap-2 text-[12.5px] text-danger"
        >
          <AlertCircle size={14} className="shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <Button
        type="button"
        onClick={onVerify}
        disabled={loading || !allFilled}
        className="mt-6 h-12 w-full text-[15px]"
      >
        {loading ? (
          <span className="flex items-center gap-2">
            <Loader2 size={16} className="animate-spin" />
            {t("otp_submitting")}
          </span>
        ) : (
          t("otp_submit")
        )}
      </Button>

      <div className="mt-5 flex flex-col items-center gap-1.5 text-center text-[12px] leading-[1.6] text-ink-400">
        <p>{t("otp_help")}</p>
        {resendIn > 0 ? (
          <p className="text-ink-500">
            {t("otp_resend_countdown").replace("{s}", String(resendIn))}
          </p>
        ) : (
          <button
            type="button"
            onClick={onResend}
            disabled={sendOtp.isPending}
            className="font-semibold text-brand-600 underline underline-offset-2 transition-colors hover:text-brand-700 disabled:opacity-60"
          >
            {sendOtp.isPending ? t("otp_submitting") : t("otp_resend")}
          </button>
        )}
      </div>
    </div>
  );
}

export default function OtpPage() {
  return (
    <React.Suspense fallback={<div className="w-full max-w-[372px]" />}>
      <OtpForm />
    </React.Suspense>
  );
}