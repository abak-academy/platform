"use client";

import { Book, Minus, Plus, PlayCircle, Trash2, Trophy } from "lucide-react";
import type { OrderItem } from "@/lib/types";
import { formatRupiah } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";

const TYPE_META: Record<string, { label: string; tone: string; bg: string; Icon: typeof Book }> = {
  book: { label: "Buku", tone: "text-warn", bg: "bg-warn-bg", Icon: Book },
  course: { label: "Kursus", tone: "text-success", bg: "bg-success-bg", Icon: PlayCircle },
  package: { label: "Kompetisi", tone: "text-violet", bg: "bg-violet-bg", Icon: Trophy },
};

export interface CartLineItemProps {
  item: OrderItem;
  onRemove: () => void;
  onQtyChange: (qty: number) => void;
  removing?: boolean;
  updatingQty?: boolean;
  /** Drops the item's own card chrome so it can sit inside a shared container —
   *  the cart groups items and the courier picker into one block. */
  flat?: boolean;
}

export function CartLineItem({ item, onRemove, onQtyChange, removing, updatingQty, flat }: CartLineItemProps) {
  const { t } = useTranslation();
  const meta = TYPE_META[item.product_type] ?? TYPE_META.book;
  const { Icon } = meta;
  const lineTotal = item.jumlah ?? item.unit_price * item.qty;
  const busy = removing || updatingQty;
  const isDigital = item.product_type === "exam" || item.product_type === "course";

  return (
    <div
      className={
        flat
          ? "flex gap-4 px-4 py-4"
          : "flex gap-4 rounded-lg border border-line bg-surface p-4 shadow-[var(--sh-sm)]"
      }
    >
      <div
        className="flex size-16 shrink-0 items-center justify-center rounded-md bg-paper"
        aria-hidden
      >
        <Icon className="size-7 text-ink-400" strokeWidth={1.5} />
      </div>

      <div className="flex flex-1 flex-col gap-1">
        <div className="flex items-start justify-between gap-2">
          <div className="flex flex-col gap-1">
            <span className="text-[15px] font-semibold leading-snug text-ink-900">{item.name}</span>
            <Badge variant="outline" className={cn("w-fit border-transparent", meta.bg, meta.tone)}>
              {meta.label}
            </Badge>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="size-8 text-ink-400 hover:text-danger"
            onClick={onRemove}
            disabled={busy}
            aria-label={`Hapus ${item.name} dari keranjang`}
          >
            <Trash2 className="size-4" />
          </Button>
        </div>

        <div className="mt-1 flex items-center justify-between gap-2 text-sm">
          {isDigital ? (
            <span className="text-xs text-ink-500">{t("product_digital_single_qty" as any)}</span>
          ) : (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => onQtyChange(item.qty - 1)}
                disabled={busy || item.qty <= 1}
                className="flex size-7 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                aria-label="Kurangi jumlah"
              >
                <Minus className="size-3" />
              </button>
              <span className="w-6 text-center font-semibold text-ink-900">{item.qty}</span>
              <button
                type="button"
                onClick={() => onQtyChange(item.qty + 1)}
                disabled={busy || item.qty >= 10}
                className="flex size-7 items-center justify-center rounded-full border border-line text-ink-600 hover:bg-paper disabled:opacity-40"
                aria-label="Tambah jumlah"
              >
                <Plus className="size-3" />
              </button>
              <span className="text-xs text-ink-400">× {formatRupiah(item.unit_price)}</span>
            </div>
          )}
          <span className="font-serif text-base font-bold text-ink-900">{formatRupiah(lineTotal)}</span>
        </div>
      </div>
    </div>
  );
}