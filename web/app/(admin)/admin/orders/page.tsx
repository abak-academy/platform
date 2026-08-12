"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { Receipt } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminOrders,
  useAdminOrderSummary,
  useConfirmOrder,
  useShipOrder,
  useShipOrderManual,
  useCompleteOrder,
  useRefundOrder,
  useReconcileOrder,
  useRefreshShipment,
  useCancelShipment,
  useOrderTracking,
} from "@/lib/hooks/admin-orders";
import { useTranslation } from "@/lib/i18n";
import { isShipmentFailure } from "@/lib/shipment-status";
import { CancelShipmentModal } from "@/components/admin/CancelShipmentModal";
import { TrackingModal } from "@/components/admin/TrackingModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { OrderDetailModal } from "@/components/admin/OrderDetailModal";
import { OrderRow, type OrderRowMenuAction, type OrderRowPrimaryAction } from "@/components/admin/OrderRow";
import { OrdersToolbar } from "@/components/admin/OrdersToolbar";
import { ShipOrderModal } from "@/components/admin/ShipOrderModal";
import { ConfirmOrderModal } from "@/components/admin/ConfirmOrderModal";
import { RefundOrderModal } from "@/components/admin/RefundOrderModal";
import type { Order, OrderStatus, AdminOrderQuery, AdminOrderFilterStatus, AdminOrderQueue } from "@/lib/types";

// Mirrors AdminOrderFilterStatus. An unknown ?status= falls back to "all"
// rather than sending a value the API would reject.
const ORDER_FILTER_STATUSES: AdminOrderFilterStatus[] = [
  "all", "pending", "paid", "processing", "shipped", "failed", "cancelled",
];

// Mirrors AdminOrderQueue. Also doubles as the backward-compat list: prod has
// shipped `?status=ready_to_ship` / `?status=shipment_failed` links since
// before the queue split, so a legacy ?status= carrying one of these is
// treated as the equivalent ?queue= rather than breaking the bookmark.
const ORDER_QUEUES: AdminOrderQueue[] = ["ready_to_ship", "shipment_failed"];

function orderNumber(order: Order): string {
  return `#${order.id.slice(-8)}`;
}

function hasPhysicalItem(order: Order): boolean {
  return (order.items ?? []).some((it) => it.product_type === "book" || it.product_type === "merchandise" || it.product_type === "medal");
}

function actionAllowed(status: OrderStatus, action: "confirm" | "ship" | "complete" | "refund" | "reconcile"): boolean {
  switch (action) {
    case "confirm":
      return status === "payment_pending";
    case "ship":
      return status === "paid" || status === "processing";
    case "complete":
      return status === "shipped" || status === "processing";
    // Not `completed`: the order is finished and the goods are with the buyer,
    // so a refund there is a returns case, not a routine order action. Offering
    // it on every historical row buried the states where it is actually the
    // right move. A dead shipment re-opens it — see refundAllowed below.
    case "refund":
      return status === "paid" || status === "processing" || status === "shipped";
    case "reconcile":
      return status === "payment_pending";
  }
}

// A parcel the courier never accepted is money taken for goods that will not
// arrive, whatever orders.status says — the webhook never walks the status
// back. That is the one case where a completed order still deserves a refund.
function refundAllowed(order: Order): boolean {
  return actionAllowed(order.status, "refund") || isShipmentFailure(order.shipment_status);
}

// A cancelled or rejected booking leaves orders.status at "shipped" — the
// webhook never walks it back — so the status alone says the parcel is on its
// way when nothing is. Gating Ship on status left those orders with no exit but
// a refund.
//
// Mirrors the backend's shippable() exactly, including the `=== "shipped"`
// clause. refundAllowed re-opens on a failed shipment from ANY status, and
// copying that shape here would offer Ship on a completed or cancelled order
// with a dead parcel — states the backend refuses with order_not_shippable, so
// the button would be there and never work.
function shipAllowed(order: Order): boolean {
  return (
    actionAllowed(order.status, "ship") ||
    (order.status === "shipped" && isShipmentFailure(order.shipment_status))
  );
}

