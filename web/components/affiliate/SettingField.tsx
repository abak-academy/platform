"use client";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import type { FieldHelp } from "@/lib/affiliate-settings-help";
import { useTranslation } from "@/lib/i18n";

interface SettingFieldProps {
  id: string;
  label: string;
  hint: string;
  help: FieldHelp;
  value: string;
  onChange: (value: string) => void;
  // Shown greyed out when the field is empty, e.g. the global value a level falls back to.
  placeholder?: string;
  disabled?: boolean;
}

const UNIT_KEYS = {
  days: "admin_aff_unit_days",
  months: "admin_aff_unit_months",
  rupiah: "admin_aff_unit_rupiah",
  percent: "admin_aff_unit_percent",
} as const;

// One setting: label, an input with its unit, and a one-sentence explanation with an example.
export function SettingField({ id, label, hint, help, value, onChange, placeholder, disabled }: SettingFieldProps) {
  const { t } = useTranslation();
  const unit = help.unit ? t(UNIT_KEYS[help.unit]) : null;
  return (
    <div className="space-y-1.5">
      <Label htmlFor={id}>{label}</Label>
      <div className="relative">
        <Input
          id={id}
          inputMode="decimal"
          value={value}
          placeholder={placeholder}
          disabled={disabled}
          onChange={(e) => onChange(e.target.value)}
          aria-describedby={`${id}-hint`}
          className={unit ? "pr-16" : undefined}
        />
        {unit && (
          <span className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-xs text-ink-500">{unit}</span>
        )}
      </div>
      <p id={`${id}-hint`} className="text-xs leading-relaxed text-ink-500">
        {hint} <span className="text-ink-600">{t("admin_aff_example")}: {help.example}</span>
      </p>
    </div>
  );
}
