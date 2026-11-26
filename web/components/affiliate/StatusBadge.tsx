"use client";

import { Badge, statusTone } from "@/components/ui/badge";
import { affLabel } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

// Option-chip classes (OptionSelect); the badge itself uses the shared Badge variants.
export const STATUS_TONE: Record<string, string> = {
  paid: "bg-teal-50 text-teal-700",
  available: "bg-teal-50 text-teal-700",
  active: "bg-teal-50 text-teal-700",
  pending: "bg-gold-50 text-gold-700",
  requested: "bg-gold-50 text-gold-700",
  processing: "bg-gold-50 text-gold-700",
  pending_approval: "bg-gold-50 text-gold-700",
  suspended: "bg-gold-50 text-gold-700",
  cancelled: "bg-danger-50 text-danger-600",
  rejected: "bg-danger-50 text-danger-600",
  deactivated: "bg-danger-50 text-danger-600",
};

const BADGE_TONE: Record<string, Parameters<typeof Badge>[0]["variant"]> = {
  available: "teal",
  requested: "gold",
  processing: "gold",
  pending_approval: "gold",
  suspended: "gold",
  deactivated: "danger",
};

export function StatusBadge({ status }: { status: string }) {
  const { t } = useTranslation();
  return <Badge variant={BADGE_TONE[status] ?? statusTone(status)}>{affLabel(t, status)}</Badge>;
}
