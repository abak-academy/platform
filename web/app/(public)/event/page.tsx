"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { SearchX, Trophy } from "lucide-react";
import { EditionCard } from "@/components/catalog/EditionCard";
import { EventSeriesPicker } from "@/components/events/EventSeriesPicker";
import { EventStoryCard } from "@/components/events/EventStoryCard";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import { useEventSeries } from "@/lib/hooks/event-series";
import { useTranslation } from "@/lib/i18n";
import { waHref } from "@/lib/site-config";
import { cn } from "@/lib/utils";

const ALL = "all";
const wrap = "mx-auto w-full max-w-public px-6";

function FilterRow({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
}) {
  const { t } = useTranslation();
  return (
    <div role="group" aria-label={label} className="flex flex-wrap items-center gap-2">
      <span className="type-label-sm w-16 text-ink-500">{label}</span>
      {[{ value: ALL, label: t("event_page_filter_all") }, ...options].map((o) => (
        <button
          key={o.value}
          type="button"
          aria-pressed={value === o.value}
          onClick={() => onChange(o.value)}
          className={cn(
            "h-8 rounded-full border-[1.5px] px-3 type-label-sm transition-colors",
            value === o.value ? "border-brand-600 bg-brand-50 text-brand-ink" : "border-line bg-surface hover:border-ink-300",
          )}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

export default function EventPage() {
  const { t, lang } = useTranslation();
  const { data: events, isLoading } = useEventSeries(lang);
  const [selected, setSelected] = useState("");
  const [fEvent, setFEvent] = useState(ALL);
  const [fScope, setFScope] = useState(ALL);
  const [fStage, setFStage] = useState(ALL);
  const storyRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!events?.length) return;
    const apply = (scroll: boolean) => {
      const slug = decodeURIComponent(window.location.hash.slice(1));
      const hit = events.find((e) => e.slug === slug);
      if (hit) {
        setSelected(hit.slug);
        if (scroll) storyRef.current?.scrollIntoView?.({ behavior: "smooth", block: "start" });
      }
    };
    apply(true);
    const onHash = () => apply(true);
    window.addEventListener("hashchange", onHash);
    return () => window.removeEventListener("hashchange", onHash);
  }, [events]);

  const current = events?.find((e) => e.slug === selected) ?? events?.[0];

  const rows = useMemo(
    () => (events ?? []).flatMap((event) => event.open_editions.map((edition) => ({ event, edition }))),
    [events],
  );
  const stages = useMemo(() => [...new Set(rows.flatMap((r) => r.edition.stages))], [rows]);
  const filtered = rows.filter(
    (r) =>
      (fEvent === ALL || r.event.slug === fEvent) &&
      (fScope === ALL || r.event.scope === fScope) &&
      (fStage === ALL || r.edition.stages.includes(fStage)),
  );
  const reset = () => {
    setFEvent(ALL);
    setFScope(ALL);
    setFStage(ALL);
  };

  const noEvents = (
    <EmptyState
      icon={Trophy}
      title={t("land_event_empty_title")}
      text={t("land_event_empty_text")}
      action={
        <Button asChild>
          <a href={waHref(t("land_event_empty_wa"))} target="_blank" rel="noopener noreferrer">
            {t("land_event_empty_cta")}
          </a>
        </Button>
      }
    />
  );

  return (
    <div>
      <section className="relative overflow-hidden pt-32 pb-12">
        <div aria-hidden className="page-hero-fade absolute inset-0 -z-10 bg-brand-50" />
        <div className={wrap}>
          <span className="type-overline text-ember-700">{t("event_page_eyebrow")}</span>
          <h1 className="mt-2 type-display">{t("event_page_title")}</h1>
          <p className="mt-3 max-w-2xl type-body text-ink-700">{t("event_page_lede")}</p>
        </div>
      </section>

      <section data-reveal className="py-12">
        <div className={wrap}>
          <span className="type-overline text-ink-500">{t("event_page_know_eyebrow")}</span>
          <h2 className="mt-1 mb-6 type-h1">{t("event_page_know_title")}</h2>
          {isLoading ? (
            <Skeleton data-testid="events-skeleton" className="h-72 w-full rounded-3xl" />
          ) : !events?.length || !current ? (
            noEvents
          ) : (
            <div className="grid gap-6">
              <EventSeriesPicker events={events} selected={current.slug} onSelect={setSelected} />
              <div ref={storyRef} className="scroll-mt-24">
                <EventStoryCard event={current} />
              </div>
            </div>
          )}
        </div>
      </section>

      <section data-reveal className="pt-12 pb-20">
        <div className={wrap}>
          <span className="type-overline text-ink-500">{t("event_page_editions_eyebrow")}</span>
          <h2 className="mt-1 mb-6 type-h1">{t("event_page_editions_title")}</h2>
          {isLoading ? (
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {[0, 1, 2].map((i) => (
                <Skeleton key={i} className="h-40 rounded-2xl" />
              ))}
            </div>
          ) : !events?.length ? (
            noEvents
          ) : (
            <>
              <div className="mb-8 grid gap-3">
                <FilterRow
                  label={t("event_page_filter_event")}
                  value={fEvent}
                  options={events.filter((e) => e.open_editions.length).map((e) => ({ value: e.slug, label: e.short_name ?? e.name }))}
                  onChange={setFEvent}
                />
                <FilterRow
                  label={t("event_page_filter_level")}
                  value={fScope}
                  options={[
                    { value: "national", label: t("event_page_scope_national") },
                    { value: "international", label: t("event_page_scope_international") },
                  ]}
                  onChange={setFScope}
                />
                <FilterRow
                  label={t("event_page_filter_grade")}
                  value={fStage}
                  options={stages.map((s) => ({ value: s, label: s }))}
                  onChange={setFStage}
                />
              </div>
              <p className="mb-4 type-label-sm text-ink-500">{t("event_page_count").replace("{n}", String(filtered.length))}</p>
              {filtered.length ? (
                <div data-reveal-grid className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                  {filtered.map((r) => (
                    <div key={r.edition.product_id} data-tilt className="h-full">
                      <EditionCard event={r.event} edition={r.edition} />
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyState
                  icon={SearchX}
                  title={t("event_page_empty_title")}
                  text={t("event_page_empty_text")}
                  action={
                    <Button variant="outline" onClick={reset}>
                      {t("event_page_reset")}
                    </Button>
                  }
                />
              )}
            </>
          )}
        </div>
      </section>
    </div>
  );
}
