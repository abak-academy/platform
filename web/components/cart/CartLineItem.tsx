"use client";

import {
  Award,
  Book,
  ClipboardList,
  Minus,
  Package,
  PlayCircle,
  Plus,
  Trash2,
} from "lucide-react";
import type { OrderItem, ProductType } from "@/lib/types";
import { formatRupiah } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import { useTranslation } from "@/lib/i18n";
import { isDigitalType } from "@/lib/shipping";

// Keyed by ProductType, not by string: the compiler now refuses an incomplete
// map. It used to be Record<string, …> holding only book, course and a
// "package" type that does not exist — so exam, merchandise and medal all
// missed the lookup and a `?? book` default shipped them to checkout labelled
// "Buku". Mirrors the map in catalog/[id] and ProductCard.
const TYPE_META: Record<
  ProductType,
  { labelKey: string; tone: string; bg: string; Icon: typeof Book }
> = {
  book: {
    labelKey: "product_type_book",
    tone: "text-warn",
    bg: "bg-warn-bg",
    Icon: Book,
  },
  course: {
    labelKey: "product_type_course",
    tone: "text-success",
    bg: "bg-success-bg",
    Icon: PlayCircle,
  },
  exam: {
    labelKey: "product_type_exam",
    tone: "text-info",
    bg: "bg-info-bg",
    Icon: ClipboardList,
  },
  merchandise: {
    labelKey: "product_type_merchandise",
    tone: "text-warn",
    bg: "bg-warn-bg",
    Icon: Package,
  },
  medal: {
    labelKey: "product_type_medal",
    tone: "text-warn",
    bg: "bg-warn-bg",
    Icon: Award,
  },
};

// OrderItem carries product_type as a plain string, so an unrecognised one has
// to land somewhere. It shows its own name in neutral chrome — a lookup miss
// must never again resolve to a confident wrong type.
const UNKNOWN_META = {
  labelKey: "",
  tone: "text-ink-600",
  bg: "bg-line-2",
  Icon: Package,
};

export interface CartLineItemProps {
  item: OrderItem;
  onRemove: () => void;
  onQtyChange: (qty: number) => void;
  removing?: boolean;
  updatingQty?: boolean;
}

// Prototype `.line`: thumb · name/sub · stepper or Digital chip · price · remove.
export function CartLineItem({
  item,
  onRemove,
  onQtyChange,
  removing,
  updatingQty,
}: CartLineItemProps) {
  const { t } = useTranslation();
  const meta = TYPE_META[item.product_type as ProductType] ?? UNKNOWN_META;
  const { Icon } = meta;
  const label = meta.labelKey ? t(meta.labelKey as any) : item.product_type;
  const lineTotal = item.jumlah ?? item.unit_price * item.qty;
  const busy = removing || updatingQty;
  const isDigital = isDigitalType(item.product_type as ProductType);

  return (
    <div className="flex flex-wrap items-center gap-3 border-b border-dashed border-line py-3">
      <span
        className={cn(
          "grid size-[38px] shrink-0 place-items-center rounded-[12px]",
          meta.bg,
          meta.tone,
        )}
        aria-hidden
      >
        <Icon className="size-[18px]" />
      </span>

      <div className="min-w-0 flex-1 max-narrow:basis-[calc(100%-50px)]">
        <b className="block font-bold text-ink-900">{item.name}</b>
        <small className="block text-[12.5px] text-ink-600">
          <span>{label}</span>
          {item.qty > 1 && (
            <>
              {" · "}
              <span className="tabular-nums">× {formatRupiah(item.unit_price)}</span>
            </>
          )}
          {isDigital && (
            <>
              {" · "}
              <span>{t("product_digital_single_qty" as any)}</span>
            </>
          )}
        </small>
      </div>

      {/* On a phone the controls drop under the name instead of crushing it. */}
      <div className="ml-auto flex shrink-0 items-center gap-3">
        {isDigital ? (
          <Badge variant="teal">{t("cart_digital_chip" as any)}</Badge>
        ) : (
          <div className="inline-flex h-[34px] shrink-0 items-center rounded-full border-[1.5px] border-line bg-surface">
            <button
              type="button"
              onClick={() => onQtyChange(item.qty - 1)}
              disabled={busy || item.qty <= 1}
              className="grid h-full w-[30px] place-items-center rounded-full text-ink-900 disabled:opacity-40"
              aria-label={t("cart_qty_decrease")}
            >
              <Minus className="size-4" />
            </button>
            <output className="min-w-[26px] text-center font-[family-name:var(--font-display)] text-[15px] font-extrabold text-ink-900">
              {item.qty}
            </output>
            <button
              type="button"
              onClick={() => onQtyChange(item.qty + 1)}
              disabled={busy || item.qty >= 10}
              className="grid h-full w-[30px] place-items-center rounded-full text-ink-900 disabled:opacity-40"
              aria-label={t("cart_qty_increase")}
            >
              <Plus className="size-4" />
            </button>
          </div>
        )}

        <b className="shrink-0 font-bold tabular-nums text-ink-900">
          {formatRupiah(lineTotal)}
        </b>

        <button
          type="button"
          onClick={onRemove}
          disabled={busy}
          aria-label={t("cart_remove_item").replace("{name}", item.name)}
          className="grid size-8 shrink-0 place-items-center rounded-full border border-line bg-surface text-ink-700 transition-colors hover:text-danger disabled:opacity-40"
        >
          <Trash2 className="size-4" />
        </button>
      </div>
    </div>
  );
}
