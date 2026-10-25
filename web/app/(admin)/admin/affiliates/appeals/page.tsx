"use client";

import { useState } from "react";
import { Scale } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { AppealBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import { useAdminAffiliateAppeals, useDecideAffiliateAppeal } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliateAppealRow } from "@/lib/types";

const STATUSES = ["pending", "upheld", "rejected"] as const;

export default function AdminAffiliateAppealsPage() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const [status, setStatus] = useState("pending");
  const query = useAdminAffiliateAppeals(status);
  const decide = useDecideAffiliateAppeal();

  if (!canManage) return <NoAccess />;

  const act = (id: string, outcome: "upheld" | "rejected") =>
    decide.mutate(
      { id, outcome },
      {
        onSuccess: () => toast.success(t("admin_aff_appeals_decided")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed")),
      },
    );

  const columns: DataTableColumn<AffiliateAppealRow>[] = [
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
    { key: "sanction", header: t("aff_col_sanction"), cell: (r) => affLabel(t, r.sanction) },
    { key: "description", header: t("aff_col_description"), cell: (r) => r.description },
    { key: "appeal", header: t("admin_aff_col_appeal_text"), cell: (r) => r.appeal_text ?? "—" },
    { key: "status", header: t("aff_col_appeal"), cell: (r) => <AppealBadge status={r.appeal_status} /> },
    {
      key: "actions",
      header: t("admin_aff_col_actions"),
      cell: (r) =>
        r.appeal_status === "pending" ? (
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => act(r.id, "upheld")} disabled={decide.isPending}>
              {t("admin_aff_appeals_uphold")}
            </Button>
            <Button size="sm" onClick={() => act(r.id, "rejected")} disabled={decide.isPending}>
              {t("admin_aff_appeals_overturn")}
            </Button>
          </div>
        ) : (
          "—"
        ),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Scale} title={t("admin_aff_appeals_title")} description={t("admin_aff_appeals_desc")} />
      <select
        aria-label={t("aff_filter_status")}
        value={status}
        onChange={(e) => setStatus(e.target.value)}
        className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
      >
        {STATUSES.map((s) => (
          <option key={s} value={s}>
            {t(`aff_appeal_v_${s}` as I18nKey)}
          </option>
        ))}
      </select>
      <AffiliateList query={query} columns={columns} rowKey={(r) => r.id} empty={t("admin_aff_appeals_empty")} />
    </div>
  );
}
