import { formatRupiah } from "@/lib/format";
import type { AffiliateCommissionRuleType } from "@/lib/types";

export const COMMISSION_RULE_TYPES: readonly AffiliateCommissionRuleType[] = ["percent", "fixed"];

export function formatCommissionRule(type: string, value: number, perUnit: string): string {
  return type === "fixed" ? `${formatRupiah(value)}${perUnit}` : `${value}%`;
}
