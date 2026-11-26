"use client";

import Link from "next/link";
import { ArrowUpRight } from "lucide-react";
import { EventMark } from "@/components/admin/EventMark";
import { EventSchedule } from "@/components/events/EventSchedule";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { levelText } from "@/lib/student-home";
import { waHref } from "@/lib/site-config";
import type { EventIllustration, EventTone, PublicEvent } from "@/lib/types";

export function EventStoryCard({ event }: { event: PublicEvent }) {
  const { t } = useTranslation();
  const open = event.open_editions.slice(0, 2);
  const label = event.short_name ?? event.name;
  return (
    <article className="grid gap-6 rounded-3xl border border-line bg-surface p-6 sm:p-8 lg:grid-cols-[220px_1fr]">
      <div className="flex flex-col items-start gap-3">
        <EventMark logoKey={event.logo_url} tone={event.tone as EventTone} illustration={event.illustration as EventIllustration} className="size-32" />
        <div className="flex flex-wrap gap-2">
          {event.scope && (
            <Badge variant="sky">{t(event.scope === "international" ? "event_page_scope_international" : "event_page_scope_national")}</Badge>
          )}
          {event.levels_label && <Badge variant="mute">{event.levels_label}</Badge>}
        </div>
      </div>
      <div className="grid min-w-0 gap-5">
        <div>
          <span className="type-overline text-ink-500">{event.full_name}</span>
          <h3 className="mt-1 type-h2 text-ink-900">{label}</h3>
          <p className="mt-1 type-body text-brand-ink">{event.tagline}</p>
        </div>
        {event.story.map((p, i) => (
          <p key={i} className="type-body text-ink-700">
            {p}
          </p>
        ))}
        <EventSchedule schedule={event.schedule} />
        {event.rounds.length > 0 && (
          <div>
            <span className="type-overline text-ink-500">{t("land_story_rounds")}</span>
            <ol className="mt-3 flex flex-wrap gap-2">
              {event.rounds.map((r, i) => (
                <li key={r} className="flex items-center gap-2 rounded-full bg-surface-sunken px-3 py-1 type-label-sm">
                  <span className="grid size-5 place-items-center rounded-full bg-brand-600 text-on-brand type-label-xs">{i + 1}</span>
                  {r}
                </li>
              ))}
            </ol>
          </div>
        )}
        {event.categories.length > 0 && (
          <div>
            <span className="type-overline text-ink-500">{t("land_story_categories")}</span>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              {event.categories.map((c) => (
                <div key={c.name} className="rounded-xl border border-line p-3">
                  <b className="type-label">{c.name}</b>
                  <small className="block type-caption text-ink-500">{c.detail}</small>
                </div>
              ))}
            </div>
          </div>
        )}
        {event.facts.length > 0 && (
          <dl className="grid gap-3 sm:grid-cols-3">
            {event.facts.map((f) => (
              <div key={f.label}>
                <dt className="type-caption text-ink-500">{f.label}</dt>
                <dd className="type-label">{f.value}</dd>
              </div>
            ))}
          </dl>
        )}
        <div className="flex flex-wrap gap-3">
          {open.length > 0 ? (
            open.map((ed) => {
              const level = levelText(ed.stages, ed.grade_min, ed.grade_max, t("grade"));
              const price = ed.price > 0 ? formatRupiah(ed.price) : t("event_page_free");
              const text = level
                ? t("land_story_register").replace("{level}", level).replace("{price}", price)
                : t("land_story_register_plain").replace("{price}", price);
              return (
                <Button key={ed.product_id} asChild size="lg">
                  <Link href={`/catalog/${ed.product_id}`}>{text}</Link>
                </Button>
              );
            })
          ) : (
            <Button asChild size="lg">
              <a href={waHref(t("land_story_ask_wa").replace("{event}", label))} target="_blank" rel="noopener noreferrer">
                {t("land_story_ask")}
              </a>
            </Button>
          )}
          {event.official_url && (
            <Button asChild size="lg" variant="outline">
              <a href={event.official_url} target="_blank" rel="noopener noreferrer">
                {t("land_story_official")}
                <ArrowUpRight />
              </a>
            </Button>
          )}
        </div>
      </div>
    </article>
  );
}
