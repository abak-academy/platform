"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { User, Lock, Eye, EyeOff, Loader2, AlertCircle } from "lucide-react";

import { useLogin } from "@/lib/hooks/auth";
import { redirectForRole } from "@/lib/auth-redirect";
import { GoogleSignInButton, googleSignInAvailable } from "@/components/auth/GoogleSignInButton";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

export default function LoginPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const login = useLogin();
  const [identifier, setIdentifier] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [showPw, setShowPw] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      const data = await login.mutateAsync({ identifier, password });
      router.push(redirectForRole(data.user?.role));
    } catch (err) {
      if (err instanceof ApiError && err.code === "verification_pending") {
        const pendingToken = err.body?.pending_token;
        if (typeof window !== "undefined" && typeof pendingToken === "string") {
          sessionStorage.setItem("abak-pending-token", pendingToken);
        }
        router.push(`/otp?id=${encodeURIComponent(identifier)}`);
        return;
      }
      const msg =
        err instanceof ApiError ? err.message : t("login_failed");
      setError(msg);
    }
  };

  const loading = login.isPending;

  return (
    <div className="w-full max-w-[372px]">
      <div className="mb-7">
        <h2 className="font-serif text-[27px] font-bold leading-tight tracking-[-0.01em] text-ink-900 [text-wrap:balance]">
          {t("login_title")}
        </h2>
        {/* Body copy sits at ink-600: ink-500/400 render below the 4.5:1 AA floor
            on this surface, so each pair is separated by size, not by contrast. */}
        <p className="mt-3 text-[14.5px] font-semibold leading-[1.4] text-ink-900">
          {t("login_lede")}
        </p>
        <p className="mt-1 text-[13.5px] leading-[1.55] text-ink-600">
          {t("login_subtitle")}
        </p>
        <p className="mt-3 text-[12.5px] leading-[1.55] text-ink-600">
          <span className="font-semibold">{t("login_lede_local")}</span>{" "}
          {t("login_subtitle_local")}
        </p>
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

      <p className="mt-6 text-center text-[13px] text-ink-500">
        {t("login_no_account")}{" "}
        <button
          type="button"
          onClick={() => router.push("/register")}
          className="font-bold text-success transition-colors hover:text-success/80"
        >
          {t("login_sign_up_link")}
        </button>
      </p>

      {googleSignInAvailable && (
        <>
          <div className="mt-6 mb-5 flex items-center gap-3">
            <div className="h-px flex-1 bg-line" />
            <span className="text-xs font-medium text-ink-400">{t("auth_or_divider")}</span>
            <div className="h-px flex-1 bg-line" />
          </div>
          <GoogleSignInButton text="signin_with" />
        </>
      )}
    </div>
  );
}