"use client";

import { ShieldOff } from "lucide-react";
import { EmptyState } from "@/components/ui/empty-state";
import { useTranslation } from "@/lib/i18n";

export function NoAccess() {
  const { t } = useTranslation();

  return (
    <div data-testid="no-access" className="fade-in">
      <EmptyState framed icon={ShieldOff} title={t("no_access_title")} text={t("no_access_desc")} />
    </div>
  );
}
