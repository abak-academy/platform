"use client";

import type { LucideIcon } from "lucide-react";
import { Card } from "@/components/ui/card";
import { useTranslation } from "@/lib/i18n";

interface UnderMaintenanceProps {
  icon: LucideIcon;
  title: string;
  description?: string;
  estimatedTimeline?: string;
}

export function UnderMaintenance({
  icon: Icon,
  title,
  description,
  estimatedTimeline,
}: UnderMaintenanceProps) {
  const { t } = useTranslation();
  const desc = description ?? t("maint_default_desc");
  return (
    <Card className="gap-0 py-0">
      <div className="flex flex-col items-center justify-center px-4 py-16 text-center">
        <div className="flex size-14 items-center justify-center rounded-[20px] bg-ember-50 text-ember-700">
          <Icon size={28} />
        </div>
        <h2 className="type-h3 mt-4 text-ink-900">{title}</h2>
        <p className="mt-2 max-w-sm text-sm text-muted-foreground">{desc}</p>
        {estimatedTimeline ? (
          <p className="type-label-sm mt-4 text-muted-foreground">{estimatedTimeline}</p>
        ) : null}
      </div>
    </Card>
  );
}
