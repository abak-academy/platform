"use client";

import type { OrderShipmentEvent } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import { shipmentStatusLabel } from "@/lib/shipment-status";

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
      {/* Same rail as the order's status history directly above it: two lists
          describing the same parcel should not speak in two visual dialects.
          Newest first here, because a courier log is read for "where is it
          now", not for the story from the beginning. */}
      <ol className="relative flex flex-col gap-3 pl-5 text-sm">
        {sorted.map((ev, i) => {
          const isLast = i === sorted.length - 1;
          const delay = `${i * 80}ms`;
          return (
            <li key={ev.id} className="tl-step relative" style={{ animationDelay: delay }}>
              {!isLast && (
                <span
                  className="tl-rail absolute -left-4 top-[14px] h-[calc(100%+4px)] w-px bg-line"
                  style={{ animationDelay: delay }}
                  aria-hidden
                />
              )}
              <span
                className={`tl-dot absolute -left-5 top-1.5 size-2 rounded-full ${
                  i === 0 ? "tl-current bg-brand-600 text-brand-600" : "bg-ink-400"
                }`}
                style={{ animationDelay: delay }}
                aria-hidden
              />
              <div className="flex items-start justify-between gap-3">
                <span data-testid="shipment-event-status" className="text-ink-900">
                  {shipmentStatusLabel(ev.status, lang)}
                </span>
                <span className="shrink-0 text-ink-500">
                  {formatOccurredAt(ev.occurred_at, lang)}
                </span>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}
