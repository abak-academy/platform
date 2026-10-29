"use client";

import { TriangleAlert } from "lucide-react";
import { useAffiliatorMe } from "@/lib/hooks/affiliates";
import { useTranslation } from "@/lib/i18n";

// A deactivated affiliator keeps a read-only session, so every affiliator page
// says why its write actions fail instead of letting the buttons error out.
export function useAffiliatorReadOnly(): boolean {
  const { data } = useAffiliatorMe();
  return data?.status === "deactivated";
}

export function AffiliatorNotice() {
  const { t } = useTranslation();
  const readOnly = useAffiliatorReadOnly();
  if (!readOnly) return null;
  return (
    <div role="status" className="mb-6 flex items-start gap-3 rounded-[12px] border border-warn/40 bg-warn-bg px-4 py-3 text-sm text-ink-900">
      <TriangleAlert size={18} className="mt-0.5 shrink-0 text-warn" aria-hidden />
      <p>{t("aff_deactivated_notice")}</p>
    </div>
  );
}
