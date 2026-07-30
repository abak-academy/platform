"use client";

import type { OrderShipmentEvent } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";

export interface ShipmentTimelineProps {
  events?: OrderShipmentEvent[] | null;
}

function formatOccurredAt(iso: string, lang: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-GB", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(d);
}

export function ShipmentTimeline({ events }: ShipmentTimelineProps) {
  const { t, lang } = useTranslation();
  if (!events || events.length === 0) return null;

  // occurred_at, not array order, decides newest-first — the API makes no
  // ordering guarantee.
  const sorted = [...events].sort(
    (a, b) => new Date(b.occurred_at).getTime() - new Date(a.occurred_at).getTime(),
  );

  return (
    <div className="mt-4 flex flex-col gap-2">
      <h4 className="text-xs font-semibold uppercase tracking-[0.06em] text-ink-500">
        {t("order_shipment_timeline_heading")}
      </h4>
      <ol className="flex flex-col gap-1.5 text-sm">
        {sorted.map((ev) => (
          <li key={ev.id} className="flex items-start justify-between gap-3">
            <span data-testid="shipment-event-status" className="text-ink-900">
              {ev.status}
            </span>
            <span className="text-ink-500">{formatOccurredAt(ev.occurred_at, lang)}</span>
          </li>
        ))}
      </ol>
    </div>
  );
}
