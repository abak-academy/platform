"use client";

import { useState } from "react";
import { Copy, Check } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { useTranslation } from "@/lib/i18n";
import { isShipmentFailure, shipmentStatusLabel } from "@/lib/shipment-status";
import type { OrderTracking, OrderTrackingEntry } from "@/lib/types";

export interface TrackingModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tracking?: OrderTracking | null;
  isLoading: boolean;
  error?: string | null;
}

function formatCheckpoint(iso: string, lang: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return new Intl.DateTimeFormat(lang === "id" ? "id-ID" : "en-GB", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(d);
}

/**
 * The parcel's journey, in our own dialog rather than a hand-off to the
 * courier's site.
 *
 * It opens with the waybill set as a shipping label would print it — the
 * number on the box is the thing staff and the caller on the phone are both
 * looking at, so it is the first thing here too.
 */
export function TrackingModal({
  open,
  onOpenChange,
  tracking,
  isLoading,
  error,
}: TrackingModalProps) {
  const { t, lang } = useTranslation();
  const [copied, setCopied] = useState(false);

  const waybill = tracking?.waybill ?? "";
  const history = tracking?.history ?? [];
  const failed = isShipmentFailure(tracking?.status);

  async function copyWaybill() {
    try {
      await navigator.clipboard.writeText(waybill);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard is blocked in some embedded browsers; the number is on
      // screen and selectable, so there is nothing to recover from.
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="gap-0 overflow-hidden p-0 sm:max-w-md [&>button[data-slot=dialog-close]]:top-5 [&>button[data-slot=dialog-close]]:text-white/70 [&>button[data-slot=dialog-close]]:hover:text-white">
        {/* The label band: this is the sticker on the box, quoted.

            The navy is a literal rather than bg-ink-900 on purpose. Every ink
            token inverts in dark mode, which turned this band near-white with
            white type on it — 1.05:1, unreadable. Printed ink does not invert
            with the UI theme, and neither does this. */}
        <DialogHeader className="space-y-0 bg-[#15183a] px-6 py-5 text-left">
          <span className="inline-flex w-fit rounded-full border border-white/25 px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-white/90">
            {[tracking?.courier, tracking?.service].filter(Boolean).join(" · ") ||
              t("shipment_track_title")}
          </span>

          <div className="flex items-end justify-between gap-3 pt-3">
            <DialogTitle className="font-mono text-[19px] leading-tight font-semibold tracking-[0.14em] break-all text-white">
              {waybill || "—"}
            </DialogTitle>
            {waybill && (
              <button
                type="button"
                onClick={copyWaybill}
                data-testid="tracking-copy"
                className="mb-0.5 flex shrink-0 items-center gap-1.5 rounded-full px-2 py-1 text-[11px] font-medium text-white/70 transition-colors hover:bg-white/10 hover:text-white focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white/60"
              >
                {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
                {copied ? t("action_copied") : t("action_copy")}
              </button>
            )}
          </div>
          <DialogDescription className="sr-only">
            {t("shipment_track_title")}
          </DialogDescription>
        </DialogHeader>

        {isLoading && (
          <div className="space-y-3 px-6 py-5">
            <Skeleton className="h-5 w-40" />
            <Skeleton className="h-3 w-56" />
            <Skeleton className="h-3 w-48" />
          </div>
        )}

        {!isLoading && error && (
          <div role="alert" className="px-6 py-5 text-sm text-danger">
            {error}
          </div>
        )}

        {!isLoading && !error && tracking && (
          <>
            <div className="border-b border-line px-6 py-4">
              <div className="text-[11px] font-semibold uppercase tracking-[0.08em] text-ink-600">
                {t("shipment_track_current")}
              </div>
              <div
                data-testid="tracking-current-status"
                className={`pt-1 text-lg leading-snug font-semibold ${failed ? "text-danger" : "text-ink-900"}`}
              >
                {tracking.status
                  ? shipmentStatusLabel(tracking.status, lang)
                  : t("order_shipment_status_unknown")}
              </div>
              {/* Says whose log this is. A thin local history must never read
                  as though the courier reported only that much. */}
              <div className="pt-0.5 text-xs text-ink-600">
                {tracking.source === "courier"
                  ? t("shipment_track_source_courier")
                  : t("shipment_track_source_local")}
              </div>
            </div>

            {history.length === 0 ? (
              <p className="px-6 py-8 text-center text-sm text-ink-600">
                {t("shipment_track_empty")}
              </p>
            ) : (
              <ol
                data-testid="tracking-history"
                className="max-h-[42vh] overflow-y-auto px-6 py-5"
              >
                {history.map((entry, i) => (
                  <Checkpoint
                    key={`${entry.status}-${entry.occurred_at}-${i}`}
                    entry={entry}
                    index={i}
                    isLatest={i === 0}
                    isLast={i === history.length - 1}
                    failed={failed && i === 0}
                    lang={lang}
                  />
                ))}
              </ol>
            )}
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}

function Checkpoint({
  entry,
  index,
  isLatest,
  isLast,
  failed,
  lang,
}: {
  entry: OrderTrackingEntry;
  index: number;
  isLatest: boolean;
  isLast: boolean;
  failed: boolean;
  lang: string;
}) {
  const { t } = useTranslation();
  const dotTone = failed ? "bg-danger" : isLatest ? "bg-brand-600" : "bg-transparent";
  const ringTone = failed ? "border-danger" : isLatest ? "border-brand-600" : "border-ink-400";

  return (
    <li
      className="relative grid grid-cols-[11px_1fr] gap-x-3 pb-5 last:pb-0 animate-in fade-in slide-in-from-bottom-1 motion-reduce:animate-none"
      // The reveal runs newest to oldest, so the eye lands on where the
      // parcel is now before it reads how it got there. Capped so a long log
      // does not keep animating after the reader has started reading.
      style={{ animationDelay: `${Math.min(index, 6) * 45}ms` }}
    >
      {/* The rail stops at the last checkpoint rather than trailing into
          nothing — the journey is only as long as what the courier scanned. */}
      {!isLast && <span aria-hidden className="absolute top-4 bottom-0 left-[5px] w-px bg-line" />}
      <span
        aria-hidden
        className={`mt-1 size-[11px] rounded-full border-2 ${ringTone} ${dotTone}`}
      />
      <div className="min-w-0">
        <div className="text-sm leading-snug font-medium text-ink-900">
          {shipmentStatusLabel(entry.status, lang as "id" | "en")}
        </div>
        {entry.note && <div className="pt-0.5 text-xs text-ink-600">{entry.note}</div>}
        {entry.driver_name && (
          <div className="pt-0.5 text-xs text-ink-600">
            {t("shipment_track_driver")}: {entry.driver_name}
          </div>
        )}
        <div className="pt-1 text-[11px] tabular-nums text-ink-600">
          {formatCheckpoint(entry.occurred_at, lang)}
        </div>
      </div>
    </li>
  );
}
