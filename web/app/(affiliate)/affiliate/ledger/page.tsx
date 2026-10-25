"use client";

import { useState } from "react";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import type { DataTableColumn } from "@/components/ui/data-table";
import { useAffiliateLedger } from "@/lib/hooks/affiliate";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateLedgerEntry } from "@/lib/types";

export default function AffiliateLedgerPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const query = useAffiliateLedger({ page, order: "desc" });

  const columns: DataTableColumn<AffiliateLedgerEntry>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "type", header: t("aff_col_type"), cell: (r) => affLabel(t, r.entry_type) },
    {
      key: "amount",
      header: t("aff_col_amount"),
      align: "right",
      cell: (r) => (
        <span className={r.amount < 0 ? "text-danger" : "text-ink-900"}>{formatRupiah(r.amount)}</span>
      ),
    },
    { key: "note", header: t("aff_col_note"), cell: (r) => r.note ?? "—" },
  ];

  return (
    <div className="space-y-4">
      <div>
        <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_ledger_title")}</h1>
        {query.data && (
          <p className="mt-1 text-sm text-ink-600">
            {t("aff_ledger_balance").replace("{amount}", formatRupiah(query.data.saldo))}
          </p>
        )}
      </div>
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={page}
        onPageChange={setPage}
        empty={t("aff_ledger_empty")}
      />
    </div>
  );
}
