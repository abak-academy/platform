"use client";

import { useState } from "react";
import { Flag } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { useAdminAffiliateFlags, useReviewAffiliateFlag } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { DICT, useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliateFlagRow } from "@/lib/types";

const STATUSES = ["open", "reviewed", "valid", "ignored"] as const;

export default function AdminAffiliateFlagsPage() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const [status, setStatus] = useState("open");
  const query = useAdminAffiliateFlags(status);
  const review = useReviewAffiliateFlag();

  if (!canManage) return <NoAccess />;

  const act = (id: string, next: "valid" | "ignored") =>
    review.mutate(
      { id, status: next },
      {
        onSuccess: () => toast.success(t("admin_aff_flags_reviewed")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
      },
    );

  const ruleLabel = (rule: string) => {
    const key = `aff_rule_${rule}`;
    return key in DICT.id ? t(key as I18nKey) : rule;
  };

  const columns: DataTableColumn<AffiliateFlagRow>[] = [
    {
      key: "who",
      header: t("admin_aff_col_affiliator"),
      cell: (r) => (
        <div>
          <div className="font-medium text-ink-900">{r.affiliator_name}</div>
          <div className="text-xs text-ink-500">{r.affiliator_code}</div>
        </div>
      ),
    },
    { key: "rule", header: t("admin_aff_col_rule"), cell: (r) => ruleLabel(r.rule) },
    {
      key: "detail",
      header: t("admin_aff_col_detail"),
      cell: (r) => <code className="text-xs text-ink-600">{r.payload ? JSON.stringify(r.payload) : "—"}</code>,
    },
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    {
      key: "actions",
      header: t("admin_aff_col_actions"),
      cell: (r) =>
        r.status === "open" ? (
          <div className="flex gap-2">
            <Button size="sm" onClick={() => act(r.id, "valid")} disabled={review.isPending}>
              {t("admin_aff_flags_valid")}
            </Button>
            <Button size="sm" variant="outline" onClick={() => act(r.id, "ignored")} disabled={review.isPending}>
              {t("admin_aff_flags_ignore")}
            </Button>
          </div>
        ) : (
          "—"
        ),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Flag} title={t("admin_aff_flags_title")} description={t("admin_aff_flags_desc")} />
      <select
        aria-label={t("aff_filter_status")}
        value={status}
        onChange={(e) => setStatus(e.target.value)}
        className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
      >
        {STATUSES.map((s) => (
          <option key={s} value={s}>
            {s === "open" ? t("admin_aff_all_flags") : affLabel(t, s)}
          </option>
        ))}
      </select>
      <AffiliateList query={query} columns={columns} rowKey={(r) => r.id} empty={t("admin_aff_flags_empty")} />
    </div>
  );
}