export default function OrdersPage() {
  const { t } = useTranslation();
  const [query, setQuery] = useState<AdminOrderQuery>({ status: "all" });

  // Read ?queue=/?status= once on mount so the store dashboard's queue cards
  // can deep link into their own queue. Deliberately an effect rather than a
  // lazy useState initializer: this is a client component but Next still
  // renders it on the server, where the URL is not available, and seeding
  // from window.location during render would be a hydration mismatch.
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const rawQueue = params.get("queue");
    if (rawQueue && ORDER_QUEUES.includes(rawQueue as AdminOrderQueue)) {
      setQuery((q) => ({ ...q, status: "all", queue: rawQueue as AdminOrderQueue }));
      return;
    }
    const rawStatus = params.get("status");
    // Legacy link: pre-split builds/bookmarks used ?status= for the two
    // queue values too.
    if (rawStatus && ORDER_QUEUES.includes(rawStatus as AdminOrderQueue)) {
      setQuery((q) => ({ ...q, status: "all", queue: rawStatus as AdminOrderQueue }));
      return;
    }
    // Legacy link: the tab that filters `cancelled` was labelled and keyed
    // "refunded" until it was renamed to match what it actually returns.
    if (rawStatus === "refunded") {
      setQuery((q) => ({ ...q, status: "cancelled" }));
      return;
    }
    if (rawStatus && ORDER_FILTER_STATUSES.includes(rawStatus as AdminOrderFilterStatus)) {
      setQuery((q) => ({ ...q, status: rawStatus as AdminOrderFilterStatus }));
    }
  }, []);
  const {
    data,
    isLoading,
    isError,
    error,
    hasNextPage,
    isFetchingNextPage,
    fetchNextPage,
  } = useAdminOrders(query);
  const { data: summary } = useAdminOrderSummary(query);
  const confirm = useConfirmOrder();
  const ship = useShipOrder();
  const shipManual = useShipOrderManual();
  const complete = useCompleteOrder();
  const refund = useRefundOrder();
  const reconcile = useReconcileOrder();
  const refreshShipment = useRefreshShipment();
  const cancelShipment = useCancelShipment();
  const [trackingOrderId, setTrackingOrderId] = useState<string | null>(null);
  const tracking = useOrderTracking(trackingOrderId);
  const [detailOrder, setDetailOrder] = useState<Order | null>(null);
  const [shippingOrder, setShippingOrder] = useState<Order | null>(null);
  const [cancellingShipment, setCancellingShipment] = useState<Order | null>(null);
  const [cancelShipmentError, setCancelShipmentError] = useState<string | null>(null);
  const [shipError, setShipError] = useState<string | null>(null);
  const [confirmingOrder, setConfirmingOrder] = useState<Order | null>(null);
  const [confirmError, setConfirmError] = useState<string | null>(null);
  const [refundingOrder, setRefundingOrder] = useState<Order | null>(null);
  const [refundError, setRefundError] = useState<string | null>(null);
  const [completingOrder, setCompletingOrder] = useState<Order | null>(null);

  const orders = data?.pages.flatMap((p) => p.data) ?? [];
  const total = summary?.buckets.total;

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  async function handleConfirmSubmit(id: string, paymentProofUrl: string) {
    setConfirmError(null);
    try {
      await confirm.mutateAsync({ id, paymentProofUrl });
      setConfirmingOrder(null);
      toast.success(t("orders_confirm"));
    } catch (e) {
      setConfirmError(errorMessage(e));
    }
  }

  async function handleShipBook(
    id: string,
    schedule?: { deliveryDate: string; deliveryTime: string },
  ) {
    setShipError(null);
    try {
      await ship.mutateAsync(schedule ? { id, ...schedule } : id);
      setShippingOrder(null);
      toast.success(t("orders_shipped"));
    } catch (e) {
      setShipError(errorMessage(e));
    }
  }

  async function handleRefreshShipment(id: string) {
    try {
      await refreshShipment.mutateAsync(id);
      toast.success(t("shipment_refreshed"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleCancelShipment(id: string, reason: string) {
    setCancelShipmentError(null);
    try {
      await cancelShipment.mutateAsync({ id, reason });
      setCancellingShipment(null);
      toast.success(t("shipment_cancelled"));
    } catch (e) {
      setCancelShipmentError(errorMessage(e));
    }
  }

  async function handleShipManual(id: string, trackingNumber: string) {
    setShipError(null);
    try {
      await shipManual.mutateAsync({ id, trackingNumber });
      setShippingOrder(null);
      toast.success(t("orders_shipped"));
    } catch (e) {
      setShipError(errorMessage(e));
    }
  }

  async function handleComplete(id: string) {
    try {
      await complete.mutateAsync(id);
      setCompletingOrder(null);
      toast.success(t("toast_order_completed"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  // No window.confirm: a refund needs a transfer receipt, not a yes/no, so
  // both entry points hand off to RefundOrderModal the same way the detail
  // view already hands off to ShipOrderModal. The modal also states plainly
  // that money and stock are not returned automatically — see issue #72.
  async function handleRefundSubmit(id: string, refundProofUrl: string) {
    setRefundError(null);
    try {
      await refund.mutateAsync({ id, refundProofUrl });
      setRefundingOrder(null);
      toast.success(t("orders_refunded"));
    } catch (e) {
      setRefundError(errorMessage(e));
    }
  }

  async function handleReconcile(id: string) {
    try {
      await reconcile.mutateAsync(id);
      toast.success(t("orders_reconciled"));
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  // One action gets a button; the rest go behind the menu. The order of the
  // chain is the order the admin walks the pesanan through.
  function primaryAction(order: Order): OrderRowPrimaryAction | undefined {
    if (actionAllowed(order.status, "confirm")) {
      return {
        label: t("action_confirm"),
        onClick: () => {
          setConfirmError(null);
          setConfirmingOrder(order);
        },
        disabled: confirm.isPending,
      };
    }
    if (shipAllowed(order) && hasPhysicalItem(order)) {
      return {
        label: t("action_ship"),
        onClick: () => {
          setShipError(null);
          setShippingOrder(order);
        },
        disabled: ship.isPending,
      };
    }
    if (actionAllowed(order.status, "complete") && (order.status === "shipped" || !hasPhysicalItem(order))) {
      return {
        label: t("action_complete"),
        onClick: () => setCompletingOrder(order),
        disabled: complete.isPending,
      };
    }
    return undefined;
  }

  function menuActions(order: Order): OrderRowMenuAction[] {
    const actions: OrderRowMenuAction[] = [];
    if (order.tracking_number) {
      actions.push({ label: t("shipment_track_button"), onClick: () => setTrackingOrderId(order.id) });
    }
    if (order.biteship_order_id) {
      // Disabled while in flight: these two fire a mutation straight from the
      // menu rather than opening a modal, and they greyed out as row buttons.
      // Refresh has no server-side idempotency guard.
      actions.push({
        label: t("shipment_refresh"),
        disabled: refreshShipment.isPending,
        onClick: () => handleRefreshShipment(order.id),
      });
      actions.push({
        label: t("shipment_cancel"),
        onClick: () => {
          setCancelShipmentError(null);
          setCancellingShipment(order);
        },
      });
    }
    if (actionAllowed(order.status, "reconcile")) {
      actions.push({
        label: t("action_reconcile"),
        disabled: reconcile.isPending,
        onClick: () => handleReconcile(order.id),
      });
    }
    if (refundAllowed(order)) {
      actions.push({
        label: t("action_refund"),
        destructive: true,
        onClick: () => {
          setRefundError(null);
          setRefundingOrder(order);
        },
      });
    }
    return actions;
  }

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Receipt}
        title={t("admin_orders_page_title")}
        description={t("admin_orders_page_description")}
      />

      <OrdersToolbar value={query} onChange={setQuery} counts={summary?.buckets} />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("orders_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="space-y-3">
          <div className="overflow-x-auto md-card-outlined">
            <table className="w-full text-sm">
              <thead className="bg-muted">
                <tr>
                  <th className="px-4 py-3.5 text-left text-[13px] font-semibold tracking-wide text-ink-600 uppercase">{t("orders")}</th>
                  <th className="hidden px-4 py-3.5 text-left text-[13px] font-semibold tracking-wide text-ink-600 uppercase md:table-cell">{t("th_buyer")}</th>
                  <th className="hidden px-4 py-3.5 text-left text-[13px] font-semibold tracking-wide text-ink-600 uppercase md:table-cell">{t("th_product")}</th>
                  <th className="hidden px-4 py-3.5 text-right text-[13px] font-semibold tracking-wide text-ink-600 uppercase md:table-cell">{t("th_total")}</th>
                  <th className="px-4 py-3.5 text-left text-[13px] font-semibold tracking-wide text-ink-600 uppercase">{t("th_status")}</th>
                  <th className="px-4 py-3.5 text-right text-[13px] font-semibold tracking-wide text-ink-600 uppercase">{t("th_actions")}</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((order) => (
                  <OrderRow
                    key={order.id}
                    order={order}
                    onOpen={() => setDetailOrder(order)}
                    onTrack={order.tracking_number ? () => setTrackingOrderId(order.id) : undefined}
                    primaryAction={primaryAction(order)}
                    menuActions={menuActions(order)}
                  />
                ))}
                {orders.length === 0 && (
                  <tr>
                    <td colSpan={6} className="px-4 py-8 text-center text-muted-foreground">
                      {t("empty_orders")}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="flex items-center justify-between gap-4 px-1">
            <p className="text-xs tabular-nums text-ink-600">
              {t("orders_showing")} {orders.length}
              {total !== undefined && ` ${t("orders_of_total")} ${total}`}
            </p>
            {hasNextPage && (
              <Button
                variant="outline"
                size="sm"
                onClick={() => fetchNextPage()}
                disabled={isFetchingNextPage}
              >
                {t("orders_load_more")}
              </Button>
            )}
          </div>
        </div>
      )}

      <OrderDetailModal
        order={detailOrder}
        onOpenChange={() => setDetailOrder(null)}
        onShip={
          detailOrder && shipAllowed(detailOrder) && hasPhysicalItem(detailOrder)
            ? () => {
                const order = detailOrder;
                setDetailOrder(null);
                setShipError(null);
                setShippingOrder(order);
              }
            : undefined
        }
        onTrack={
          detailOrder?.tracking_number ? () => setTrackingOrderId(detailOrder.id) : undefined
        }
        onRefreshShipment={
          detailOrder?.biteship_order_id
            ? () => handleRefreshShipment(detailOrder.id)
            : undefined
        }
        isRefreshingShipment={refreshShipment.isPending}
        onCancelShipment={
          detailOrder?.biteship_order_id
            ? () => {
                const order = detailOrder;
                setDetailOrder(null);
                setCancelShipmentError(null);
                setCancellingShipment(order);
              }
            : undefined
        }
        onRefund={
          detailOrder && actionAllowed(detailOrder.status, "refund")
            ? () => {
                // Same hand-off as onShip above: detailOrder is a snapshot that
                // could not show the new status anyway, and the refund cannot
                // proceed until a transfer receipt has been uploaded.
                const order = detailOrder;
                setDetailOrder(null);
                setRefundError(null);
                setRefundingOrder(order);
              }
            : undefined
        }
        isRefunding={refund.isPending}
      />

      {confirmingOrder && (
        <ConfirmOrderModal
          open
          onOpenChange={() => {
            setConfirmingOrder(null);
            setConfirmError(null);
          }}
          orderNumber={orderNumber(confirmingOrder)}
          onConfirm={(paymentProofUrl) => handleConfirmSubmit(confirmingOrder.id, paymentProofUrl)}
          isPending={confirm.isPending}
          error={confirmError}
        />
      )}

      {completingOrder && (
        <Dialog
          open
          onOpenChange={(o) => {
            if (!o) setCompletingOrder(null);
          }}
        >
          <DialogContent className="sm:max-w-md">
            <DialogHeader>
              <DialogTitle>{t("confirm_complete_title")}</DialogTitle>
              <DialogDescription>{t("confirm_complete_desc")}</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setCompletingOrder(null)}
                disabled={complete.isPending}
              >
                {t("cancel")}
              </Button>
              <Button onClick={() => handleComplete(completingOrder.id)} disabled={complete.isPending}>
                {t("action_complete")}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}

      {refundingOrder && (
        <RefundOrderModal
          open
          onOpenChange={() => {
            setRefundingOrder(null);
            setRefundError(null);
          }}
          orderNumber={orderNumber(refundingOrder)}
          onRefund={(refundProofUrl) => handleRefundSubmit(refundingOrder.id, refundProofUrl)}
          isPending={refund.isPending}
          error={refundError}
        />
      )}

      <TrackingModal
        open={Boolean(trackingOrderId)}
        onOpenChange={(o) => {
          if (!o) setTrackingOrderId(null);
        }}
        tracking={tracking.data}
        isLoading={tracking.isLoading}
        error={tracking.isError ? t("shipment_track_failed") : null}
      />

      {cancellingShipment && (
        <CancelShipmentModal
          open
          onOpenChange={(o) => {
            if (!o) setCancellingShipment(null);
          }}
          orderNumber={orderNumber(cancellingShipment)}
          onCancelShipment={(reason) => handleCancelShipment(cancellingShipment.id, reason)}
          isPending={cancelShipment.isPending}
          error={cancelShipmentError}
        />
      )}

      {shippingOrder && (
        <ShipOrderModal
          open
          onOpenChange={() => {
            setShippingOrder(null);
            setShipError(null);
          }}
          orderNumber={orderNumber(shippingOrder)}
          onBook={(schedule) => handleShipBook(shippingOrder.id, schedule)}
          onSubmitManual={(trackingNumber) => handleShipManual(shippingOrder.id, trackingNumber)}
          isPending={ship.isPending || shipManual.isPending}
          error={shipError}
        />
      )}
    </div>
  );
}
