"use client";

import * as React from "react";

import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useValidateAffiliateCode } from "@/lib/hooks/affiliate";
import { useTranslation } from "@/lib/i18n";

interface ReferralCodeFieldProps {
  value: string;
  onChange: (value: string) => void;
}

// The check is advisory: an invalid code never blocks sign-up, the server just
// skips the binding.
export function ReferralCodeField({ value, onChange }: ReferralCodeFieldProps) {
  const { t } = useTranslation();
  const validate = useValidateAffiliateCode();
  const [result, setResult] = React.useState<{ valid: boolean; name: string } | null>(null);

  const check = () => {
    const code = value.trim();
    if (!code) {
      setResult(null);
      return;
    }
    validate.mutate(code, {
      onSuccess: (res) => setResult({ valid: res.valid, name: res.affiliator_name }),
      onError: () => setResult(null),
    });
  };

  return (
    <div className="mb-5">
      <Label htmlFor="referral_code" className="mb-1.5 text-[12.5px] font-semibold text-ink-600">
        {t("register_referral_label")}
      </Label>
      <Input
        id="referral_code"
        value={value}
        onChange={(e) => {
          onChange(e.target.value.toUpperCase());
          setResult(null);
        }}
        onBlur={check}
        placeholder={t("register_referral_placeholder")}
        autoComplete="off"
        className="h-11"
      />
      {result && (
        <div className={`mt-1 text-[11.5px] ${result.valid ? "text-success" : "text-ink-500"}`}>
          {result.valid
            ? t("register_referral_valid").replace("{name}", result.name)
            : t("register_referral_invalid")}
        </div>
      )}
    </div>
  );
}
