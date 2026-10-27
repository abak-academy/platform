import type { PeriodPreset, PeriodRange } from "@/components/admin/PeriodBar";

function jakartaDaysAgo(days: number): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(new Date(Date.now() - days * 86400000));
}

// The affiliate endpoints read no dates as all time, unlike the admin dashboards
// where no dates means the last 30 days, so the 30-day preset is spelled out.
export function queryRange(preset: PeriodPreset, range: PeriodRange): PeriodRange {
  return preset === "30d" ? { from: jakartaDaysAgo(29) } : range;
}
