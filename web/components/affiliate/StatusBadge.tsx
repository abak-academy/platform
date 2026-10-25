"use client";

import { Badge } from "@/components/ui/badge";
import { affLabel } from "@/lib/affiliate-labels";
import { useTranslation, type I18nKey } from "@/lib/i18n";

const TONE: Record<string, string> = {
  paid: "bg-success/15 text-success",
  available: "bg-success/15 text-success",
  active: "bg-success/15 text-success",
  upheld: "bg-warn/15 text-warn",
  pending: "bg-warn/15 text-warn",
  requested: "bg-warn/15 text-warn",
  processing: "bg-warn/15 text-warn",
  held: "bg-warn/15 text-warn",
  pending_approval: "bg-warn/15 text-warn",
  suspended: "bg-warn/15 text-warn",
  cancelled: "bg-danger/15 text-danger",
  rejected: "bg-danger/15 text-danger",
  deactivated: "bg-danger/15 text-danger",
};

const APPEAL_TONE: Record<string, string> = {
  pending: "bg-warn/15 text-warn",
  upheld: "bg-danger/15 text-danger",
  rejected: "bg-success/15 text-success",
};

// An appeal outcome names what happened to the sanction, so it needs its own
// copy: the shared "Ditolak" would read as the appeal being refused.
export function AppealBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return (
    <Badge variant="secondary" className={APPEAL_TONE[status] ?? "bg-surface-2 text-ink-600"}>
      {t(`aff_appeal_v_${status}` as I18nKey)}
    </Badge>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return (
    <Badge variant="secondary" className={TONE[status] ?? "bg-surface-2 text-ink-600"}>
      {affLabel(t, status)}
    </Badge>
  );
}
