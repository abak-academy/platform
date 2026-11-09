"use client";

import { Suspense } from "react";
import Link from "next/link";
import { Download, ShoppingCart } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { RetryPaymentButton } from "@/components/partner/RetryPaymentButton";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { downloadCsv } from "@/lib/csv";
import { fetchAllPartnerPages, partnerKeys, usePartnerOrders } from "@/lib/hooks/partner";
import { useListParams } from "@/lib/hooks/use-list-params";
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
  return (
    <Suspense fallback={null}>
      <OrdersContent />
    </Suspense>
  );
}

function OrdersContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const filter = { q: params.q, status: params.status, from: params.from, to: params.to };
  const query = usePartnerOrders({ ...filter, page: params.page, per_page: params.per_page });

  const exportCsv = async () => {
    try {
      const all = await fetchAllPartnerPages<PartnerOrder>("/partners/me/orders", filter);
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
      <AdminPageHeader
        icon={ShoppingCart}
        title={t("partner_orders_title")}
        description={t("partner_orders_desc")}
        actions={
          <>
            <Button variant="outline" onClick={exportCsv}>
              <Download className="mr-2 size-4" />
              {t("partner_csv_download")}
            </Button>
            <Button asChild>
              <Link href="/affiliate/partner/orders/new">{t("partner_orders_new")}</Link>
            </Button>
          </>
        }
      />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("partner_orders_search")}
        filters={[
          {
            key: "status",
            label: t("partner_orders_filter_status_label"),
            options: Object.entries(FILTER_STATUSES).map(([value, key]) => ({ value, label: t(key) })),
          },
        ]}
        dateLabel={t("partner_orders_filter_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(o) => o.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("partner_orders_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
