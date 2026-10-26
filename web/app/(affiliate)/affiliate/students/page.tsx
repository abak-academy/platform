"use client";

import { Suspense } from "react";
import { Users } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateBindings } from "@/lib/hooks/affiliate";
import { useListParams } from "@/lib/hooks/use-list-params";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateBinding } from "@/lib/types";

const SOURCES = ["link", "manual_code", "checkout_code"] as const;
const STATUSES = ["active", "ended"] as const;

export default function AffiliateStudentsPage() {
  return (
    <Suspense fallback={null}>
      <StudentsContent />
    </Suspense>
  );
}

function StudentsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliateBindings({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    status: params.status,
    type: params.type,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliateBinding>[] = [
    { key: "name", header: t("aff_col_name"), cell: (r) => <span className="font-medium text-ink-900">{r.name}</span> },
    { key: "registered", header: t("aff_col_registered"), cell: (r) => formatAffDate(r.registered_at) },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "started", header: t("aff_col_started"), cell: (r) => formatAffDate(r.started_at) },
    { key: "ended", header: t("aff_col_ended"), cell: (r) => formatAffDate(r.ended_at) },
    { key: "source", header: t("aff_col_source"), cell: (r) => affLabel(t, r.source) },
    { key: "spend", header: t("aff_col_spend"), align: "right", cell: (r) => formatRupiah(r.total_spend) },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Users} title={t("aff_students_title")} description={t("aff_students_desc")} />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_students_search")}
        filters={[
          { key: "status", label: t("aff_filter_binding_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "type", label: t("aff_filter_source"), options: SOURCES.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
        dateLabel={t("aff_filter_start_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.binding_id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_students_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
