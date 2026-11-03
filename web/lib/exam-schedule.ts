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

function parts(d: Date, lang: Lang) {
  return {
    date: new Intl.DateTimeFormat(localeFor(lang), {
      timeZone: ZONE,
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
): AccessWindowParts | null {
  const start = startIso ? new Date(startIso) : null;
  if (!start || Number.isNaN(start.getTime())) return null;
  const end = endIso ? new Date(endIso) : null;
  const validEnd = end && !Number.isNaN(end.getTime()) ? end : null;
  return {
    from: parts(start, lang),
    to: validEnd ? parts(validEnd, lang) : null,
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
