"use client";

import { Badge } from "@/components/ui/badge";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { PublicMilestone } from "@/lib/types";

export function formatDay(iso: string, lang: "id" | "en", withYear = true) {
  return new Intl.DateTimeFormat(lang === "en" ? "en-GB" : "id-ID", {
    day: "numeric",
    month: "short",
    ...(withYear && { year: "numeric" }),
    timeZone: "Asia/Jakarta",
  }).format(new Date(iso));
}

export function EventSchedule({ schedule }: { schedule: PublicMilestone[] }) {
  const { t, lang } = useTranslation();
  if (schedule.length === 0) return null;
  return (
    <div>
      <span className="type-overline text-ink-600">{t("land_story_schedule")}</span>
      <ol className="mt-3 grid gap-2">
        {schedule.map((m) => (
          <li
            key={`${m.label}-${m.starts_on}`}
            data-state={m.state}
            className={cn("flex flex-wrap items-center gap-x-3 gap-y-1", m.state === "done" && "text-ink-600")}
          >
            <span
              aria-hidden
              className={cn(
                "size-2.5 rounded-full border-2 border-brand-600",
                m.state === "done" && "border-ink-300 bg-ink-300",
                m.state === "ongoing" && "bg-ember-600 border-ember-600",
              )}
            />
            <b className="type-label">{m.label}</b>
            <time className="type-caption text-ink-600">
              {m.ends_on && m.ends_on !== m.starts_on
                ? `${formatDay(m.starts_on, lang, false)} – ${formatDay(m.ends_on, lang)}`
                : formatDay(m.starts_on, lang)}
            </time>
            {m.state === "ongoing" && <Badge variant="live">{t("land_story_state_ongoing")}</Badge>}
            {m.state === "done" && <Badge variant="mute">{t("land_story_state_done")}</Badge>}
          </li>
        ))}
      </ol>
    </div>
  );
}
