"use client";

import { useTranslation } from "@/lib/i18n";

type Strength = "weak" | "fair" | "strong";

export function passwordStrength(password: string): Strength | null {
  if (password.length === 0) return null;
  if (password.length < 6) return "weak";
  if (password.length < 10) return "fair";
  return "strong";
}

const BAR: Record<Strength, string> = { weak: "bg-danger w-1/3", fair: "bg-warn w-2/3", strong: "bg-success w-full" };
const TEXT: Record<Strength, string> = { weak: "text-danger", fair: "text-warn", strong: "text-success" };

// Same meter the student register page draws, shared so the two stay in step.
export function PasswordStrength({ password }: { password: string }) {
  const { t } = useTranslation();
  const strength = passwordStrength(password);
  if (!strength) return null;
  const label = { weak: t("register_pw_weak"), fair: t("register_pw_fair"), strong: t("register_pw_strong") }[strength];
  return (
    <div className="mt-1.5 flex items-center gap-2" data-testid="password-strength">
      <div className="h-1 flex-1 overflow-hidden rounded-full bg-line">
        <div className={`h-full rounded-full transition-all duration-200 ${BAR[strength]}`} />
      </div>
      <span className={`text-[11px] font-bold ${TEXT[strength]}`}>{label}</span>
    </div>
  );
}
