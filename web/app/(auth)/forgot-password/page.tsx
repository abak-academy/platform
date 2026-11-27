"use client";

import * as React from "react";
import Link from "next/link";
import { Mail, Loader2, AlertCircle, MailCheck } from "lucide-react";

import { useForgotPassword } from "@/lib/hooks/auth";
import { ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

export default function ForgotPasswordPage() {
  const { t } = useTranslation();
  const forgot = useForgotPassword();
  const [email, setEmail] = React.useState("");
  const [sent, setSent] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    try {
      await forgot.mutateAsync(email.trim());
      setSent(true);
    } catch (err) {
      if (err instanceof ApiError && err.code === "rate_limited") setError(t("forgot_rate_limited"));
      else if (err instanceof ApiError && err.code === "email_delivery_failed") setError(t("forgot_email_failed"));
      else setError(t("forgot_failed"));
    }
  };

  const backLink = (
    <p className="mt-6 text-center text-[13px]">
      <Link href="/login" className="font-bold text-success transition-colors hover:text-success/80">
        {t("forgot_back_to_login")}
      </Link>
    </p>
  );

  if (sent) {
    return (
      <div className="w-full max-w-[372px]">
        <MailCheck size={32} className="mb-4 text-success" />
        <h2 className="font-serif text-[27px] font-bold leading-tight text-ink-900">{t("forgot_sent_title")}</h2>
        <p className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("forgot_sent_body")}</p>
        {backLink}
      </div>
    );
  }

  return (
    <div className="w-full max-w-[372px]">
      <div className="mb-7">
        <h2 className="font-serif text-[27px] font-bold leading-tight text-ink-900">{t("forgot_title")}</h2>
        <p className="mt-3 text-[13.5px] leading-[1.55] text-ink-600">{t("forgot_subtitle")}</p>
      </div>

      <form onSubmit={onSubmit} noValidate>
        <Label htmlFor="email" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
          {t("forgot_email_label")}
        </Label>
        <div className="relative">
          <Input
            id="email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="nama@email.com"
            autoComplete="email"
            required
            className="h-11 pl-10"
          />
          <span className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-ink-400">
            <Mail size={16} />
          </span>
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

        <Button type="submit" disabled={forgot.isPending || !email.trim()} className="mt-5 h-12 w-full text-[15px]">
          {forgot.isPending ? (
            <span className="flex items-center gap-2">
              <Loader2 size={16} className="animate-spin" />
              {t("forgot_submitting")}
            </span>
          ) : (
            t("forgot_submit")
          )}
        </Button>
      </form>
      {backLink}
    </div>
  );
}
