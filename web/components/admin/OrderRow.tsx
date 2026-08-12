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

/** "SMAN 3 Bogor · Kelas 12" — whichever parts exist, never a stray separator. */
function buyerContextLabel(order: Order): string {
  return [order.student_school?.trim(), order.student_grade ? `Kelas ${order.student_grade}` : ""]
    .filter(Boolean)
    .join(" · ");
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

  // created_at is stamped by MintCart, so a cart that sat idle for days carries
  // a date the buyer never saw. checked_out_at is when the order was placed —
  // null only on orders predating migration 0009.
  const placedIso = order.checked_out_at ?? order.created_at;
  const placedAt = placedIso ? Date.parse(placedIso) : NaN;
  const ageMs = Number.isNaN(placedAt) ? 0 : Math.max(0, Date.now() - placedAt);
  const hasPlacedAt = !Number.isNaN(placedAt);
  const stale = hasPlacedAt && isStale(order, ageMs);

  const items = order.items ?? [];
  const extraItems = Math.max(0, items.length - 1);
  const buyerContext = buyerContextLabel(order);

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
          className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-destructive"
        >
          <PackageX className="size-3.5" aria-hidden />
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
            className="group/track inline-flex items-center gap-1.5 rounded-sm text-[13px] font-semibold text-brand-700 underline underline-offset-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
          >
            <Package
              className="size-3.5 transition-transform duration-150 group-hover/track:translate-x-0.5 motion-reduce:transition-none"
              aria-hidden
            />
            {line}
          </button>
          <div className="font-mono text-xs text-ink-600">{order.tracking_number}</div>
        </>
      );
    }

    return (
      <span data-testid="row-shipment-status" className="block text-[13px] text-ink-600">
        {courierLabel ?? "—"}
      </span>
    );
  }

  return (
    <tr
      onClick={onOpen}
      className="cursor-pointer border-t align-top transition-colors hover:bg-brand-50/70"
    >
      <td className="px-4 py-4">
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onOpen();
          }}
          className="rounded-sm font-mono text-[15px] font-semibold hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
        >
          <span className="sr-only">{t("orders_detail_open")} </span>
          {orderNumber(order)}
        </button>
        {hasPlacedAt && (
          <div className="mt-1 text-[13px] text-ink-600">
            {dateFormatter.format(placedAt)} ·{" "}
            <span
              data-testid="row-age"
              className={cn(stale && "font-semibold text-destructive")}
            >
              {ageLabel(ageMs)}
            </span>
          </div>
        )}
        <div className="mt-1.5 space-y-0.5 text-[13px] text-ink-600 md:hidden">
          <div>{buyerLabel(order)}</div>
          <div className="tabular-nums">{formatRupiah(order.total)}</div>
        </div>
      </td>

      {/* School and grade, not student_id. The id was a raw UUID that meant
          nothing to anyone reading the table; this is what actually tells two
          buyers of the same name apart. Either may be missing, so the line
          collapses rather than printing a stray separator. */}
      <td className="hidden max-w-[16rem] px-4 py-4 md:table-cell">
        <div className="truncate text-[15px] font-medium">{buyerLabel(order)}</div>
        {buyerContext && (
          <div className="mt-1 truncate text-[13px] text-ink-600" title={buyerContext}>
            {buyerContext}
          </div>
        )}
      </td>

      <td className="hidden max-w-xs px-4 py-4 md:table-cell">
        <div className="truncate text-[15px]">{items[0]?.name ?? "—"}</div>
        {extraItems > 0 && (
          <div className="mt-1 text-[13px] text-ink-600">+{extraItems} lainnya</div>
        )}
      </td>

      <td className="hidden px-4 py-4 text-right text-[15px] tabular-nums whitespace-nowrap md:table-cell">
        {formatRupiah(order.total)}
      </td>

      {/* A column, not inline flow. The badge and the courier line are both
          inline-level, so they ran together on one line; the mockup stacks the
          courier state beneath the order state. */}
      <td className="px-4 py-4">
        <div className="flex flex-col items-start gap-1.5">
          <OrderStatusBadge status={order.status} />
          {courierSubLine()}
        </div>
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
