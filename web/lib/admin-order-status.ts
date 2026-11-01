import type { I18nKey } from "@/lib/i18n";
import type { Order } from "@/lib/types";
import { hasPhysicalItems } from "@/lib/shipping";
import { isShipmentFailure } from "@/lib/shipment-status";

export function adminOrderStatusLabel(order: Order, t: (key: I18nKey) => string): string | undefined {
  switch (order.status) {
    case "payment_pending":
      return t("filter_pending");
    case "paid":
    case "processing":
      return hasPhysicalItems(order.items ?? [])
        ? t("admin_home_ready_to_ship")
        : t(order.status === "paid" ? "filter_paid" : "filter_processing");
    case "shipped":
      return t(isShipmentFailure(order.shipment_status) ? "shipment_failed_badge" : "filter_in_transit");
    case "completed":
      return t("filter_completed");
    case "payment_expired":
      return t("filter_payment_expired");
    case "cancelled":
      return t("filter_cancelled");
    default:
      return undefined;
  }
}
