"use client";

import { Badge } from "@/components/ui/badge";
import type { OrderStatus } from "@/lib/types";

type Tone = "mute" | "gold" | "teal" | "sky" | "ember" | "danger";

interface StatusMeta {
  label: string;
  tone: Tone;
}

// Prototype ORDER_STATUS tones ('' → mute, info → sky).
const STATUS_META: Record<OrderStatus, StatusMeta> = {
  cart: { label: "Keranjang", tone: "mute" },
  payment_pending: { label: "Menunggu Pembayaran", tone: "gold" },
  paid: { label: "Dibayar", tone: "teal" },
  processing: { label: "Diproses", tone: "sky" },
  shipped: { label: "Dikirim", tone: "ember" },
  completed: { label: "Selesai", tone: "teal" },
  payment_expired: { label: "Kadaluarsa", tone: "danger" },
  cancelled: { label: "Dibatalkan", tone: "mute" },
};

export interface OrderStatusBadgeProps {
  status: OrderStatus;
  className?: string;
  label?: string;
}

export function OrderStatusBadge({ status, className, label }: OrderStatusBadgeProps) {
  const meta = STATUS_META[status] ?? STATUS_META.cart;
  return (
    <Badge variant={meta.tone} className={className}>
      {label ?? meta.label}
    </Badge>
  );
}

export const ORDER_STATUS_META = STATUS_META;
