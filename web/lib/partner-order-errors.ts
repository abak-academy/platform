import { ApiError } from "@/lib/api";
import type { I18nKey } from "@/lib/i18n";

type Translate = (key: I18nKey) => string;

// partnerOrderErrorMessage turns what the server refuses a Mitra order for into
// a sentence the Mitra can act on: who is not allowed, who already owns the
// product, how much voucher quota is left.
export function partnerOrderErrorMessage(err: unknown, t: Translate): string {
  if (!(err instanceof ApiError)) return t("partner_err_generic");
  const details = err.body?.details as { students?: { name?: string }[]; remaining?: number; min?: number } | undefined;
  switch (err.code) {
    case "student_not_allowed":
      return t("partner_err_student_not_allowed");
    case "product_not_allowed":
      return t("partner_err_product_not_allowed");
    case "participant_already_owns": {
      const names = (details?.students ?? []).map((s) => s.name).filter(Boolean).join(", ");
      return t("partner_err_owns").replace("{names}", names);
    }
    case "promo_quota_insufficient":
      return t("partner_err_quota").replace("{n}", String(details?.remaining ?? 0));
    case "promo_min_students":
      return t("partner_err_min_students").replace("{n}", String(details?.min ?? 0));
    case "invalid_promo":
      return t("partner_err_voucher");
    case "zero_net_new_participants":
      return t("partner_err_all_own");
    case "account_deactivated":
      return t("prog_readonly_banner");
    case "partner_program_off":
      return t("partner_program_off_banner");
    default:
      return err.message || t("partner_err_generic");
  }
}
