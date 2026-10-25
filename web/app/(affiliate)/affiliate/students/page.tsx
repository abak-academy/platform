"use client";

import { useState } from "react";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateBindings } from "@/lib/hooks/affiliate";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateBinding } from "@/lib/types";

export default function AffiliateStudentsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const query = useAffiliateBindings({ page });

  const columns: DataTableColumn<AffiliateBinding>[] = [
    { key: "name", header: t("aff_col_name"), cell: (r) => r.name },
    { key: "registered", header: t("aff_col_registered"), cell: (r) => formatAffDate(r.registered_at) },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "started", header: t("aff_col_started"), cell: (r) => formatAffDate(r.started_at) },
    { key: "ended", header: t("aff_col_ended"), cell: (r) => formatAffDate(r.ended_at) },
    { key: "source", header: t("aff_col_source"), cell: (r) => affLabel(t, r.source) },
    { key: "spend", header: t("aff_col_spend"), align: "right", cell: (r) => formatRupiah(r.total_spend) },
  ];

  return (
    <div className="space-y-4">
      <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_students_title")}</h1>
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.binding_id}
        page={page}
        onPageChange={setPage}
        empty={t("aff_students_empty")}
      />
    </div>
  );
}
