"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { User, Mail, Lock, Eye, EyeOff, CheckCircle2, Loader2, AlertCircle } from "lucide-react";
import { toast } from "sonner";

import { useRegister } from "@/lib/hooks/auth";
import { GoogleSignInButton } from "@/components/auth/GoogleSignInButton";
import { ApiError } from "@/lib/api";
import { adoptResendBudget } from "@/lib/otp-budget";
import { readAffRefCookie } from "@/lib/aff-ref";
import { storeManualReferral } from "@/lib/referral";
import { ReferralCodeField } from "@/components/auth/ReferralCodeField";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

type Errors = {
  name?: string;
  email?: string;
  password?: string;
  confirm?: string;
};

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export default function RegisterPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const register = useRegister();
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [confirm, setConfirm] = React.useState("");
  const [showPw, setShowPw] = React.useState(false);
  const [showCf, setShowCf] = React.useState(false);
  const [errors, setErrors] = React.useState<Errors>({});
  const [referral, setReferral] = React.useState("");

  React.useEffect(() => {
    setReferral(readAffRefCookie());
  }, []);

  const pwStrength =
    password.length === 0
      ? null
      : password.length < 6
        ? "weak"
        : password.length < 10
          ? "fair"
          : "strong";

  const strengthColor: Record<string, string> = {
    weak: "bg-danger",
    fair: "bg-warn",
    strong: "bg-success",
  };
  const strengthText: Record<string, string> = {
    weak: t("register_pw_weak"),
    fair: t("register_pw_fair"),
    strong: t("register_pw_strong"),
  };
  const strengthTextColor: Record<string, string> = {
    weak: "text-danger",
    fair: "text-warn",
    strong: "text-success",
  };
  const strengthWidth: Record<string, string> = {
    weak: "w-1/3",
    fair: "w-2/3",
    strong: "w-full",
  };

  const validate = (): Errors => {
    const e: Errors = {};
    if (!name.trim()) e.name = t("register_err_name_required");
    if (!EMAIL_RE.test(email)) e.email = t("register_err_email_invalid");
    if (password.length < 8) e.password = t("register_err_password_min");
    if (confirm !== password) e.confirm = t("register_err_confirm_mismatch");
    return e;
  };

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const errs = validate();
    setErrors(errs);
    if (Object.keys(errs).length > 0) return;

    try {
      const res = await register.mutateAsync({ name: name.trim(), email: email.trim(), password });
      // Registration returns an OTP challenge — hand the pending token + email to
      // the OTP page (it reads ?id= and sessionStorage["abak-pending-token"]).
      if (res.otp_required && res.pending_token) {
        if (typeof window !== "undefined") {
          sessionStorage.setItem("abak-pending-token", res.pending_token);
          // Delivery/budget flags for the OTP page's banners and countdown.
          sessionStorage.setItem("abak-email-failed", res.email_failed ? "1" : "0");
          sessionStorage.setItem("abak-otp-active", res.otp_active ? "1" : "0");
          adoptResendBudget(Number(res.resend_after) || 60);
          storeManualReferral(referral);
        }
        if (res.email_failed) {
          toast.warning(t("otp_email_failed_banner"));
        } else {
          toast.success(t("register_success"));
        }
        router.push(`/otp?id=${encodeURIComponent(email.trim())}`);
      } else {
        toast.success(t("register_success"));
        router.push("/login");
      }
    } catch (err) {
      const msg =
        err instanceof ApiError ? err.message : t("register_failed");
      toast.error(msg);
    }
  };

  const loading = register.isPending;
  const passwordsMatch = confirm.length > 0 && confirm === password;

  return (
    <div className="w-full max-w-[372px]">
      <div className="mb-7">
        <div className="mb-2 text-[11.5px] font-bold uppercase tracking-[0.06em] text-success">
          {t("register_eyebrow")}
        </div>
        <h2 className="font-serif text-[27px] font-bold leading-tight tracking-[-0.01em] text-ink-900">
          {t("register_title")}
        </h2>
        <p className="mt-2 text-[13.5px] leading-[1.55] text-ink-500">
          {t("register_subtitle")}
        </p>
      </div>

      <div className="mb-5">
        <GoogleSignInButton text="signup_with" />
      </div>

      <div className="mb-5 flex items-center gap-3">
        <div className="h-px flex-1 bg-line" />
        <span className="text-xs font-medium text-ink-400">{t("auth_or_divider")}</span>
        <div className="h-px flex-1 bg-line" />
      </div>

      <form onSubmit={onSubmit} noValidate>
        <div className="mb-3.5">
          <Label htmlFor="name" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("register_full_name_label")}
          </Label>
          <div className="relative">
            <Input
              id="name"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setErrors((p) => ({ ...p, name: undefined }));
              }}
              placeholder={t("register_name_placeholder")}
              autoComplete="name"
              className="h-11 pl-10"
            />
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              <User size={16} />
            </span>
          </div>
          {errors.name && (
            <div className="mt-1 text-[11.5px] text-danger">{errors.name}</div>
          )}
        </div>

        <div className="mb-3.5">
          <Label htmlFor="email" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("register_email_label")}
          </Label>
          <div className="relative">
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                setErrors((p) => ({ ...p, email: undefined }));
              }}
              placeholder={t("register_email_placeholder")}
              autoComplete="email"
              className="h-11 pl-10"
            />
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              <Mail size={16} />
            </span>
          </div>
          {errors.email && (
            <div className="mt-1 text-[11.5px] text-danger">{errors.email}</div>
          )}
        </div>

        <div className="mb-1.5">
          <Label htmlFor="password" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("register_password_label")}
          </Label>
          <div className="relative">
            <Input
              id="password"
              type={showPw ? "text" : "password"}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value);
                setErrors((p) => ({ ...p, password: undefined }));
              }}
              placeholder={t("register_password_placeholder")}
              autoComplete="new-password"
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
          {pwStrength && (
            <div className="mt-1.5 flex items-center gap-2">
              <div className="h-1 flex-1 overflow-hidden rounded-full bg-line">
                <div
                  className={`h-full rounded-full transition-all duration-200 ${strengthColor[pwStrength]} ${strengthWidth[pwStrength]}`}
                />
              </div>
              <span className={`text-[11px] font-bold ${strengthTextColor[pwStrength]}`}>
                {strengthText[pwStrength]}
              </span>
            </div>
          )}
          {errors.password && (
            <div className="mt-1 text-[11.5px] text-danger">{errors.password}</div>
          )}
        </div>

        <div className="mb-5">
          <Label htmlFor="confirm" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("register_confirm_label")}
          </Label>
          <div className="relative">
            <Input
              id="confirm"
              type={showCf ? "text" : "password"}
              value={confirm}
              onChange={(e) => {
                setConfirm(e.target.value);
                setErrors((p) => ({ ...p, confirm: undefined }));
              }}
              placeholder="••••••••"
              autoComplete="new-password"
              className="h-11 pl-10 pr-11"
            />
            <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
              {passwordsMatch ? (
                <CheckCircle2 size={16} className="text-success" />
              ) : (
                <Lock size={16} />
              )}
            </span>
            <button
              type="button"
              onClick={() => setShowCf((p) => !p)}
              aria-label={showCf ? t("auth_hide_password") : t("auth_show_password")}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
            >
              {showCf ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
          {errors.confirm && (
            <div className="mt-1 text-[11.5px] text-danger">{errors.confirm}</div>
          )}
        </div>

        <ReferralCodeField value={referral} onChange={setReferral} />

        <Button
          type="submit"
          disabled={loading}
          className="h-12 w-full text-[15px]"
        >
          {loading ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("register_submitting")}
            </span>
          ) : (
            t("register_submit")
          )}
        </Button>
      </form>

      <p className="mt-6 text-center text-[13px] text-ink-500">
        {t("register_already_have_account")}{" "}
        <button
          type="button"
          onClick={() => router.push("/login")}
          className="font-bold text-success transition-colors hover:text-success/80"
        >
          {t("register_sign_in_link")}
        </button>
      </p>
    </div>
  );
}