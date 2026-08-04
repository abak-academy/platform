"use client";

import { MoreHorizontal, Package, PackageX } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { OrderStatusBadge } from "@/components/orders/OrderStatusBadge";
import { isShipmentFailure, shipmentStatusLabel } from "@/lib/shipment-status";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { Order, OrderStatus } from "@/lib/types";

export const STALE_AFTER_DAYS = 3;

const DAY_MS = 86_400_000;

/** A finished order is not stale however old — reddening one teaches admins to ignore the colour. */
const TERMINAL_STATUSES: readonly OrderStatus[] = ["completed", "cancelled", "payment_expired"];

/** States where nothing moves until an admin acts. */
const AWAITING_ACTION_STATUSES: readonly OrderStatus[] = ["payment_pending", "paid", "processing"];

const dateFormatter = new Intl.DateTimeFormat("id-ID", { day: "numeric", month: "short" });

export interface OrderRowMenuAction {
  label: string;
  onClick: () => void;
  destructive?: boolean;
  /**
   * These actions were row buttons that greyed out while their mutation was in
   * flight. Refund and refresh-shipment have no server-side idempotency guard,
   * so losing that would make a double-click a second refund.
   */
  disabled?: boolean;
}

export interface OrderRowPrimaryAction {
  label: string;
  onClick: () => void;
  disabled?: boolean;
}

export interface OrderRowProps {
  order: Order;
  onOpen: () => void;
  onTrack?: () => void;
  primaryAction?: OrderRowPrimaryAction;
  menuActions: OrderRowMenuAction[];
}

function orderNumber(order: Order): string {
  return `#${order.id.slice(-8)}`;
}

function buyerLabel(order: Order): string {
  return order.student_name?.trim() || order.student_id;
}

function ageLabel(ms: number): string {
  if (ms < 3_600_000) return "<1 jam";
  if (ms < DAY_MS) return `${Math.floor(ms / 3_600_000)} jam`;
  return `${Math.floor(ms / DAY_MS)} hari`;
}

function isStale(order: Order, ageMs: number): boolean {
  if (TERMINAL_STATUSES.includes(order.status)) return false;
  if (ageMs < STALE_AFTER_DAYS * DAY_MS) return false;
  return (
    AWAITING_ACTION_STATUSES.includes(order.status) || isShipmentFailure(order.shipment_status)
  );
}

export function OrderRow({ order, onOpen, onTrack, primaryAction, menuActions }: OrderRowProps) {
  const { t, lang } = useTranslation();

  const createdAt = order.created_at ? Date.parse(order.created_at) : NaN;
  const ageMs = Number.isNaN(createdAt) ? 0 : Math.max(0, Date.now() - createdAt);
  const hasCreatedAt = !Number.isNaN(createdAt);
  const stale = hasCreatedAt && isStale(order, ageMs);

  const items = order.items ?? [];
  const extraItems = Math.max(0, items.length - 1);

  const failedShipment = isShipmentFailure(order.shipment_status);
  const courierLabel = order.shipment_status
    ? shipmentStatusLabel(order.shipment_status, lang)
    : null;

  function courierSubLine() {
    // A courier-not-found parcel has no waybill to query, so it is deliberately
    // inert — same icon slot, same size, no underline and no pointer.
    if (failedShipment) {
      return (
        <span
          data-testid="row-shipment-status"
          className="mt-1 inline-flex items-center gap-1 text-[11px] font-semibold text-destructive"
        >
          <PackageX className="size-3" aria-hidden />
          {courierLabel}
        </span>
      );
    }

    if (order.tracking_number) {
      const line = [courierLabel ?? t("action_track"), order.courier_code?.toUpperCase()]
        .filter(Boolean)
        .join(" · ");
      return (
        <>
          <button
            type="button"
            data-testid="row-track-button"
            onClick={(e) => {
              e.stopPropagation();
              onTrack?.();
            }}
            className="group/track mt-1 inline-flex items-center gap-1 rounded-sm text-[11px] font-semibold text-brand-700 underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
          >
            <Package
              className="size-3 transition-transform duration-150 group-hover/track:translate-x-0.5 motion-reduce:transition-none"
              aria-hidden
            />
            {line}
          </button>
          <div className="font-mono text-[10px] text-ink-600">{order.tracking_number}</div>
        </>
      );
    }

    return (
      <span data-testid="row-shipment-status" className="mt-1 block text-[11px] text-ink-600">
        {courierLabel ?? "—"}
      </span>
    );
  }

  return (
    <tr
      onClick={onOpen}
      className="cursor-pointer border-t align-top transition-colors hover:bg-brand-50/70"
    >
      <td className="px-4 py-3">
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onOpen();
          }}
          className="rounded-sm font-mono font-semibold hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
        >
          <span className="sr-only">{t("orders_detail_open")} </span>
          {orderNumber(order)}
        </button>
        {hasCreatedAt && (
          <div className="mt-0.5 text-[11px] text-ink-600">
            {dateFormatter.format(createdAt)} ·{" "}
            <span
              data-testid="row-age"
              className={cn(stale && "font-semibold text-destructive")}
            >
              {ageLabel(ageMs)}
            </span>
          </div>
        )}
        <div className="mt-1 space-y-0.5 text-[11px] text-ink-600 md:hidden">
          <div>{buyerLabel(order)}</div>
          <div className="tabular-nums">{formatRupiah(order.total)}</div>
        </div>
      </td>

      {/* max-w + truncate: student_id is a raw UUID and wrapped to three lines,
          which made every row in the table a different height. */}
      <td className="hidden max-w-[14rem] px-4 py-3 md:table-cell">
        <div className="truncate font-medium">{buyerLabel(order)}</div>
        <div className="truncate font-mono text-[10px] text-ink-600" title={order.student_id}>
          {order.student_id}
        </div>
      </td>

      <td className="hidden max-w-xs px-4 py-3 md:table-cell">
        <div className="truncate">{items[0]?.name ?? "—"}</div>
        {extraItems > 0 && (
          <div className="mt-0.5 text-[11px] text-ink-600">+{extraItems} lainnya</div>
        )}
      </td>

      <td className="hidden px-4 py-3 text-right tabular-nums whitespace-nowrap md:table-cell">
        {formatRupiah(order.total)}
      </td>

      <td className="px-4 py-3">
        <OrderStatusBadge status={order.status} />
        {courierSubLine()}
      </td>

      <td className="px-4 py-3 text-right" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-end gap-2">
          {primaryAction && (
            <Button size="sm" onClick={primaryAction.onClick} disabled={primaryAction.disabled}>
              {primaryAction.label}
            </Button>
          )}
          {menuActions.length > 0 && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  data-testid="row-menu-trigger"
                  aria-label={t("row_actions_more")}
                >
                  <MoreHorizontal />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {menuActions.map((action) => (
                  <DropdownMenuItem
                    key={action.label}
                    variant={action.destructive ? "destructive" : "default"}
                    disabled={action.disabled}
                    onSelect={action.onClick}
                  >
                    {action.label}
                  </DropdownMenuItem>
                ))}
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        </div>
      </td>
    </tr>
  );
}
