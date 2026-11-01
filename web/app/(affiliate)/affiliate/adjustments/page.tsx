"use client";

import { SlidersHorizontal } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateAdjustments } from "@/lib/hooks/affiliate";
import { useListParams } from "@/lib/hooks/use-list-params";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdjustment } from "@/lib/types";

const CATEGORIES = ["correction", "offline", "bonus", "other"] as const;

export default function AffiliateAdjustmentsPage() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliateAdjustments({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    type: params.type,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliateAdjustment>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "category", header: t("aff_col_category"), cell: (r) => affLabel(t, r.category) },
    {
      key: "amount",
      header: t("aff_col_amount"),
      align: "right",
      cell: (r) => <span className={r.amount < 0 ? "font-semibold text-danger" : "font-semibold text-ink-900"}>{formatRupiah(r.amount)}</span>,
    },
    { key: "reason", header: t("aff_col_reason_text"), cell: (r) => r.reason },
    { key: "applied", header: t("aff_col_applied"), cell: (r) => (r.applied ? t("aff_adj_applied_yes") : t("aff_adj_applied_no")) },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={SlidersHorizontal} title={t("aff_adj_title")} description={t("aff_adj_desc")} />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_adj_search")}
        filters={[{ key: "type", label: t("aff_adj_filter_category"), options: CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("aff_adj_filter_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_adj_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
