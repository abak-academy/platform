"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { User, Mail, Phone, Lock, Eye, EyeOff, CheckCircle2, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { useRegisterAffiliator } from "@/lib/hooks/auth";
import { useAffiliateProgramStatus } from "@/lib/hooks/affiliate";
import { ApiError } from "@/lib/api";
import { AFFILIATE_TERMS_VERSION } from "@/lib/affiliate-terms";
import { adoptResendBudget } from "@/lib/otp-budget";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordStrength } from "@/components/auth/PasswordStrength";
import { useTranslation, type I18nKey } from "@/lib/i18n";

type Errors = Partial<Record<"name" | "email" | "phone" | "password" | "confirm", string>>;

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

const iconClass = "pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400";
const eyeClass = "absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600";

export default function AffiliateRegisterPage() {
  const router = useRouter();
  const { t } = useTranslation();
  const register = useRegisterAffiliator();
  const programStatus = useAffiliateProgramStatus();
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [phone, setPhone] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [confirm, setConfirm] = React.useState("");
  const [showPw, setShowPw] = React.useState(false);
  const [showCf, setShowCf] = React.useState(false);
  const [terms, setTerms] = React.useState(false);
  const [errors, setErrors] = React.useState<Errors>({});

  const validate = (): Errors => {
    const e: Errors = {};
    if (!name.trim()) e.name = t("register_err_name_required");
    if (!EMAIL_RE.test(email)) e.email = t("register_err_email_invalid");
    if (!phone.trim()) e.phone = t("aff_reg_err_phone_required");
    if (password.length < 8) e.password = t("register_err_password_min");
    if (confirm !== password) e.confirm = t("register_err_confirm_mismatch");
    return e;
  };

  const clear = (field: keyof Errors) => setErrors((p) => ({ ...p, [field]: undefined }));

  const onSubmit = async (ev: React.FormEvent) => {
    ev.preventDefault();
    if (!terms) return;
    const errs = validate();
    setErrors(errs);
    if (Object.keys(errs).length > 0) return;

    try {
      const res = await register.mutateAsync({
        name: name.trim(),
        email: email.trim(),
        phone: phone.trim(),
        password,
        terms_version: AFFILIATE_TERMS_VERSION,
      });
      if (res.otp_required && res.pending_token) {
        sessionStorage.setItem("abak-pending-token", res.pending_token);
        sessionStorage.setItem("abak-email-failed", res.email_failed ? "1" : "0");
        sessionStorage.setItem("abak-otp-active", res.otp_active ? "1" : "0");
        adoptResendBudget(Number(res.resend_after) || 60);
        if (res.email_failed) toast.warning(t("otp_email_failed_banner"));
        else toast.success(t("register_success"));
        router.push(`/otp?id=${encodeURIComponent(email.trim())}`);
      } else {
        toast.success(t("register_success"));
        router.push("/login");
      }
    } catch (err) {
      if (err instanceof ApiError && err.code === "affiliate_registration_closed") {
        toast.error(t("aff_reg_closed"));
      } else if (err instanceof ApiError && err.code === "affiliate_registration_cap") {
        toast.error(t("aff_reg_cap"));
      } else {
        toast.error(err instanceof ApiError ? err.message : t("register_failed"));
      }
    }
  };

  const loading = register.isPending;
  const passwordsMatch = confirm.length > 0 && confirm === password;

  const field = (id: string, label: I18nKey, error: string | undefined, input: React.ReactNode, className = "mb-3.5") => (
    <div className={className}>
      <Label htmlFor={id} className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
        {t(label)}
      </Label>
      {input}
      {error && <div className="mt-1 text-[11.5px] text-danger">{error}</div>}
    </div>
  );

  if (programStatus.data?.enabled === false) {
    return (
      <div className="w-full max-w-[372px]" role="status">
        <h2 className="font-serif text-[27px] font-bold leading-tight tracking-[-0.01em] text-ink-900">{t("aff_reg_title")}</h2>
        <p className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("aff_register_closed")}</p>
        <Link href="/login" className="mt-5 inline-block text-sm font-bold text-success hover:text-success/80">
          {t("aff_register_closed_back")}
        </Link>
      </div>
    );
  }

  return (
    <div className="w-full max-w-[372px]">
      <div className="mb-7">
        <div className="mb-2 text-[11.5px] font-bold uppercase tracking-[0.06em] text-success">{t("aff_reg_eyebrow")}</div>
        <h2 className="font-serif text-[27px] font-bold leading-tight tracking-[-0.01em] text-ink-900">{t("aff_reg_title")}</h2>
        <p className="mt-2 text-[13.5px] leading-[1.55] text-ink-500">{t("aff_reg_subtitle")}</p>
      </div>

      <form onSubmit={onSubmit} noValidate>
        {field(
          "name",
          "register_full_name_label",
          errors.name,
          <div className="relative">
            <Input
              id="name"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                clear("name");
              }}
              placeholder={t("register_name_placeholder")}
              autoComplete="name"
              className="h-11 pl-10"
            />
            <span className={iconClass}>
              <User size={16} />
            </span>
          </div>,
        )}
        {field(
          "email",
          "register_email_label",
          errors.email,
          <div className="relative">
            <Input
              id="email"
              type="email"
              value={email}
              onChange={(e) => {
                setEmail(e.target.value);
                clear("email");
              }}
              placeholder={t("register_email_placeholder")}
              autoComplete="email"
              className="h-11 pl-10"
            />
            <span className={iconClass}>
              <Mail size={16} />
            </span>
          </div>,
        )}
        {field(
          "phone",
          "aff_reg_phone_label",
          errors.phone,
          <div className="relative">
            <Input
              id="phone"
              type="tel"
              value={phone}
              onChange={(e) => {
                setPhone(e.target.value);
                clear("phone");
              }}
              placeholder={t("aff_reg_phone_placeholder")}
              autoComplete="tel"
              className="h-11 pl-10"
            />
            <span className={iconClass}>
              <Phone size={16} />
            </span>
          </div>,
        )}
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
                clear("password");
              }}
              placeholder={t("register_password_placeholder")}
              autoComplete="new-password"
              className="h-11 pl-10 pr-11"
            />
            <span className={iconClass}>
              <Lock size={16} />
            </span>
            <button
              type="button"
              onClick={() => setShowPw((p) => !p)}
              aria-label={showPw ? t("auth_hide_password") : t("auth_show_password")}
              className={eyeClass}
            >
              {showPw ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>
          <PasswordStrength password={password} />
          {errors.password && <div className="mt-1 text-[11.5px] text-danger">{errors.password}</div>}
        </div>
        {field(
          "confirm",
          "register_confirm_label",
          errors.confirm,
          <div className="relative">
            <Input
              id="confirm"
              type={showCf ? "text" : "password"}
              value={confirm}
              onChange={(e) => {
                setConfirm(e.target.value);
                clear("confirm");
              }}
              placeholder="••••••••"
              autoComplete="new-password"
              className="h-11 pl-10 pr-11"
            />
            <span className={iconClass}>
              {passwordsMatch ? <CheckCircle2 size={16} className="text-success" /> : <Lock size={16} />}
            </span>
            <button
              type="button"
              onClick={() => setShowCf((p) => !p)}
              aria-label={showCf ? t("auth_hide_password") : t("auth_show_password")}
              className={eyeClass}
            >
              {showCf ? <EyeOff size={16} /> : <Eye size={16} />}
            </button>
          </div>,
          "mb-5",
        )}

        <div className="mb-5">
          <label className="flex items-start gap-2 text-[13px] text-ink-600">
            <input
              type="checkbox"
              checked={terms}
              onChange={(e) => setTerms(e.target.checked)}
              className="mt-0.5 size-4 shrink-0 accent-success"
            />
            <span>
              {t("aff_reg_terms_prefix")}{" "}
              <Link href="/affiliate-terms" target="_blank" className="font-bold text-success transition-colors hover:text-success/80">
                {t("aff_reg_terms_link")}
              </Link>
            </span>
          </label>
          {!terms && <div className="mt-1.5 text-[11.5px] text-ink-500">{t("aff_reg_terms_hint")}</div>}
        </div>

        <Button type="submit" disabled={loading || !terms} className="h-12 w-full text-[15px]">
          {loading ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("register_submitting")}
            </span>
          ) : (
            t("aff_reg_submit")
          )}
        </Button>
      </form>

      <p className="mt-6 text-center text-[13px] text-ink-500">
        {t("register_already_have_account")}{" "}
        <Link href="/login" className="font-bold text-success transition-colors hover:text-success/80">
          {t("register_sign_in_link")}
        </Link>
      </p>
      <p className="mt-2 text-center text-[13px] text-ink-500">
        <Link href="/register" className="font-semibold text-ink-600 transition-colors hover:text-ink-900">
          {t("aff_reg_student_link")}
        </Link>
      </p>
    </div>
  );
}
