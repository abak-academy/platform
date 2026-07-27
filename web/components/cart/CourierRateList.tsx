"use client";

import { Fragment } from "react";
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

// Carriers name the same speed differently and in two languages, so the tier is
// read off the service name. This is a stopgap: estimated_days should decide it,
// but the backend reports 0 for every rate because Biteship returns a human
// string ("1 - 2 days") that strconv.Atoi cannot read. Once that is fixed the
// number wins and these patterns stay only as the fallback.
const SAME_DAY = /same.?day|instant|sameday/i;
const NEXT_DAY = /besok|esok|one night|overnight|next.?day|\bYES\b/i;

function speedTier(rate: CourierRate): SpeedTier {
  if (rate.estimated_days === 1) return "next_day";
  const service = rate.service ?? "";
  if (SAME_DAY.test(service)) return "same_day";
  if (NEXT_DAY.test(service)) return "next_day";
  return "regular";
}

// Carriers brand themselves two ways: as acronyms (JNE, TIKI) and as compounds
// (SiCepat, AnterAja). Truncating the first kind reads as a mistake — "JN" looks
// like a typo — so short all-caps names are kept whole and compounds collapse to
// their capitals. Real carrier logos can replace this without touching layout;
// the tile keeps its box.
function monogram(courier: string): string {
  const cleaned = courier.replace(/[^A-Za-z]/g, "");
  if (!cleaned) return "?";
  if (cleaned.length <= 4 && cleaned === cleaned.toUpperCase()) return cleaned;
  const capitals = cleaned.replace(/[^A-Z]/g, "");
  if (capitals.length >= 2) return capitals.slice(0, 3);
  return cleaned.slice(0, 4).toUpperCase();
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
  const { t } = useTranslation();

  if (isLoading) {
    return (
      <div className="flex flex-col gap-3 rounded-lg border border-line bg-surface p-5">
        <div className="h-5 w-32 animate-pulse rounded bg-line" />
        <div className="space-y-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-12 animate-pulse rounded-lg bg-line" />
          ))}
        </div>
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

  const grouped = TIER_ORDER.map((tier) => ({
    tier,
    rates: rates
      .filter((r) => speedTier(r) === tier)
      .sort((a, b) => a.price - b.price),
  })).filter((g) => g.rates.length > 0);

  const tierLabel: Record<SpeedTier, string> = {
    same_day: t("cart_speed_same_day" as any) || "Arrives today",
    next_day: t("cart_speed_next_day" as any) || "Arrives tomorrow",
    regular: t("cart_speed_regular" as any) || "Standard",
  };

  return (
    <div className="flex flex-col gap-4 rounded-lg border border-line bg-surface p-5">
      <h3 className="font-semibold text-ink-900">
        {t("cart_shipping_options") || "Shipping Options"}
      </h3>

      <div role="radiogroup" aria-label={t("cart_shipping_options") || "Shipping Options"}>
        {grouped.map((group, groupIndex) => (
          <Fragment key={group.tier}>
            <div className={`flex items-baseline gap-2 pb-1.5 ${groupIndex > 0 ? "pt-4" : ""}`}>
              <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-500">
                {tierLabel[group.tier]}
              </span>
              <span className="h-px flex-1 bg-line" aria-hidden="true" />
              <span className="text-[11px] tabular-nums text-ink-400">{group.rates.length}</span>
            </div>

            <div className="flex flex-col gap-1.5">
              {group.rates.map((rate) => {
                const key = courierRateKey(rate);
                const isSelected = selectedKey === key;
                return (
                  <button
                    key={key}
                    type="button"
                    role="radio"
                    aria-checked={isSelected}
                    onClick={() => onSelect(rate)}
                    className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2.5 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-500 ${
                      isSelected
                        ? "border-brand-400 bg-brand-50"
                        : "border-line bg-surface hover:border-brand-200 hover:bg-surface-2"
                    }`}
                  >
                    <span
                      aria-hidden="true"
                      className={`flex h-7 w-8 shrink-0 items-center justify-center rounded-md font-bold tracking-tight ${
                        monogram(rate.courier).length > 3 ? "text-[8px]" : "text-[10px]"
                      } ${isSelected ? "bg-brand-500 text-white" : "bg-line-2 text-ink-600"}`}
                    >
                      {monogram(rate.courier)}
                    </span>

                    <span className="flex min-w-0 flex-1 flex-col">
                      <span className="truncate text-sm font-semibold text-ink-900">
                        {rate.courier}
                        {rate.is_estimate && (
                          <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-[10px] font-medium text-warn">
                            {t("cart_rate_estimate_badge" as any)}
                          </span>
                        )}
                      </span>
                      <span className="truncate text-xs text-ink-500">
                        {rate.service}
                        {/* Only render a duration we actually have — the backend
                            reports 0 when it could not parse the carrier's
                            duration string, and "0 days" reads as a promise. */}
                        {rate.estimated_days > 0 && (
                          <>
                            {" · "}
                            {rate.estimated_days} {t("cart_rate_days" as any) || "days"}
                          </>
                        )}
                      </span>
                    </span>

                    <span className="shrink-0 text-sm font-semibold tabular-nums text-ink-900">
                      {formatRupiah(rate.price)}
                    </span>
                  </button>
                );
              })}
            </div>
          </Fragment>
        ))}
      </div>
    </div>
  );
}
