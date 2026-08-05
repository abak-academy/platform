"use client";

import { ShieldOff } from "lucide-react";
import { useTranslation } from "@/lib/i18n";

export function NoAccess() {
  const { t } = useTranslation();

  return (
    <div className="md-card-outlined flex flex-col items-center justify-center gap-3 py-16 text-center fade-in">
      <ShieldOff size={40} className="text-muted-foreground" />
      <h2 className="text-title-large">{t("no_access_title")}</h2>
      <p className="max-w-md text-muted-foreground">{t("no_access_desc")}</p>
    </div>
  );
}
