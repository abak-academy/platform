"use client";

import { useState } from "react";
import { ChevronDown } from "lucide-react";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { CourierRate } from "@/lib/types";

// courierRateKey uniquely identifies a quoted rate by carrier + service, since
// a single carrier (e.g. JNE) can offer multiple services at different prices.
export function courierRateKey(rate: { courier: string; service: string }): string {
  return `${rate.courier}::${rate.service}`;
}

// Carriers announce same-day delivery in the service name rather than as a day
// count, so estimated_days is 0 for both "arrives today" and "we could not read
// the carrier's duration". This tells those two apart.
const SAME_DAY = /same.?day|instant|sameday/i;

function isSameDay(rate: CourierRate): boolean {
  return rate.estimated_days === 0 && SAME_DAY.test(rate.service ?? "");
}

// Sorted by how soon it arrives, then by price. Same-day sorts first; rates with
// no readable duration sort last, since we cannot promise anything about them.
function arrivalRank(rate: CourierRate): number {
  if (isSameDay(rate)) return 0;
  if (rate.estimated_days > 0) return rate.estimated_days;
  return Number.MAX_SAFE_INTEGER;
}

// The weekday is dropped when the copy already says "besok"/"tomorrow" — saying
// both gives "besok, Sel, 28 Jul", which reads as two separate answers to the
// same question.
function formatArrivalDate(days: number, lang: string, withWeekday: boolean): string {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-GB", {
    ...(withWeekday ? { weekday: "short" as const } : {}),
    day: "numeric",
    month: "short",
  }).format(d);
}

export function CourierRateList({
  rates,
  selectedKey,
  onSelect,
  isLoading,
  isError,
}: CourierRateListProps) {
  const { t, lang } = useTranslation();
  const [open, setOpen] = useState(false);

  // A date reads as a firmer commitment than "2 days" does, so it stays hedged:
  // the carrier's figure is an estimate and the wording says so. Returns null
  // when there is nothing honest to show.
  function arrivalLabel(rate: CourierRate): string | null {
    const prefix = t("cart_eta_prefix" as any) || "Estimated arrival";
    if (isSameDay(rate)) return `${prefix} ${t("cart_eta_today" as any) || "today"}`;
    if (rate.estimated_days > 0) {
      const tomorrow = rate.estimated_days === 1;
      const date = formatArrivalDate(rate.estimated_days, lang, !tomorrow);
      if (tomorrow) {
        return `${prefix} ${t("cart_eta_tomorrow" as any) || "tomorrow"}, ${date}`;
      }
      return `${prefix} ${date}`;
    }
    return null;
  }

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
        <div className="h-5 w-32 animate-pulse rounded bg-line" />
        <div className="h-14 animate-pulse rounded-lg bg-line" />
      </div>
    );
  }

  if (isError || !rates || rates.length === 0) {
    return (
      <div className="rounded-lg border border-line bg-surface p-5 text-center">
        <p className="text-sm text-ink-500">
          {t("cart_shipping_error") || "Unable to calculate shipping cost"}
        </p>
      </div>
    );
  }

  const selected = rates.find((r) => courierRateKey(r) === selectedKey) ?? null;
  const sorted = [...rates].sort(
    (a, b) => arrivalRank(a) - arrivalRank(b) || a.price - b.price,
  );

  // Collapsed once something is chosen: the list has served its purpose and the
  // buyer's next step is payment, not more comparison. Nothing chosen yet means
  // there is a decision outstanding, so the options stay open.
  const expanded = !selected || open;

  function choose(rate: CourierRate) {
    onSelect(rate);
    setOpen(false);
  }

  return (
    <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
      <h3 className="font-semibold text-ink-900">
        {t("cart_shipping_options") || "Shipping Options"}
      </h3>

      {!expanded && selected ? (
        <button
          type="button"
          onClick={() => setOpen(true)}
          className="flex w-full items-center gap-3 rounded-lg border border-line px-3 py-2.5 text-left transition-colors hover:border-brand-200 hover:bg-surface-2 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500"
        >
          <CarrierTile courier={selected.courier} selected={false} />
          <span className="flex min-w-0 flex-1 flex-col">
            <span className="truncate text-sm font-semibold text-ink-900">
              {selected.courier} · {selected.service}
            </span>
            <span className="truncate text-xs text-ink-500">
              {arrivalLabel(selected) ?? formatRupiah(selected.price)}
            </span>
          </span>
          <span className="shrink-0 text-sm font-semibold tabular-nums text-ink-900">
            {formatRupiah(selected.price)}
          </span>
          <ChevronDown className="size-4 shrink-0 text-ink-400" aria-hidden="true" />
          <span className="sr-only">{t("cart_shipping_change" as any) || "Change"}</span>
        </button>
      ) : (
        <div
          role="radiogroup"
          aria-label={t("cart_shipping_options") || "Shipping Options"}
          className="flex flex-col gap-1.5"
        >
          {sorted.map((rate) => {
            const key = courierRateKey(rate);
            const isSelected = selectedKey === key;
            const eta = arrivalLabel(rate);
            return (
              <button
                key={key}
                type="button"
                role="radio"
                aria-checked={isSelected}
                onClick={() => choose(rate)}
                className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 ${
                  isSelected
                    ? "border-brand-400 bg-brand-50"
                    : "border-line bg-surface hover:border-brand-200 hover:bg-surface-2"
                }`}
              >
                <CarrierTile courier={rate.courier} selected={isSelected} />
                <span className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-sm font-semibold text-ink-900">
                    {rate.courier} · {rate.service}
                    {rate.is_estimate && (
                      <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-[10px] font-medium text-warn">
                        {t("cart_rate_estimate_badge" as any)}
                      </span>
                    )}
                  </span>
                  {eta && <span className="truncate text-xs text-ink-500">{eta}</span>}
                </span>
                <span className="shrink-0 text-sm font-semibold tabular-nums text-ink-900">
                  {formatRupiah(rate.price)}
                </span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// Carriers brand themselves two ways: as acronyms (JNE, TIKI) and as compounds
// (SiCepat, AnterAja). Truncating the first kind reads as a mistake — "JN" looks
// like a typo — so short all-caps names are kept whole and compounds collapse to
// their capitals. Real carrier logos can replace this without touching layout.
function monogram(courier: string): string {
  const cleaned = courier.replace(/[^A-Za-z]/g, "");
  if (!cleaned) return "?";
  if (cleaned.length <= 4 && cleaned === cleaned.toUpperCase()) return cleaned;
  const capitals = cleaned.replace(/[^A-Z]/g, "");
  if (capitals.length >= 2) return capitals.slice(0, 3);
  return cleaned.slice(0, 4).toUpperCase();
}

function CarrierTile({ courier, selected }: { courier: string; selected: boolean }) {
  const label = monogram(courier);
  return (
    <span
      aria-hidden="true"
      className={`flex h-7 w-8 shrink-0 items-center justify-center rounded-md font-bold tracking-tight ${
        label.length > 3 ? "text-[8px]" : "text-[10px]"
      } ${selected ? "bg-brand-500 text-white" : "bg-line-2 text-ink-600"}`}
    >
      {label}
    </span>
  );
}

interface CourierRateListProps {
  rates: CourierRate[] | undefined;
  selectedKey: string | null;
  onSelect: (rate: CourierRate) => void;
  isLoading: boolean;
  isError: boolean;
}
