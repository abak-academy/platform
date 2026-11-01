"use client";

import { Badge } from "@/components/ui/badge";
import { affLabel } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

export const STATUS_TONE: Record<string, string> = {
  paid: "bg-success/15 text-success",
  available: "bg-success/15 text-success",
  active: "bg-success/15 text-success",
  pending: "bg-warn/15 text-warn",
  requested: "bg-warn/15 text-warn",
  processing: "bg-warn/15 text-warn",
  pending_approval: "bg-warn/15 text-warn",
  suspended: "bg-warn/15 text-warn",
  cancelled: "bg-danger/15 text-danger",
  rejected: "bg-danger/15 text-danger",
  deactivated: "bg-danger/15 text-danger",
};

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return (
    <Badge variant="secondary" className={STATUS_TONE[status] ?? "bg-surface-2 text-ink-600"}>
      {affLabel(t, status)}
    </Badge>
  );
}
