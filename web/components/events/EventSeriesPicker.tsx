"use client";

import { useRef, type KeyboardEvent } from "react";
import { EventMark } from "@/components/admin/EventMark";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import type { EventIllustration, EventTone, PublicEvent } from "@/lib/types";

export function EventSeriesPicker({
  events,
  selected,
  onSelect,
}: {
  events: PublicEvent[];
  selected: string;
  onSelect: (slug: string) => void;
}) {
  const { t } = useTranslation();
  const refs = useRef<(HTMLButtonElement | null)[]>([]);

  const onKeyDown = (e: KeyboardEvent, i: number) => {
    const step = e.key === "ArrowRight" || e.key === "ArrowDown" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const next = (i + step + events.length) % events.length;
    onSelect(events[next].slug);
    refs.current[next]?.focus();
  };

  return (
    <div role="radiogroup" aria-label={t("land_event_rail_label")} className="flex gap-3 overflow-x-auto pb-1 sm:flex-wrap">
      {events.map((ev, i) => {
        const on = ev.slug === selected;
        return (
          <button
            key={ev.slug}
            ref={(el) => {
              refs.current[i] = el;
            }}
            type="button"
            role="radio"
            aria-checked={on}
            tabIndex={on ? 0 : -1}
            onClick={() => onSelect(ev.slug)}
            onKeyDown={(e) => onKeyDown(e, i)}
            className={cn(
              "flex min-w-[150px] flex-col items-start gap-1 rounded-2xl border-[1.5px] bg-surface p-3 text-left transition-colors outline-none focus-visible:outline-[2.5px] focus-visible:outline-offset-2 focus-visible:outline-brand-600",
              on ? "border-brand-600 bg-brand-50" : "border-line hover:border-ink-300",
            )}
          >
            <EventMark logoKey={ev.logo_url} tone={ev.tone as EventTone} illustration={ev.illustration as EventIllustration} className="size-12" />
            <b className="type-label text-ink-900">{ev.short_name ?? ev.name}</b>
            <small className="type-caption text-ink-600">{ev.levels_label}</small>
          </button>
        );
      })}
    </div>
  );
}
