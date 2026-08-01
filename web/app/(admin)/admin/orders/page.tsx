"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";
import { Receipt } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  useAdminOrders,
  useConfirmOrder,
  useShipOrder,
  useShipOrderManual,
  useCompleteOrder,
  useRefundOrder,
  useReconcileOrder,
} from "@/lib/hooks/admin-orders";
import { useTranslation } from "@/lib/i18n";
import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { OrderDetailModal } from "@/components/admin/OrderDetailModal";
import { ShipOrderModal } from "@/components/admin/ShipOrderModal";
import { ConfirmOrderModal } from "@/components/admin/ConfirmOrderModal";
import { formatRupiah } from "@/lib/format";
import type { Order, OrderStatus, AdminOrderFilterStatus } from "@/lib/types";

const FILTER_OPTIONS: AdminOrderFilterStatus[] = ["all", "pending", "paid", "processing", "shipped", "failed", "refunded"];

function orderNumber(order: Order): string {
  return `#${order.id.slice(-8)}`;
}

function buyerLabel(order: Order): string {
  return order.student_name?.trim() || order.student_id;
}

function productSummary(order: Order): string {
  if (!order.items || order.items.length === 0) return "-";
  const names = order.items.slice(0, 2).map((it) => it.name);
  const suffix = order.items.length > 2 ? ` +${order.items.length - 2}` : "";
  return names.join(", ") + suffix;
}

function hasPhysicalItem(order: Order): boolean {
  return (order.items ?? []).some((it) => it.product_type === "book" || it.product_type === "merchandise" || it.product_type === "medal");
}

function isShipped(order: Order): boolean {
  return Boolean(order.tracking_number || order.shipped_at);
}

function actionAllowed(status: OrderStatus, action: "confirm" | "ship" | "complete" | "refund" | "reconcile"): boolean {
  switch (action) {
    case "confirm":
      return status === "payment_pending";
    case "ship":
      return status === "paid" || status === "processing";
    case "complete":
      return status === "shipped" || status === "processing";
    case "refund":
      return status === "paid" || status === "processing" || status === "shipped" || status === "completed";
    case "reconcile":
      return status === "payment_pending";
  }
}

