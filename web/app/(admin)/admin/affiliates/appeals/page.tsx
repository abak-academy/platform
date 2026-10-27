"use client";

import { Suspense } from "react";
import { Download, Scale } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { AppealBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import { downloadAffiliateCsv, useAdminAffiliateAppeals, useDecideAffiliateAppeal } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliateAppealRow } from "@/lib/types";

const STATUSES = ["pending", "upheld", "rejected"] as const;

export default function AdminAffiliateAppealsPage() {
  return (
    <Suspense fallback={null}>
      <AppealsContent />
    </Suspense>
  );
}

function AppealsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const canManage = useHasCapability("affiliates:*");
  const queue = { q: params.q, status: params.status, from: params.from, to: params.to };
  const query = useAdminAffiliateAppeals({ ...queue, page: params.page, per_page: params.per_page });
  const decide = useDecideAffiliateAppeal();

  if (!canManage) return <NoAccess />;

  const exportCsv = async () => {
    try {
      await downloadAffiliateCsv("appeals", queue);
    } catch {
      toast.error(t("admin_aff_export_failed"));
    }
  };

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
      <AdminPageHeader
        icon={Scale}
        title={t("admin_aff_appeals_title")}
        description={t("admin_aff_appeals_desc")}
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
        filters={[{ key: "status", label: t("aff_col_appeal"), options: STATUSES.map((v) => ({ value: v, label: t(`aff_appeal_v_${v}` as I18nKey) })) }]}
        dateLabel={t("admin_aff_date_violation")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_appeals_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
