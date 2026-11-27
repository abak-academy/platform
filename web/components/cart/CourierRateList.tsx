"use client";

import { useRef } from "react";
import { cn } from "@/lib/utils";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { CourierRate } from "@/lib/types";

// courierRateKey uniquely identifies a quoted rate by carrier + service, since
// a single carrier (e.g. JNE) can offer multiple services at different prices.
export function courierRateKey(rate: { courier: string; service: string }): string {
  return `${rate.courier}::${rate.service}`;
}

type SpeedTier = "same_day" | "next_day" | "regular";
const TIER_ORDER: SpeedTier[] = ["same_day", "next_day", "regular"];

// Carriers announce same-day delivery in the service name rather than as a day
// count, so estimated_days is 0 for both "arrives today" and "we could not read
// the carrier's duration". This tells those two apart.
const SAME_DAY = /same.?day|instant|sameday/i;

function isSameDay(rate: CourierRate): boolean {
  return rate.estimated_days === 0 && SAME_DAY.test(rate.service ?? "");
}

function speedTier(rate: CourierRate): SpeedTier {
  if (isSameDay(rate)) return "same_day";
  if (rate.estimated_days === 1) return "next_day";
  return "regular";
}

function formatArrivalDate(days: number, lang: string): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
  }).format(d);
}

interface CourierRateListProps {
  rates: CourierRate[] | undefined;
  selectedKey: string | null;
  onSelect: (rate: CourierRate) => void;
  isLoading: boolean;
  isError: boolean;
}

export function CourierRateList({
  rates,
  selectedKey,
  onSelect,
  isLoading,
  isError,
}: CourierRateListProps) {
  const { t, lang } = useTranslation();
  const groupRef = useRef<HTMLDivElement>(null);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        {[0, 1, 2].map((i) => (
          <div key={i} className="h-[69px] animate-pulse rounded-[16px] bg-line" />
        ))}
      </div>
    );
  }

  if (isError || !rates || rates.length === 0) {
    return (
      <p className="text-ink-600">
        {t("cart_shipping_error") || "Unable to calculate shipping cost"}
      </p>
    );
  }

  // Sorted by arrival first, then price. "Takes a few days" spans anything from
  // two days to a week, so ordering it purely by price puts a later delivery
  // above an earlier one over a thousand rupiah — which reads as a mistake.
  const sorted = TIER_ORDER.flatMap((tier) =>
    rates
      .filter((r) => speedTier(r) === tier)
      .sort((a, b) => a.estimated_days - b.estimated_days || a.price - b.price),
  );

  const tierLabel: Record<SpeedTier, string> = {
    same_day: t("cart_speed_same_day" as any) || "Arrives today",
    next_day: t("cart_speed_next_day" as any) || "Arrives tomorrow",
    regular: t("cart_speed_regular" as any) || "Takes a few days",
  };

  // Same-day and next-day already say when; only "a few days" gains the date.
  function rowDate(rate: CourierRate): string | null {
    if (speedTier(rate) !== "regular" || rate.estimated_days < 1) return null;
    return formatArrivalDate(rate.estimated_days, lang);
  }

  // Prototype `.choices` / `.choice`: one flat list of radio cards.
  const checkedIdx = sorted.findIndex((r) => courierRateKey(r) === selectedKey);

  function onKeyDown(e: React.KeyboardEvent) {
    const step = { ArrowDown: 1, ArrowRight: 1, ArrowUp: -1, ArrowLeft: -1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const next = (Math.max(checkedIdx, 0) + step + sorted.length) % sorted.length;
    onSelect(sorted[next]);
    groupRef.current?.querySelectorAll<HTMLElement>('[role="radio"]')[next]?.focus();
  }

  return (
    <div
      ref={groupRef}
      role="radiogroup"
      aria-label={t("cart_shipping_options") || "Shipping Options"}
      onKeyDown={onKeyDown}
      className="flex flex-col gap-2"
    >
      {sorted.map((rate, i) => {
        const key = courierRateKey(rate);
        const isSelected = selectedKey === key;
        const date = rowDate(rate);
        return (
          <button
            key={key}
            type="button"
            role="radio"
            aria-checked={isSelected}
            tabIndex={i === Math.max(checkedIdx, 0) ? 0 : -1}
            onClick={() => onSelect(rate)}
            className={cn(
              "flex w-full items-center gap-3 rounded-[16px] border-[1.5px] px-4 py-[13px] text-left text-sm transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600",
              isSelected ? "border-brand-600 bg-brand-50" : "border-line bg-surface hover:border-line-strong",
            )}
          >
            <span
              aria-hidden="true"
              className={cn(
                "grid size-[18px] shrink-0 place-items-center rounded-full border-[1.5px] bg-surface",
                isSelected ? "border-brand-600" : "border-line-strong",
              )}
            >
              {isSelected && <span className="size-2.5 rounded-full bg-brand-600" />}
            </span>
            <span className="min-w-0 flex-1">
              <b className="block font-bold text-ink-900">
                {rate.courier} · {rate.service}
              </b>
              <small className="block text-[12.5px] text-ink-600">
                {tierLabel[speedTier(rate)]}
                {date ? ` · ${date}` : ""}
              </small>
            </span>
            <b className="shrink-0 font-bold tabular-nums text-ink-900">{formatRupiah(rate.price)}</b>
          </button>
        );
      })}
    </div>
  );
}
