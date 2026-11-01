"use client";

import { AlertTriangle } from "lucide-react";
import { useAffiliateMe } from "@/lib/hooks/affiliate";
import { useTranslation } from "@/lib/i18n";

// Shown on every affiliator menu while the admin has the program switched off.
export function ProgramOffBanner() {
  const { t } = useTranslation();
  const me = useAffiliateMe();
  if (me.data?.program_enabled !== false) return null;
  return (
    <div role="status" className="mb-4 flex items-start gap-3 rounded-lg border border-warn/30 bg-warn-bg px-5 py-3 text-sm text-warn">
      <AlertTriangle className="mt-0.5 size-4 shrink-0" aria-hidden />
      <p>{t("aff_program_off_banner")}</p>
    </div>
  );
}
