"use client";

import { Suspense } from "react";
import { Download, Flag } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { downloadAffiliateCsv, useAdminAffiliateFlags, useReviewAffiliateFlag } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { DICT, useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliateFlagRow } from "@/lib/types";

const STATUSES = ["open", "reviewed", "valid", "ignored"] as const;

export default function AdminAffiliateFlagsPage() {
  return (
    <Suspense fallback={null}>
      <FlagsContent />
    </Suspense>
  );
}

function FlagsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const canManage = useHasCapability("affiliates:*");
  const queue = { q: params.q, status: params.status, from: params.from, to: params.to };
  const query = useAdminAffiliateFlags({ ...queue, page: params.page, per_page: params.per_page });
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

  const exportCsv = async () => {
    try {
      await downloadAffiliateCsv("flags", queue);
    } catch {
      toast.error(t("admin_aff_export_failed"));
    }
  };

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
      <AdminPageHeader
        icon={Flag}
        title={t("admin_aff_flags_title")}
        description={t("admin_aff_flags_desc")}
        actions={
          <Button variant="outline" onClick={exportCsv}>
            <Download className="size-4" aria-hidden />
            {t("admin_aff_export_csv")}
          </Button>
        }
      />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("admin_aff_search_queue")}
        filters={[{ key: "status", label: t("aff_col_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("admin_aff_date_detected")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_flags_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
