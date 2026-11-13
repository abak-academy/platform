"use client";

import { use } from "react";
import Link from "next/link";
import { ArrowLeft, ShoppingCart } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { SnapCheckout } from "@/components/cart/SnapCheckout";
import { RetryPaymentButton } from "@/components/partner/RetryPaymentButton";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { partnerKeys, usePartnerOrder } from "@/lib/hooks/partner";
import { formatRupiah } from "@/lib/format";
import { useTranslation, type I18nKey } from "@/lib/i18n";

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

export default function PartnerOrderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { t } = useTranslation();
  const query = usePartnerOrder(id);
  const order = query.data;

  return (
    <div className="space-y-4">
      <Button asChild variant="ghost" size="sm">
        <Link href="/affiliate/partner/orders">
          <ArrowLeft className="mr-1 size-4" />
          {t("partner_order_back")}
        </Link>
      </Button>
      <AdminPageHeader icon={ShoppingCart} title={t("partner_order_detail_title")} description={order ? `#${order.id.slice(-8)}` : ""} />
      {query.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : query.isError || !order ? (
        <p role="alert" className="text-sm text-danger">
          {t("partner_order_load_failed")}
        </p>
      ) : (
        <>
          <section className="md-card-outlined space-y-3 p-5">
            <dl className="grid grid-cols-2 gap-2 text-sm">
              <dt className="text-ink-500">{t("partner_order_status")}</dt>
              <dd className="text-right font-medium">{t(STATUS_KEYS[order.status] ?? "partner_order_status_cart")}</dd>
              <dt className="text-ink-500">{t("partner_order_created")}</dt>
              <dd className="text-right">{new Date(order.created_at).toLocaleString("id-ID")}</dd>
              <dt className="text-ink-500">{t("partner_order_voucher")}</dt>
              <dd className="text-right font-mono">{order.promo_code ?? t("partner_order_no_voucher")}</dd>
            </dl>
            {order.status === "cart" && (
              <SnapCheckout orderId={order.id} basePath="/partners/me/orders" invalidateKey={partnerKeys.order(order.id)} />
            )}
            {order.status === "payment_pending" && (
              <div className="flex justify-end">
                <RetryPaymentButton orderId={order.id} />
              </div>
            )}
          </section>

          <section className="md-card-outlined space-y-2 p-5">
            <h2 className="font-semibold text-ink-900">{t("partner_order_products")}</h2>
            <ul className="divide-y divide-line text-sm">
              {order.items.map((i) => (
                <li key={i.product_id} className="flex items-center justify-between gap-3 py-2">
                  <span className="font-medium text-ink-900">{i.name}</span>
                  <span className="text-ink-600">
                    {t("partner_order_qty").replace("{n}", String(i.qty))} × {formatRupiah(i.unit_price)}
                  </span>
                </li>
              ))}
            </ul>
            <dl className="grid grid-cols-2 gap-2 border-t border-line pt-3 text-sm">
              <dt className="text-ink-500">{t("partner_new_subtotal")}</dt>
              <dd className="text-right">{formatRupiah(order.subtotal)}</dd>
              <dt className="text-ink-500">{t("partner_new_discount")}</dt>
              <dd className="text-right">- {formatRupiah(order.discount)}</dd>
              <dt className="font-semibold text-ink-900">{t("partner_new_total")}</dt>
              <dd className="text-right font-semibold text-ink-900">{formatRupiah(order.total)}</dd>
            </dl>
          </section>

          <section className="md-card-outlined space-y-2 p-5">
            <h2 className="font-semibold text-ink-900">{t("partner_order_students")}</h2>
            <ul className="divide-y divide-line text-sm">
              {(order.students ?? []).map((s) => (
                <li key={s.id} className="flex items-center justify-between gap-3 py-2">
                  <span className="font-medium text-ink-900">{s.name}</span>
                  <span className="text-ink-500">{s.school_name ?? "-"}</span>
                </li>
              ))}
            </ul>
          </section>
        </>
      )}
    </div>
  );
}
