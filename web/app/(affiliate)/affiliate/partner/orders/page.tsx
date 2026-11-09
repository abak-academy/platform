"use client";

import Link from "next/link";
import { ShoppingCart } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { RetryPaymentButton } from "@/components/partner/RetryPaymentButton";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { partnerKeys, usePartnerOrders } from "@/lib/hooks/partner";
import { formatRupiah } from "@/lib/format";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type { PartnerOrder } from "@/lib/types";

const STATUS_KEYS: Record<string, I18nKey> = {
  cart: "partner_order_status_cart",
  payment_pending: "partner_order_status_pending",
  paid: "partner_order_status_paid",
  processing: "partner_order_status_paid",
  shipped: "partner_order_status_paid",
  completed: "partner_order_status_paid",
  payment_expired: "partner_order_status_expired",
  cancelled: "partner_order_status_cancelled",
};

export default function PartnerOrdersPage() {
  const { t } = useTranslation();
  const query = usePartnerOrders();

  const columns: DataTableColumn<PartnerOrder>[] = [
    { key: "date", header: t("partner_orders_col_date"), cell: (o) => new Date(o.created_at).toLocaleDateString("id-ID") },
    { key: "product", header: t("partner_orders_col_product"), cell: (o) => o.items.map((i) => i.name).join(", ") },
    { key: "seats", header: t("partner_orders_col_seats"), cell: (o) => String(o.items.reduce((n, i) => n + i.qty, 0)) },
    { key: "total", header: t("partner_orders_col_total"), align: "right", cell: (o) => formatRupiah(o.total) },
    { key: "status", header: t("partner_orders_col_status"), cell: (o) => t(STATUS_KEYS[o.status] ?? "partner_order_status_cart") },
    {
      key: "pay",
      header: "",
      align: "right",
      cell: (o) => {
        if (o.status === "cart") return <SnapCheckout orderId={o.id} basePath="/partners/me/orders" invalidateKey={partnerKeys.orders()} />;
        if (o.status === "payment_pending") return <RetryPaymentButton orderId={o.id} />;
        return null;
      },
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={ShoppingCart} title={t("partner_orders_title")} description={t("partner_orders_desc")} />
      <div className="flex justify-end">
        <Button asChild>
          <Link href="/affiliate/partner/orders/new">{t("partner_orders_new")}</Link>
        </Button>
      </div>
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
        <DataTable columns={columns} rows={query.data?.data ?? []} rowKey={(o) => o.id} empty={t("partner_orders_empty")} />
      )}
    </div>
  );
}
