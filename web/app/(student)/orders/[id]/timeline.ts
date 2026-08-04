import type { Order } from "@/lib/types";

// Lives beside the page rather than inside it: Next validates page.tsx's
// exports and rejects anything that is not one of its own fields, so a helper
// cannot sit there just because a test wants to import it.

export interface TimelineStep {
  key: string;
  label: string;
  at?: string;
  reached: boolean;
  cancelled?: boolean;
}

export function buildTimeline(o: Order, t: (key: any) => string): TimelineStep[] {
  const cancelled = o.status === "cancelled";
  return [
    {
      key: "created",
      label: t("order_tl_created"),
      at: o.created_at,
      reached: Boolean(o.created_at),
    },
    {
      key: "checkout",
      label: t("order_tl_checkout"),
      at: o.checked_out_at,
      reached: Boolean(o.checked_out_at),
    },
    {
      key: "paid",
      label: t("order_tl_paid"),
      at: o.paid_at,
      reached: Boolean(o.paid_at),
    },
    {
      key: "shipped",
      label: t("order_tl_shipped"),
      at: o.shipped_at,
      reached: Boolean(o.shipped_at),
    },
    {
      key: "completed",
      label: t("order_tl_completed"),
      at: o.completed_at,
      reached: Boolean(o.completed_at),
    },
    {
      key: "cancelled",
      label: t("order_tl_cancelled"),
      at: o.cancelled_at,
      reached: cancelled,
      cancelled,
    },
    // Only what actually happened. The old filter always kept completed and
    // cancelled, so a finished order still listed "Order cancelled" in grey —
    // a step that can never be reached now, presented as though pending.
  ].filter((s) => s.reached);
}
