"use client";

import { Copy, Ticket } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useAffiliatePromos } from "@/lib/hooks/affiliate";
import { useTranslation } from "@/lib/i18n";
import type { AffiliatePromo } from "@/lib/types";

export default function AffiliateVouchersPage() {
  const { t } = useTranslation();
  const query = useAffiliatePromos();

  const copy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success(t("aff_promos_copied"));
    } catch {
      toast.error(t("aff_error"));
    }
  };

  const none = t("aff_vouchers_none");
  const columns: DataTableColumn<AffiliatePromo>[] = [
    {
      key: "code",
      header: t("aff_vouchers_col_code"),
      cell: (p) => (
        <span className="flex items-center gap-2 font-mono font-bold tracking-wider text-ink-900">
          {p.code}
          <Button type="button" size="icon-xs" variant="ghost" aria-label={t("aff_promos_copy")} onClick={() => copy(p.code)}>
            <Copy className="size-3.5" />
          </Button>
        </span>
      ),
    },
    {
      key: "discount",
      header: t("aff_vouchers_col_discount"),
      cell: (p) => (p.discount_percent != null ? `${p.discount_percent}%` : formatRupiah(p.discount_amount ?? 0)),
    },
    { key: "cap", header: t("aff_vouchers_col_cap"), cell: (p) => (p.max_discount_amount ? formatRupiah(p.max_discount_amount) : none) },
    { key: "min", header: t("aff_vouchers_col_min"), cell: (p) => (p.min_order_amount ? formatRupiah(p.min_order_amount) : none) },
    {
      key: "remaining",
      header: t("aff_vouchers_col_remaining"),
      cell: (p) => (p.max_uses === null ? t("aff_vouchers_unlimited") : String(Math.max(0, p.max_uses - p.used_count))),
    },
    { key: "expires", header: t("aff_vouchers_col_expires"), cell: (p) => (p.expires_at ? formatAffDate(p.expires_at) : "-") },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Ticket} title={t("aff_vouchers_title")} description={t("aff_promos_desc")} />
      {query.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : query.isError ? (
        <div role="alert" className="space-y-2 text-sm text-danger">
          <p>{t("aff_error")}</p>
          <Button size="sm" variant="outline" onClick={() => query.refetch()}>
            {t("aff_retry")}
          </Button>
        </div>
      ) : (
        <DataTable columns={columns} rows={query.data?.data ?? []} rowKey={(p) => p.id} empty={t("aff_promos_empty")} />
      )}
    </div>
  );
}
