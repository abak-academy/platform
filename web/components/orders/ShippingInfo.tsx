"use client";

import type { Order } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { hasPhysicalItems } from "@/lib/shipping";

// Mirrors FallbackCourier in backend/internal/service/shipping_rates.go. The
// order row does not carry an is_estimate flag, so the badge has to be inferred
// from the stored courier name. "Flat" is the pre-rename label and is kept so
// orders placed under it keep their badge. Both entries disappear once the flag
// is persisted — see docs/backlog/shipping-estimate-flag.md.
const ESTIMATE_COURIERS = new Set(["Ongkir Flat", "Flat"]);

export interface ShippingInfoProps {
  order: Order;
}

export function ShippingInfo({ order }: ShippingInfoProps) {
  const { t } = useTranslation();

  const hasPhysical = hasPhysicalItems(order.items ?? []);
  if (!hasPhysical) return null;

  const addr = order.shipping_address ?? {};
  const addressLine = [addr.penerima, addr.telepon, addr.alamat, addr.kode_pos]
    .filter(Boolean)
    .join(" · ");

  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" — ");
  const isEstimate = ESTIMATE_COURIERS.has(order.selected_courier ?? "");

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
      </dl>
    </section>
  );
}