export default function OrdersPage() {
  const { t } = useTranslation();
  const [filter, setFilter] = useState<AdminOrderFilterStatus>("all");
  const { data: orders, isLoading, isError, error } = useAdminOrders(filter);
  const confirm = useConfirmOrder();
  const ship = useShipOrder();
  const shipManual = useShipOrderManual();
  const complete = useCompleteOrder();
  const refund = useRefundOrder();
  const reconcile = useReconcileOrder();
  const [detailOrder, setDetailOrder] = useState<Order | null>(null);
  const [shippingOrder, setShippingOrder] = useState<Order | null>(null);
  const [shipError, setShipError] = useState<string | null>(null);
  const [confirmingOrder, setConfirmingOrder] = useState<Order | null>(null);
  const [confirmError, setConfirmError] = useState<string | null>(null);

  const filtered = useMemo(() => {
    if (!orders) return [];
    return orders;
  }, [orders]);

  const filterLabel = (f: AdminOrderFilterStatus): string => {
    switch (f) {
      case "all": return t("tab_all");
      case "pending": return t("filter_pending");
      case "paid": return t("filter_paid");
      case "processing": return "Diproses";
      case "shipped": return "Dikirim";
      case "failed": return t("filter_failed");
      case "refunded": return t("filter_refunded");
    }
  };

  function shippingBadge(order: Order) {
    if (!hasPhysicalItem(order)) return null;
    if (isShipped(order)) {
      return <Badge className="bg-green-100 text-green-800 border-green-200">{t("status_shipped")}</Badge>;
    }
    return <Badge variant="outline">{t("status_pending_ship")}</Badge>;
  }

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

  async function handleShipBook(id: string) {
    setShipError(null);
    try {
      await ship.mutateAsync(id);
      setShippingOrder(null);
      toast.success(t("orders_shipped"));
    } catch (e) {
      setShipError(errorMessage(e));
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
    if (!window.confirm("Tandai pesanan ini sebagai selesai?")) return;
    try {
      await complete.mutateAsync(id);
      toast.success("Pesanan selesai");
    } catch (e) {
      toast.error(errorMessage(e));
    }
  }

  async function handleRefund(id: string) {
    if (!window.confirm(t("orders_refund_prompt"))) return;
    try {
      await refund.mutateAsync(id);
      toast.success(t("orders_refunded"));
    } catch (e) {
      toast.error(errorMessage(e));
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

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Receipt}
        title={t("admin_orders_page_title")}
        description={t("admin_orders_page_description")}
      />

      <div className="flex flex-wrap gap-2">
        {FILTER_OPTIONS.map((f) => (
          <button
            key={f}
            className={filter === f ? "md-btn-filled" : "md-btn-outlined"}
            onClick={() => setFilter(f)}
          >
            {filterLabel(f)}
          </button>
        ))}
      </div>

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
        <div className="overflow-x-auto md-card-outlined">
          <table className="w-full text-sm">
            <thead className="bg-muted">
              <tr>
                <th className="px-4 py-3 text-left font-medium">{t("orders")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_buyer")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_product")}</th>
                <th className="px-4 py-3 text-left font-medium">{t("th_total")}</th>
                <th className="px-4 py-3 text-left font-medium">Status</th>
                <th className="px-4 py-3 text-left font-medium">Pengiriman</th>
                <th className="px-4 py-3 text-right font-medium">{t("th_actions")}</th>
              </tr>
            </thead>
            <tbody>
              {filtered.map((order) => (
                <tr
                  key={order.id}
                  role="button"
                  tabIndex={0}
                  aria-label={`${t("orders_detail_open")} ${orderNumber(order)}`}
                  onClick={() => setDetailOrder(order)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      setDetailOrder(order);
                    }
                  }}
                  className="cursor-pointer border-t transition-colors hover:bg-muted/40 focus-visible:bg-muted/40 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
                >
                  <td className="px-4 py-3 font-mono font-medium">{orderNumber(order)}</td>
                  <td className="px-4 py-3">
                    <div className="font-medium">{buyerLabel(order)}</div>
                    <div className="font-mono text-[10px] text-muted-foreground">{order.student_id}</div>
                  </td>
                  <td className="px-4 py-3 max-w-xs truncate">{productSummary(order)}</td>
                  <td className="px-4 py-3">{formatRupiah(order.total)}</td>
                  <td className="px-4 py-3">
                    <OrderStatusBadge status={order.status} />
                  </td>
                  <td className="px-4 py-3">
                    {hasPhysicalItem(order) ? shippingBadge(order) : <span className="text-xs text-muted-foreground">—</span>}
                  </td>
                  <td className="px-4 py-3 text-right">
                    <div
                      className="flex items-center justify-end gap-2"
                      onClick={(e) => e.stopPropagation()}
                    >
                      {actionAllowed(order.status, "confirm") && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setConfirmError(null);
                            setConfirmingOrder(order);
                          }}
                          disabled={confirm.isPending}
                        >
                          {t("action_confirm")}
                        </Button>
                      )}
                      {actionAllowed(order.status, "ship") && hasPhysicalItem(order) && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setShipError(null);
                            setShippingOrder(order);
                          }}
                          disabled={ship.isPending}
                        >
                          {t("action_ship")}
                        </Button>
                      )}
                      {actionAllowed(order.status, "complete") && (order.status === "shipped" || !hasPhysicalItem(order)) && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleComplete(order.id)}
                          disabled={complete.isPending}
                        >
                          Selesai
                        </Button>
                      )}
                      {actionAllowed(order.status, "refund") && (
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => handleRefund(order.id)}
                          disabled={refund.isPending}
                        >
                          {t("action_refund")}
                        </Button>
                      )}
                      {actionAllowed(order.status, "reconcile") && (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => handleReconcile(order.id)}
                          disabled={reconcile.isPending}
                        >
                          {t("action_reconcile")}
                        </Button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
              {filtered.length === 0 && (
                <tr>
                  <td colSpan={7} className="px-4 py-8 text-center text-muted-foreground">
                    {t("empty_orders")}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      <OrderDetailModal order={detailOrder} onOpenChange={() => setDetailOrder(null)} />

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

      {shippingOrder && (
        <ShipOrderModal
          open
          onOpenChange={() => {
            setShippingOrder(null);
            setShipError(null);
          }}
          orderNumber={orderNumber(shippingOrder)}
          onBook={() => handleShipBook(shippingOrder.id)}
          onSubmitManual={(trackingNumber) => handleShipManual(shippingOrder.id, trackingNumber)}
          isPending={ship.isPending || shipManual.isPending}
          error={shipError}
        />
      )}
    </div>
  );
}
