"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Eye, EyeOff, Loader2, AlertCircle } from "lucide-react";

import { useLogin } from "@/lib/hooks/auth";
import { redirectForRole } from "@/lib/auth-redirect";
import { loginErrorMessage } from "@/lib/auth-error";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";

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
      router.push(redirectForRole(data.user?.role));
    } catch (err) {
      setError(loginErrorMessage(err, t));
    }
  };

  const loading = login.isPending;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <h1 className="font-[family-name:var(--font-display)] text-[36px] font-extrabold leading-[1.05] tracking-[-0.04em] text-ink-900">{t("admin_login_heading")}</h1>
        <p className="text-ink-600">{t("admin_login_sub")}</p>
      </div>

      <form onSubmit={onSubmit} noValidate>
        <div className="mb-4">
          <Label htmlFor="identifier" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
            {t("login_identifier_label")}
          </Label>
          <Input
            id="identifier"
            value={identifier}
            onChange={(e) => setIdentifier(e.target.value)}
            autoComplete="username"
            required
            className="h-11"
          />
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
              autoComplete="current-password"
              required
              className="h-11 pr-11"
            />
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

        <Button type="submit" disabled={loading} className="mt-5 h-12 w-full text-[15px]">
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
        {t("admin_login_not_admin")}{" "}
        <Link href="/login" className="font-bold text-success transition-colors hover:text-success/80">
          {t("admin_login_student_link")}
        </Link>
      </p>
    </div>
  );
}
