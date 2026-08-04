"use client";

import type { Order } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { hasPhysicalItems } from "@/lib/shipping";
import { ShipmentTimeline } from "@/components/orders/ShipmentTimeline";
import { shipmentStatusLabel } from "@/lib/shipment-status";

export interface ShippingInfoProps {
  order: Order;
}

export function ShippingInfo({ order }: ShippingInfoProps) {
  const { t, lang } = useTranslation();

  const hasPhysical = hasPhysicalItems(order.items ?? []);
  if (!hasPhysical) return null;

  const addr = order.shipping_address ?? {};
  const addressLine = [addr.penerima, addr.telepon, addr.alamat, addr.kode_pos]
    .filter(Boolean)
    .join(" · ");

  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" — ");
  const isEstimate = order.is_estimate ?? false;

  return (
    <section className="rounded-lg border border-line bg-surface p-5">
      <h2 className="mb-4 font-serif text-lg font-semibold text-ink-900">
        {t("order_shipping_heading" as any)}
      </h2>
      <dl className="flex flex-col gap-2 text-sm">
        {addressLine && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_shipping_address" as any)}</dt>
            <dd className="text-right text-ink-900">{addressLine}</dd>
          </div>
        )}
        {courier && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_shipping_courier" as any)}</dt>
            <dd className="text-right text-ink-900">
              {courier}
              {isEstimate && (
                <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
                  {t("order_shipping_estimate_note" as any)}
                </span>
              )}
            </dd>
          </div>
        )}
        <div className="flex items-start justify-between gap-3">
          <dt className="text-ink-500">{t("order_shipping")}</dt>
          <dd className="text-right text-ink-900">{formatRupiah(order.shipping_cost ?? 0)}</dd>
        </div>
        {order.tracking_number && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_tracking")}</dt>
            <dd className="text-right text-ink-900">{order.tracking_number}</dd>
          </div>
        )}
        {order.tracking_number && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_shipment_status")}</dt>
            <dd className="text-right text-ink-900">
              {order.shipment_status
                ? shipmentStatusLabel(order.shipment_status, lang)
                : t("order_shipment_status_unknown")}
            </dd>
          </div>
        )}
      </dl>
      <ShipmentTimeline events={order.shipment_events} />
    </section>
  );
}
