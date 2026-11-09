"use client";

import { Copy, Ticket } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { usePartnerVouchers } from "@/lib/hooks/partner";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { PartnerVoucher } from "@/lib/types";

export default function PartnerVouchersPage() {
  const { t } = useTranslation();
  const query = usePartnerVouchers();

  const discount = (v: PartnerVoucher) =>
    v.discount_percent !== null ? `${v.discount_percent}%` : formatRupiah(v.discount_amount ?? 0);
  const remaining = (v: PartnerVoucher) => (v.max_uses === null ? t("partner_voucher_unlimited") : String(Math.max(0, v.max_uses - v.used_count)));

  const copy = async (code: string) => {
    try {
      await navigator.clipboard.writeText(code);
      toast.success(t("partner_copied"));
    } catch {
      toast.error(t("partner_copy_failed"));
    }
  };

  const columns: DataTableColumn<PartnerVoucher>[] = [
    {
      key: "code",
      header: t("partner_voucher_col_code"),
      cell: (v) => (
        <span className="flex items-center gap-2 font-mono font-semibold text-ink-900">
          {v.code}
          <Button type="button" size="icon-xs" variant="ghost" aria-label={t("partner_copy")} onClick={() => copy(v.code)}>
            <Copy className="size-3.5" />
          </Button>
        </span>
      ),
    },
    { key: "discount", header: t("partner_voucher_col_discount"), cell: discount },
    {
      key: "unit",
      header: t("partner_voucher_col_unit"),
      cell: (v) => t(v.partner?.quota_unit === "student" ? "partner_unit_student" : "partner_unit_order"),
    },
    { key: "remaining", header: t("partner_voucher_col_remaining"), cell: remaining },
    {
      key: "expires",
      header: t("partner_voucher_col_expires"),
      cell: (v) => (v.expires_at ? new Date(v.expires_at).toLocaleDateString("id-ID") : "-"),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Ticket} title={t("partner_vouchers_title")} description={t("partner_vouchers_desc")} />
      <p className="rounded-lg bg-surface-2 px-4 py-3 text-sm text-ink-600">{t("partner_vouchers_note")}</p>
      {query.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : query.isError ? (
        <div role="alert" className="space-y-2 text-sm text-danger">
          <p>{t("partner_load_failed")}</p>
          <Button size="sm" variant="outline" onClick={() => query.refetch()}>
            {t("partner_retry")}
          </Button>
        </div>
      ) : (
        <DataTable columns={columns} rows={query.data?.data ?? []} rowKey={(v) => v.id} empty={t("partner_vouchers_empty")} />
      )}
    </div>
  );
}
