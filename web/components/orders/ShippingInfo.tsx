"use client";

import type { Order } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
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
  const address = [[addr.penerima, addr.telepon].filter(Boolean).join(" · "), addr.alamat, addr.kode_pos]
    .filter(Boolean)
    .join("\n");

  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" — ");
  const isEstimate = order.is_estimate ?? false;

  // Prototype "Dikirim ke": h3 + muted address + muted courier line.
  return (
    <section>
      <h3 className="font-[family-name:var(--font-display)] text-base leading-[1.35] font-bold tracking-[-0.02em] text-ink-900">
        {t("order_ship_to")}
      </h3>
      {address && <p className="mb-2.5 whitespace-pre-line text-ink-600">{address}</p>}
      <p className="mb-2.5 text-ink-600">
        <span>{courier || "—"}</span>
        {isEstimate && (
          <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
            {t("order_shipping_estimate_note" as any)}
          </span>
        )}
        {order.tracking_number && (
          <>
            {" · "}
            {t("order_shipment_status")}:{" "}
            <span>
              {order.shipment_status
                ? shipmentStatusLabel(order.shipment_status, lang)
                : t("order_shipment_status_unknown")}
            </span>
          </>
        )}
      </p>
      <ShipmentTimeline events={order.shipment_events} />
    </section>
  );
}
