"use client";

import { Suspense } from "react";
import { Coins } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateCommissions } from "@/lib/hooks/affiliate";
import { useListParams } from "@/lib/hooks/use-list-params";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { formatCommissionRule } from "@/lib/affiliate-rules";
import type { AffiliateCommission } from "@/lib/types";

const STATUSES = ["pending", "available", "cancelled", "held"] as const;
const CATEGORIES = ["exam", "course", "book", "merchandise", "medal"] as const;

export default function AffiliateCommissionsPage() {
  return (
    <Suspense fallback={null}>
      <CommissionsContent />
    </Suspense>
  );
}

function CommissionsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  // The shared toolbar's "type" filter is the product category on this list.
  const query = useAffiliateCommissions({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    status: params.status,
    product_type: params.type,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliateCommission>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.paid_at ?? r.created_at) },
    { key: "student", header: t("aff_col_student"), cell: (r) => <span className="font-medium text-ink-900">{r.student_name ?? "—"}</span> },
    { key: "product", header: t("aff_col_product_name"), cell: (r) => r.product_name ?? "—" },
    { key: "category", header: t("aff_col_product"), cell: (r) => affLabel(t, r.product_type) },
    { key: "base", header: t("aff_col_purchase"), align: "right", cell: (r) => formatRupiah(r.base_amount) },
    { key: "pct", header: t("aff_col_rate"), align: "right", cell: (r) => formatCommissionRule(r.commission_type, r.commission_value, t("aff_per_unit")) },
    { key: "amount", header: t("aff_col_commission"), align: "right", cell: (r) => <span className="font-semibold">{formatRupiah(r.amount)}</span> },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    {
      key: "wait",
      header: t("aff_col_available_from"),
      cell: (r) => (r.wait_until ? formatAffDate(r.wait_until) : r.status === "pending" ? t("aff_comm_waiting_physical") : "—"),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Coins} title={t("aff_commissions_title")} description={t("aff_comm_desc")} />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_comm_search")}
        filters={[
          { key: "status", label: t("aff_col_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "type", label: t("aff_filter_category"), options: CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
        dateLabel={t("aff_filter_paid_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_commissions_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
