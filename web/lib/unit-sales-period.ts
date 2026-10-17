import { presetRange, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";

export interface UnitSalesPeriod {
  preset: PeriodPreset;
  range: PeriodRange;
}

export function explicitUnitRange(preset: PeriodPreset, range: PeriodRange): PeriodRange {
  if (preset === "all_time") return {};
  if (preset === "custom") return range;
  const date = (days: number) => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(new Date(Date.now() - days * 86400000));
  return { from: preset === "30d" ? date(29) : presetRange(preset).from, to: date(0) };
}

function validDate(value: string) {
  return /^\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value)) && new Date(value).toISOString().slice(0, 10) === value;
}

export function parseUnitPeriod(params: { get: (key: string) => string | null }): UnitSalesPeriod {
  const preset = params.get("preset");
  const from = params.get("from") || undefined;
  const to = params.get("to") || undefined;
  const fallback = () => ({ preset: "30d" as const, range: explicitUnitRange("30d", {}) });
  if (!["7d", "30d", "90d", "this_month", "custom", "all_time"].includes(preset ?? "")) return fallback();
  if ((from && !validDate(from)) || (to && !validDate(to)) || (from && to && from > to)) return fallback();
  if (preset === "all_time") return { preset, range: {} };
  if ((!from && !to) || (preset !== "custom" && (!from || !to))) return fallback();
  const range = { from, to };
  if (preset === "custom") return { preset, range };
  const window = explicitUnitRange(preset as PeriodPreset, {});
  return window.from === from && window.to === to ? { preset: preset as PeriodPreset, range } : { preset: "custom", range };
}

export function unitPeriodSearch({ preset, range }: UnitSalesPeriod): string {
  const params = new URLSearchParams({ preset });
  if (preset !== "all_time") {
    if (range.from) params.set("from", range.from);
    if (range.to) params.set("to", range.to);
  }
  return params.toString();
}
