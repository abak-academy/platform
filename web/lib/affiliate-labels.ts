import { DICT, type I18nKey } from "@/lib/i18n";
import type { AffiliateLedgerEntry } from "@/lib/types";

// Server enum codes map to aff_v_<code>; an unknown code shows as-is rather
// than an empty cell when the server adds a value before the web does.
export function affLabel(t: (key: I18nKey) => string, code: string): string {
  const key = `aff_v_${code}`;
  return key in DICT.id ? t(key as I18nKey) : code;
}

export function formatAffDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  // A bare YYYY-MM-DD is a calendar day, not an instant: read it as that day
  // locally so it never slips to the day before.
  const day = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  const date = day ? new Date(Number(day[1]), Number(day[2]) - 1, Number(day[3])) : new Date(iso);
  return date.toLocaleDateString("id-ID", { day: "numeric", month: "short", year: "numeric" });
}

// One readable line per balance change; the stored note is never shown except an admin's reason.
export function ledgerDescription(t: (key: I18nKey) => string, row: AffiliateLedgerEntry): string {
  const student = row.student_name ?? "";
  const product = row.product_name ?? "";
  switch (row.entry_type) {
    case "commission":
      return student && product
        ? t("aff_ledger_desc_commission").replace("{student}", student).replace("{product}", product)
        : t("aff_ledger_desc_commission_plain");
    case "refund_deduction":
      return student && product
        ? t("aff_ledger_desc_refund").replace("{student}", student).replace("{product}", product)
        : t("aff_ledger_desc_refund_plain");
    case "adjustment":
      return row.note || t("aff_ledger_desc_adjustment");
    case "payout":
      return t("aff_ledger_desc_payout");
    case "payout_reversal":
      return t("aff_ledger_desc_reversal");
    default:
      return row.entry_type;
  }
}
