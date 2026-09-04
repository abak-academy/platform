"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Eye, EyeOff, Loader2, AlertCircle } from "lucide-react";

import { useLogin } from "@/lib/hooks/auth";
import { redirectForRole } from "@/lib/auth-redirect";
import { loginErrorMessage } from "@/lib/auth-error";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

// The card is the brand gradient under a scrim. The scrim is load-bearing, not
// decoration: white on the gradient's teal stop measures 2.11:1, which fails
// even the 3:1 large-text floor. Indigo and purple clear 4.5:1 on their own.
const CARD_GRADIENT =
  "linear-gradient(148deg,#3D4DDB,#7C4DDB 55%,#17C9AA)";

// Fields sit directly on the gradient, so they carry their own contrast rather
// than inheriting the light-surface Input styling.
const FIELD_CLASS =
  "h-11 rounded-[9px] border-white/40 bg-white/[0.14] text-[14px] text-white " +
  "focus-visible:border-white/70 focus-visible:ring-[3px] focus-visible:ring-white/25";

const LABEL_CLASS = "mb-1.5 text-[12.5px] font-semibold text-white/90";

export default function AdminLoginPage() {
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
      if (data.user?.must_change_password) {
        // Admin-issued temporary credential: the API already confines the
        // session to the change-password flow, so land there directly.
        router.push("/force-change-password");
        return;
      }
      router.push(redirectForRole(data.user?.role));
    } catch (err) {
      setError(loginErrorMessage(err, t));
    }
  };

  const loading = login.isPending;

  return (
    <div
      className="relative overflow-hidden rounded-[18px] shadow-[0_18px_46px_rgba(20,16,62,0.34)]"
      style={{ backgroundImage: CARD_GRADIENT }}
    >
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 bg-[rgba(20,16,62,0.42)]"
      />

      <div className="relative px-7 pb-7 pt-7">
        <div className="text-[15px] font-bold leading-none text-white">
          abak{" "}
          <span className="text-[12px] tracking-[0.14em] text-[#F5C518]">
            ACADEMY
          </span>
        </div>
        <div className="mt-1 text-[10.5px] font-medium text-white/[0.78]">
          Admin Panel
        </div>

        <h1 className="mt-6 text-[22px] font-bold leading-[1.3] text-white [text-wrap:balance]">
          <span className="text-white/60">Manage </span>Classes
          <span className="text-white/60">, </span>Exams
          <span className="text-white/60"> &amp; </span>Store
        </h1>
        <p className="mt-2 text-[12px] leading-[1.5] text-white/[0.86]">
          {t("admin_login_sub")}
        </p>

        <div className="my-6 h-px bg-white/[0.18]" />

        <form onSubmit={onSubmit} noValidate>
          <div className="mb-4">
            <Label htmlFor="identifier" className={LABEL_CLASS}>
              {t("login_identifier_label")}
            </Label>
            <Input
              id="identifier"
              value={identifier}
              onChange={(e) => setIdentifier(e.target.value)}
              autoComplete="username"
              required
              className={FIELD_CLASS}
            />
          </div>

          <div className="mb-1">
            <Label htmlFor="password" className={LABEL_CLASS}>
              {t("login_password_label")}
            </Label>
            <div className="relative">
              <Input
                id="password"
                type={showPw ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                autoComplete="current-password"
                required
                className={`${FIELD_CLASS} pr-11`}
              />
              <button
                type="button"
                onClick={() => setShowPw((p) => !p)}
                aria-label={showPw ? t("auth_hide_password") : t("auth_show_password")}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-white/70 transition-colors hover:text-white"
              >
                {showPw ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
          </div>

          {error && (
            <div
              role="alert"
              className="mt-4 flex items-center gap-2 rounded-[10px] bg-[#FFDAD6] px-3 py-2.5 text-[12.5px] text-[#410002]"
            >
              <AlertCircle size={14} className="shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <button
            type="submit"
            disabled={loading}
            className="mt-6 flex h-12 w-full items-center justify-center rounded-[9px] bg-white text-[15px] font-bold text-[#2B3674] transition-opacity hover:opacity-95 focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-white/50 disabled:opacity-70"
          >
            {loading ? (
              <span className="flex items-center gap-2">
                <Loader2 size={16} className="animate-spin" />
                {t("login_submitting")}
              </span>
            ) : (
              t("login_submit")
            )}
          </button>
        </form>

        <div className="mt-6 h-px bg-white/[0.15]" />

        <p className="mt-4 text-center text-[11.5px] text-white/80">
          {t("admin_login_not_admin")}{" "}
          <Link href="/login" className="font-bold text-white underline">
            {t("admin_login_student_link")}
          </Link>
        </p>
      </div>
    </div>
  );
}
