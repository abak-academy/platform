"use client";

import { Input } from "@/components/ui/input";
import { useTranslation } from "@/lib/i18n";

export type PeriodPreset = "7d" | "30d" | "90d" | "this_month" | "custom";

export interface PeriodRange {
  from?: string;
  to?: string;
}

// The server buckets everything in Asia/Jakarta, and Indonesia spans three
// zones (WIB/WITA/WIT), so a Bali or Papua admin's browser-local date is also
// the wrong answer here — only an explicit timezone is safe. en-CA formats as
// YYYY-MM-DD directly.
function jakartaDateString(d: Date): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(d);
}

// Jakarta has no DST, so a calendar day is always exactly 86400000ms — plain
// epoch subtraction lands on the right Jakarta calendar date.
function isoDaysAgo(days: number): string {
  return jakartaDateString(new Date(Date.now() - days * 86400000));
}

function firstOfThisMonth(): string {
  return jakartaDateString(new Date()).slice(0, 7) + "-01";
}

// 30d sends nothing on purpose: the server default is already now-30d..now, so
// an explicit range would only add a client/server "today" mismatch.
//
// `to` on the server is exclusive (today+1), so an N-day window ending today
// must start at today-(N-1) — hence days-1 below, not days.
//
// web/app/(admin)/admin/revenue/page.tsx has a byte-identical (but unfixed —
// separate ticket, out of this PR's scope) copy of isoDaysAgo/firstOfThisMonth:
// UTC-based and off by one day. Keep that in mind before assuming the two agree.
export function presetRange(preset: PeriodPreset): PeriodRange {
  switch (preset) {
    case "7d":
      return { from: isoDaysAgo(6) };
    case "90d":
      return { from: isoDaysAgo(89) };
    case "this_month":
      return { from: firstOfThisMonth() };
    default:
      return {};
  }
}

const PRESETS: { id: PeriodPreset; key: "admin_home_period_7d" | "admin_home_period_30d" | "admin_home_period_90d" | "admin_home_period_this_month" | "admin_home_period_custom" }[] = [
  { id: "7d", key: "admin_home_period_7d" },
  { id: "30d", key: "admin_home_period_30d" },
  { id: "90d", key: "admin_home_period_90d" },
  { id: "this_month", key: "admin_home_period_this_month" },
  { id: "custom", key: "admin_home_period_custom" },
];

interface PeriodBarProps {
  preset: PeriodPreset;
  range: PeriodRange;
  onChange: (preset: PeriodPreset, range: PeriodRange) => void;
  resolvedFrom?: string;
  resolvedTo?: string;
}

export function PeriodBar({ preset, range, onChange, resolvedFrom, resolvedTo }: PeriodBarProps) {
  const { t } = useTranslation();

  return (
    <div
      className="flex flex-wrap items-center gap-3 rounded-[12px] border border-line px-4 py-3"
      style={{ backgroundColor: "var(--md-sys-color-surface-container-low)" }}
    >
      <span className="text-label uppercase color-on-surface-variant" style={{ letterSpacing: "0.06em" }}>
        {t("admin_home_period")}
      </span>

      <div className="flex overflow-hidden rounded-[10px] border border-line" role="group">
        {PRESETS.map((p) => (
          <button
            key={p.id}
            type="button"
            aria-pressed={preset === p.id}
            onClick={() => onChange(p.id, p.id === "custom" ? range : presetRange(p.id))}
            className="px-3 py-1.5 text-label border-r border-line last:border-r-0"
            style={
              preset === p.id
                ? {
                    backgroundColor: "var(--md-sys-color-primary)",
                    color: "var(--md-sys-color-on-primary)",
                    fontWeight: 600,
                  }
                : undefined
            }
          >
            {t(p.key)}
          </button>
        ))}
      </div>

      {preset === "custom" && (
        <div className="flex items-center gap-2">
          <Input
            type="date"
            value={range.from ?? ""}
            max={range.to}
            onChange={(e) => onChange("custom", { ...range, from: e.target.value })}
            className="w-auto"
          />
          <Input
            type="date"
            value={range.to ?? ""}
            min={range.from}
            onChange={(e) => onChange("custom", { ...range, to: e.target.value })}
            className="w-auto"
          />
        </div>
      )}

      {resolvedFrom && resolvedTo && (
        <span className="ml-auto text-label color-on-surface-variant">
          {t("admin_home_period_showing")
            .replace("{from}", resolvedFrom)
            .replace("{to}", resolvedTo)}
        </span>
      )}
    </div>
  );
}
