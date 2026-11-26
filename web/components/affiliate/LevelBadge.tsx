"use client";

import { Crown } from "lucide-react";
import { Badge, statusTone } from "@/components/ui/badge";
import { affLabel } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

// Option-chip classes (OptionSelect); the badge itself uses the shared Badge variants.
export const LEVEL_TONE: Record<string, string> = {
  bronze: "bg-ember-50 text-ember-700",
  silver: "bg-mute-50 text-mute-700",
  gold: "bg-gold-50 text-gold-700",
};

export function LevelBadge({ level }: { level: string }) {
  const { t } = useTranslation();
  return (
    <Badge variant={statusTone(level)} className="gap-[5px] [&>svg]:size-3.5">
      <Crown aria-hidden />
      {affLabel(t, level)}
    </Badge>
  );
}
