"use client";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { OrderStatus } from "@/lib/types";

interface StatusMeta {
  label: string;
  tone: string;
  bg: string;
}

const STATUS_META: Record<OrderStatus, StatusMeta> = {
  cart: { label: "Keranjang", tone: "text-ink-600", bg: "bg-line-2" },
  payment_pending: { label: "Menunggu Pembayaran", tone: "text-warn", bg: "bg-warn-bg" },
  paid: { label: "Dibayar", tone: "text-success", bg: "bg-success-bg" },
  processing: { label: "Diproses", tone: "text-info", bg: "bg-info-bg" },
  shipped: { label: "Dikirim", tone: "text-brand-700", bg: "bg-brand-50" },
  completed: { label: "Selesai", tone: "text-success", bg: "bg-success-bg" },
  payment_expired: { label: "Kadaluarsa", tone: "text-danger", bg: "bg-danger-bg" },
  cancelled: { label: "Dibatalkan", tone: "text-ink-500", bg: "bg-line-2" },
};

export interface OrderStatusBadgeProps {
  status: OrderStatus;
  className?: string;
  label?: string;
}

export function OrderStatusBadge({ status, className, label }: OrderStatusBadgeProps) {
  const meta = STATUS_META[status] ?? STATUS_META.cart;
  return (
    <Badge variant="outline" className={cn("border-transparent", meta.bg, meta.tone, className)}>
      {label ?? meta.label}
    </Badge>
  );
}

export const ORDER_STATUS_META = STATUS_META;