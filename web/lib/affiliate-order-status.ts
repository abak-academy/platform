import type { I18nKey } from "@/lib/i18n";

// Statuses an affiliator order can show; carts never reach the lists.
export const AFFILIATE_ORDER_STATUS_LABEL: Record<string, I18nKey> = {
  payment_pending: "filter_pending",
  paid: "filter_paid",
  processing: "filter_processing",
  completed: "filter_completed",
  payment_expired: "filter_payment_expired",
  cancelled: "filter_cancelled",
};

export function affiliateOrderStatusLabel(status: string, t: (key: I18nKey) => string): string {
  const key = AFFILIATE_ORDER_STATUS_LABEL[status];
  return key ? t(key) : status;
}
