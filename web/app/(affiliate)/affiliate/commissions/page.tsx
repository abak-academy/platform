"use client";

import { useState } from "react";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateCommissions } from "@/lib/hooks/affiliate";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateCommission } from "@/lib/types";

const STATUSES = ["pending", "available", "paid", "cancelled", "held"] as const;

export default function AffiliateCommissionsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [status, setStatus] = useState("");
  const query = useAffiliateCommissions({ page, status });

  const columns: DataTableColumn<AffiliateCommission>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "product", header: t("aff_col_product"), cell: (r) => affLabel(t, r.product_type) },
    { key: "base", header: t("aff_col_base"), align: "right", cell: (r) => formatRupiah(r.base_amount) },
    { key: "pct", header: t("aff_col_pct"), align: "right", cell: (r) => `${r.percentage}%` },
    { key: "amount", header: t("aff_col_amount"), align: "right", cell: (r) => formatRupiah(r.amount) },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "wait", header: t("aff_col_wait"), cell: (r) => formatAffDate(r.wait_until) },
  ];

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_commissions_title")}</h1>
        <select
          aria-label={t("aff_filter_status")}
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
          className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
        >
          <option value="">{t("aff_filter_all")}</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {affLabel(t, s)}
            </option>
          ))}
        </select>
      </div>
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={page}
        onPageChange={setPage}
        empty={t("aff_commissions_empty")}
      />
    </div>
  );
}
