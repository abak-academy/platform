import type { Lang } from "@/lib/i18n";

const ZONE = "Asia/Jakarta";

export interface AccessWindowParts {
  from: { date: string; time: string };
  to: { date: string; time: string } | null;
  sameDay: boolean;
}

function localeFor(lang: Lang): string {
  return lang === "id" ? "id-ID" : "en-GB";
}

function dayKey(d: Date): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: ZONE }).format(d);
}

function parts(d: Date, lang: Lang, weekday: boolean) {
  return {
    date: new Intl.DateTimeFormat(localeFor(lang), {
      timeZone: ZONE,
      ...(weekday ? { weekday: "long" as const } : {}),
      day: "numeric",
      month: "short",
      year: "numeric",
    }).format(d),
    time: new Intl.DateTimeFormat(localeFor(lang), {
      timeZone: ZONE,
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    }).format(d),
  };
}

export function accessWindow(
  startIso: string | null,
  endIso: string | null,
  lang: Lang,
  opts: { weekday?: boolean } = {},
): AccessWindowParts | null {
  const start = startIso ? new Date(startIso) : null;
  if (!start || Number.isNaN(start.getTime())) return null;
  const end = endIso ? new Date(endIso) : null;
  const validEnd = end && !Number.isNaN(end.getTime()) ? end : null;
  return {
    from: parts(start, lang, !!opts.weekday),
    to: validEnd ? parts(validEnd, lang, !!opts.weekday) : null,
    sameDay: validEnd ? dayKey(start) === dayKey(validEnd) : false,
  };
}

export function workingMinutes(exam: {
  timer_mode: "overall" | "per_test";
  duration_minutes: number | null;
  sections_duration_minutes: number | null;
}): number | null {
  if (exam.timer_mode === "per_test") return exam.sections_duration_minutes ?? exam.duration_minutes;
  return exam.duration_minutes;
}

export type CountdownUnit = "day" | "hour" | "minute";

export function countdownParts(ms: number): { unit: CountdownUnit; n: number }[] | null {
  if (ms <= 0) return null;
  const minutes = Math.ceil(ms / 60_000);
  const days = Math.floor(minutes / (24 * 60));
  const hours = Math.floor((minutes % (24 * 60)) / 60);
  const mins = minutes % 60;
  const all: { unit: CountdownUnit; n: number }[] = [
    { unit: "day", n: days },
    { unit: "hour", n: hours },
    { unit: "minute", n: mins },
  ];
  return all.filter((p) => p.n > 0).slice(0, 2);
}
