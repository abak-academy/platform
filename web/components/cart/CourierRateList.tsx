"use client";

import { Fragment, useState } from "react";
import { ChevronDown } from "lucide-react";
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
  const [open, setOpen] = useState(false);

  if (isLoading) {
    return (
      <div className="flex flex-col gap-2">
        <div className="h-3 w-28 animate-pulse rounded bg-line" />
        <div className="h-12 animate-pulse rounded-lg bg-line" />
      </div>
    );
  }

  if (isError || !rates || rates.length === 0) {
    return (
      <div className="rounded-lg border border-line px-3 py-4 text-center">
        <p className="text-sm text-ink-500">
          {t("cart_shipping_error") || "Unable to calculate shipping cost"}
        </p>
      </div>
    );
  }

  const selected = rates.find((r) => courierRateKey(r) === selectedKey) ?? null;

  // Sorted by arrival first, then price. "Takes a few days" spans anything from
  // two days to a week, so ordering it purely by price puts a later delivery
  // above an earlier one over a thousand rupiah — which reads as a mistake.
  const grouped = TIER_ORDER.map((tier) => ({
    tier,
    rates: rates
      .filter((r) => speedTier(r) === tier)
      .sort((a, b) => a.estimated_days - b.estimated_days || a.price - b.price),
  })).filter((g) => g.rates.length > 0);

  const tierLabel: Record<SpeedTier, string> = {
    same_day: t("cart_speed_same_day" as any) || "Arrives today",
    next_day: t("cart_speed_next_day" as any) || "Arrives tomorrow",
    regular: t("cart_speed_regular" as any) || "Takes a few days",
  };

  // The group heading already states when it arrives, so a row only adds the
  // exact date where that is extra information. Under "Arrives today" it would
  // be repeating itself, and under "Arrives tomorrow" the weekday is implied.
  function rowDate(rate: CourierRate): string | null {
    if (isSameDay(rate) || rate.estimated_days < 1) return null;
    return formatArrivalDate(rate.estimated_days, lang);
  }

  // The courier API answers an address it cannot place with a plain "no rates"
  // rather than an error, so the flat rate standing alone is the only signal the
  // buyer gets that their region was never priced. Say so, or the estimate reads
  // as a quote.
  const fellBackToFlatRate = rates.every((r) => r.is_estimate);

  function choose(rate: CourierRate) {
    onSelect(rate);
    setOpen(false);
  }

  return (
    <div className="flex flex-col gap-2">
      <p className="text-[11px] font-semibold uppercase tracking-[0.06em] text-ink-500">
        {t("cart_shipping_options") || "Shipping Options"}
      </p>

      {fellBackToFlatRate && (
        <p className="rounded-lg border border-warn/30 bg-warn-bg px-3 py-2 text-xs text-warn">
          {t("cart_shipping_flat_fallback" as any)}
        </p>
      )}

      <div className="overflow-hidden rounded-lg border border-line">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className="flex w-full items-center gap-3 bg-surface px-3 py-2.5 text-left transition-colors hover:bg-surface-2 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500"
        >
          {selected ? (
            <>
              <CarrierTile courier={selected.courier} selected={false} />
              <span className="flex min-w-0 flex-1 flex-col">
                <span className="truncate text-sm font-semibold text-ink-900">
                  {selected.courier} · {selected.service}
                </span>
                <span className="truncate text-xs text-ink-500">
                  {tierLabel[speedTier(selected)]}
                  {rowDate(selected) ? ` · ${rowDate(selected)}` : ""}
                </span>
              </span>
              <span className="shrink-0 text-sm font-semibold tabular-nums text-ink-900">
                {formatRupiah(selected.price)}
              </span>
            </>
          ) : (
            <span className="flex-1 text-sm text-ink-500">
              {t("cart_shipping_placeholder" as any) || "Choose a shipping service"}
            </span>
          )}
          <ChevronDown
            className={`size-4 shrink-0 text-ink-400 transition-transform ${open ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </button>

        {open && (
          <div
            role="radiogroup"
            aria-label={t("cart_shipping_options") || "Shipping Options"}
            className="border-t border-line"
          >
            {grouped.map((group) => (
              <Fragment key={group.tier}>
                {/* A filled band, not a hairline: the previous eyebrow-and-rule
                    treatment read as a divider and the eye skipped it. */}
                <p className="flex items-center justify-between border-b border-line bg-surface-2 px-3 py-1.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-ink-600">
                  {tierLabel[group.tier]}
                  <span className="tabular-nums font-normal text-ink-400">{group.rates.length}</span>
                </p>
                {group.rates.map((rate) => {
                  const key = courierRateKey(rate);
                  const isSelected = selectedKey === key;
                  const date = rowDate(rate);
                  return (
                    <button
                      key={key}
                      type="button"
                      role="radio"
                      aria-checked={isSelected}
                      onClick={() => choose(rate)}
                      className={`flex w-full items-center gap-3 border-b border-line px-3 py-2.5 text-left transition-colors last:border-b-0 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-brand-500 ${
                        isSelected ? "bg-brand-50" : "bg-surface hover:bg-surface-2"
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
                        {date && (
                          <span className="truncate text-xs text-ink-500">
                            {t("cart_eta_prefix" as any) || "Estimated arrival"} {date}
                          </span>
                        )}
                      </span>
                      <span className="shrink-0 text-sm font-semibold tabular-nums text-ink-900">
                        {formatRupiah(rate.price)}
                      </span>
                    </button>
                  );
                })}
              </Fragment>
            ))}
          </div>
        )}
      </div>
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
