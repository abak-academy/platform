"use client";

import Link from "next/link";
import { Download, ShoppingCart } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { RetryPaymentButton } from "@/components/partner/RetryPaymentButton";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { downloadCsv } from "@/lib/csv";
import { fetchAllPartnerPages, partnerKeys, usePartnerOrders } from "@/lib/hooks/partner";
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

const FILTER_STATUSES: Record<string, I18nKey> = {
  cart: "partner_order_status_cart",
  payment_pending: "partner_order_status_pending",
  paid: "partner_order_status_paid",
  payment_expired: "partner_order_status_expired",
  cancelled: "partner_order_status_cancelled",
};

export default function PartnerOrdersPage() {
  const { t } = useTranslation();
  const [status, setStatus] = useState("");
  const query = usePartnerOrders(status);
  const rows = query.data?.pages.flatMap((p) => p.data) ?? [];

  const exportCsv = async () => {
    try {
      const all = await fetchAllPartnerPages<PartnerOrder>("/partners/me/orders", status ? { status } : {});
      downloadCsv(
        "pembelian-mitra.csv",
        ["order_id", "created_at", "status", "products", "seats", "subtotal", "discount", "total"],
        all.map((o) => [
          o.id,
          o.created_at,
          o.status,
          o.items.map((i) => i.name).join("; "),
          o.items.reduce((n, i) => n + i.qty, 0),
          o.subtotal,
          o.discount,
          o.total,
        ]),
      );
    } catch {
      toast.error(t("partner_csv_failed"));
    }
  };

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
    {
      key: "detail",
      header: "",
      align: "right",
      cell: (o) => (
        <Button asChild size="sm" variant="ghost">
          <Link href={`/affiliate/partner/orders/${o.id}`}>{t("partner_orders_view")}</Link>
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={ShoppingCart} title={t("partner_orders_title")} description={t("partner_orders_desc")} />
      <div className="flex flex-wrap items-center gap-3">
        <select
          aria-label={t("partner_orders_col_status")}
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
        >
          <option value="">{t("partner_orders_filter_all")}</option>
          {Object.entries(FILTER_STATUSES).map(([value, key]) => (
            <option key={value} value={value}>
              {t(key)}
            </option>
          ))}
        </select>
        <div className="ml-auto flex gap-2">
          <Button variant="outline" onClick={exportCsv}>
            <Download className="mr-2 size-4" />
            {t("partner_csv_download")}
          </Button>
          <Button asChild>
            <Link href="/affiliate/partner/orders/new">{t("partner_orders_new")}</Link>
          </Button>
        </div>
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
        <>
          <DataTable columns={columns} rows={rows} rowKey={(o) => o.id} empty={t("partner_orders_empty")} />
          {query.hasNextPage && (
            <div className="flex justify-center">
              <Button variant="outline" onClick={() => query.fetchNextPage()} disabled={query.isFetchingNextPage}>
                {t("partner_load_more")}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
