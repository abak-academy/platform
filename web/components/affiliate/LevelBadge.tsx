"use client";

import { Badge } from "@/components/ui/badge";
import { affLabel } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

const TONE: Record<string, string> = {
  bronze: "bg-[#CD7F32]/20 text-[#8A5320]",
  silver: "bg-slate-400/20 text-slate-600",
  gold: "bg-amber-400/25 text-amber-700",
};

export function LevelBadge({ level }: { level: string }) {
  const { t } = useTranslation();
  return (
    <Badge variant="secondary" className={TONE[level] ?? "bg-surface-2 text-ink-600"}>
      {affLabel(t, level)}
    </Badge>
  );
}
