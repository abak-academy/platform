"use client";

import Link from "next/link";
import { Badge } from "@/components/ui/badge";
import { formatDay } from "@/components/events/EventSchedule";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { levelText } from "@/lib/student-home";
import type { PublicEdition, PublicEvent } from "@/lib/types";

export function EditionCard({ event, edition }: { event: PublicEvent; edition: PublicEdition }) {
  const { t, lang } = useTranslation();
  const level = levelText(edition.stages, edition.grade_min, edition.grade_max, t("grade"));
  return (
    <Link
      href={`/catalog/${edition.product_id}`}
      data-tilt
      className="flex flex-col gap-2 rounded-2xl border border-line bg-surface p-5 shadow-card transition-shadow hover:shadow-pop"
    >
      <div className="flex flex-wrap items-center gap-2">
        {event.scope && (
          <Badge variant="sky">{t(event.scope === "international" ? "event_page_scope_international" : "event_page_scope_national")}</Badge>
        )}
        {edition.phase === "live" ? (
          <Badge variant="live">{t("event_page_phase_live")}</Badge>
        ) : (
          <Badge variant="teal">{t("event_page_phase_open")}</Badge>
        )}
      </div>
      <span className="type-overline text-ink-600">
        {[event.short_name ?? event.name, edition.scheduled_at && formatDay(edition.scheduled_at, lang), level].filter(Boolean).join(" · ")}
      </span>
      <h3 className="type-h3 text-ink-900">{edition.title}</h3>
      <b className="mt-auto type-label">{edition.price > 0 ? formatRupiah(edition.price) : t("event_page_free")}</b>
    </Link>
  );
}
