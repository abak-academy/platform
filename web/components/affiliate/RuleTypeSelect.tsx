"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { COMMISSION_RULE_TYPES } from "@/lib/affiliate-rules";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateCommissionRuleType } from "@/lib/types";

interface RuleTypeSelectProps {
  id?: string;
  label: string;
  value: AffiliateCommissionRuleType;
  onChange: (value: AffiliateCommissionRuleType) => void;
}

// Percent or fixed rupiah per unit; the same picker on the level rules and the product override form.
export function RuleTypeSelect({ id, label, value, onChange }: RuleTypeSelectProps) {
  const { t } = useTranslation();
  return (
    <Select value={value} onValueChange={(next) => onChange(next as AffiliateCommissionRuleType)}>
      <SelectTrigger id={id} aria-label={label} className="w-full">
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {COMMISSION_RULE_TYPES.map((type) => (
          <SelectItem key={type} value={type}>
            {t(type === "fixed" ? "admin_aff_rule_fixed" : "admin_aff_rule_percent")}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
