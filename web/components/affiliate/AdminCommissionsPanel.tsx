"use client";

import { useState } from "react";
import { Download } from "lucide-react";
import { toast } from "sonner";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { downloadAffiliateCsv, useAdminAffiliateCommissions } from "@/lib/hooks/admin-affiliates";
import type { ListParams } from "@/lib/hooks/use-list-params";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { formatCommissionRule } from "@/lib/affiliate-rules";
import type { AffiliateCommission } from "@/lib/types";

const STATUSES = ["pending", "available", "cancelled"] as const;
const CATEGORIES = ["exam", "course", "book", "merchandise", "medal"] as const;

interface AdminCommissionsPanelProps {
  affiliatorId: string;
  // The report's period; a panel without one lists every commission.
  from?: string;
  to?: string;
}

const INITIAL: ListParams = { q: "", status: "", type: "", from: "", to: "", page: 1, per_page: 10 };

// The orders behind one affiliator's commissions. Its filters are local to the
// panel (the page's own URL state belongs to the list around it).
export function AdminCommissionsPanel({ affiliatorId, from = "", to = "" }: AdminCommissionsPanelProps) {
  const { t } = useTranslation();
  const [params, setParams] = useState<ListParams>(INITIAL);
  const filter = { q: params.q, status: params.status, type: params.type, from, to };
  const query = useAdminAffiliateCommissions(affiliatorId, { ...filter, page: params.page, per_page: params.per_page });
  const hasFilters = Boolean(params.q || params.status || params.type);

  const update = (patch: Partial<ListParams>) =>
    setParams((current) => ({ ...current, ...patch, page: patch.page ?? 1 }));

  const exportCsv = async () => {
    try {
      await downloadAffiliateCsv("commissions", filter, affiliatorId);
    } catch {
      toast.error(t("admin_aff_export_failed"));
    }
  };

  const columns: DataTableColumn<AffiliateCommission>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.paid_at ?? r.created_at) },
    { key: "order", header: t("admin_aff_col_order"), cell: (r) => <code className="text-xs">{r.order_id.slice(0, 8)}</code> },
    { key: "student", header: t("aff_col_student"), cell: (r) => <span className="font-medium text-ink-900">{r.student_name ?? "—"}</span> },
    { key: "product", header: t("aff_col_product_name"), cell: (r) => r.product_name ?? "—" },
    { key: "category", header: t("aff_col_product"), cell: (r) => affLabel(t, r.product_type) },
    { key: "base", header: t("aff_col_purchase"), align: "right", cell: (r) => formatRupiah(r.base_amount) },
    { key: "pct", header: t("aff_col_rate"), align: "right", cell: (r) => formatCommissionRule(r.commission_type, r.commission_value, t("aff_per_unit")) },
    { key: "amount", header: t("aff_col_commission"), align: "right", cell: (r) => <span className="font-semibold">{formatRupiah(r.amount)}</span> },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
  ];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h3 className="font-semibold text-ink-900">{t("admin_aff_rep_orders")}</h3>
        <Button variant="outline" size="sm" onClick={exportCsv}>
          <Download className="size-4" aria-hidden />
          {t("admin_aff_export_csv")}
        </Button>
      </div>
      <ListToolbar
        params={params}
        onChange={update}
        onReset={() => setParams((current) => ({ ...INITIAL, per_page: current.per_page }))}
        searchPlaceholder={t("aff_comm_search")}
        filters={[
          { key: "status", label: t("aff_col_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "type", label: t("aff_filter_category"), options: CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_rep_orders_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
