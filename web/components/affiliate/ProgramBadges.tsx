import type { AffiliatorPrograms } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";

export function inAffiliate(programs?: AffiliatorPrograms): boolean {
  return programs?.affiliate !== false;
}

export function inPartner(programs?: AffiliatorPrograms): boolean {
  return Boolean(programs?.partner);
}

// ProgramBadges lists the programs an account belongs to. A missing programs
// object is a plain Affiliator account.
export function ProgramBadges({ programs }: { programs?: AffiliatorPrograms }) {
  const { t } = useTranslation();
  return (
    <span className="flex flex-wrap gap-1.5">
      {inAffiliate(programs) && <span className="rounded-full bg-brand-50 px-2 py-0.5 text-xs font-semibold text-brand-700">{t("admin_aff_program_affiliate")}</span>}
      {inPartner(programs) && <span className="rounded-full bg-amber-100 px-2 py-0.5 text-xs font-semibold text-amber-800">{t("admin_aff_program_partner")}</span>}
    </span>
  );
}
