"use client";

import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";
import type { AdminOrderFilterStatus, AdminOrderQuery, OrderBucketCounts } from "@/lib/types";

const FILTER_OPTIONS: AdminOrderFilterStatus[] = [
  "all",
  "pending",
  "paid",
  "processing",
  "shipped",
  "shipment_failed",
  "failed",
  "refunded",
];

const SEARCH_DEBOUNCE_MS = 300;

type DateMode = "single" | "range";

export interface OrdersToolbarProps {
  value: AdminOrderQuery;
  onChange: (q: AdminOrderQuery) => void;
  counts?: OrderBucketCounts;
}

/** Only the buckets the API actually counts — the rest stay bare rather than guess. */
function chipCount(
  status: AdminOrderFilterStatus,
  counts: OrderBucketCounts | undefined,
): number | undefined {
  if (!counts) return undefined;
  switch (status) {
    case "all":
      return counts.total;
    case "pending":
      return counts.needs_confirm;
    case "paid":
      return counts.ready_to_ship;
    case "shipped":
      return counts.in_transit;
    case "shipment_failed":
      return counts.shipment_failed;
    default:
      return undefined;
  }
}

export function OrdersToolbar({ value, onChange, counts }: OrdersToolbarProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState(value.q ?? "");

  // Single day is the common lookup ("what came in on the 2nd"), so it is the
  // default. A range is still one click away, and an inbound value that spans
  // two dates opens in range mode rather than silently collapsing.
  const [dateMode, setDateMode] = useState<DateMode>(
    value.from && value.to && value.from !== value.to ? "range" : "single",
  );

  // One day is expressed as from === to. The API's upper bound is exclusive
  // (it advances `to` by a day), so that is exactly one calendar day.
  function setSingleDate(next: string) {
    const day = next || undefined;
    onChange({ ...value, from: day, to: day });
  }

  function switchMode(next: DateMode) {
    setDateMode(next);
    if (next === "single") {
      // Collapse to the start date: an open-ended range has no single day, and
      // leaving `to` behind would keep filtering a span the input cannot show.
      onChange({ ...value, to: value.from });
    }
  }

  // The debounce must not restart when the parent re-renders with a new
  // onChange identity, so the effect reads both through a ref.
  const latest = useRef({ value, onChange });
  latest.current = { value, onChange };

  useEffect(() => {
    setSearch(value.q ?? "");
  }, [value.q]);

  useEffect(() => {
    if (search === (latest.current.value.q ?? "")) return;
    const id = setTimeout(() => {
      latest.current.onChange({ ...latest.current.value, q: search || undefined });
    }, SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [search]);

  const filterLabel = (f: AdminOrderFilterStatus): string => {
    switch (f) {
      case "all":
        return t("tab_all");
      case "pending":
        return t("filter_pending");
      case "paid":
        return t("filter_paid");
      case "processing":
        return t("filter_processing");
      case "shipped":
        return t("filter_shipped");
      case "failed":
        return t("filter_failed");
      case "refunded":
        return t("filter_refunded");
      case "shipment_failed":
        return t("shipment_failed_badge");
    }
  };

  return (
    <div className="space-y-3">
      {/* One inline row. The status chips wrapped to a second line and the
          stacked date labels made that group taller than everything beside it,
          so nothing lined up. Status is a select now; the dash carries the
          "range" meaning the date labels were carrying. */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative min-w-56 flex-1">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600"
            aria-hidden
          />
          <Input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("orders_search_placeholder")}
            aria-label={t("orders_search_placeholder")}
            className="pl-9"
          />
        </div>

        <Select
          value={value.status}
          onValueChange={(v) => onChange({ ...value, status: v as AdminOrderFilterStatus })}
        >
          <SelectTrigger className="w-56" aria-label={t("th_status")} data-testid="orders-status-filter">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FILTER_OPTIONS.map((f) => {
              const count = chipCount(f, counts);
              return (
                <SelectItem key={f} value={f}>
                  <span className="flex w-full items-center justify-between gap-4">
                    <span>{filterLabel(f)}</span>
                    {count !== undefined && (
                      <span className="tabular-nums text-ink-600">{count}</span>
                    )}
                  </span>
                </SelectItem>
              );
            })}
          </SelectContent>
        </Select>

        <div className="flex items-center gap-2">
          <div className="flex items-center gap-1" role="group" aria-label={t("orders_date_single")}>
            {(["single", "range"] as const).map((m) => (
              <button
                key={m}
                type="button"
                aria-pressed={dateMode === m}
                data-testid={`orders-date-mode-${m}`}
                onClick={() => switchMode(m)}
                className={`md-chip cursor-pointer text-[13px] transition-colors ${
                  dateMode === m ? "md-chip-primary ring-1 ring-[var(--md-sys-color-primary)]" : ""
                }`}
              >
                {t(m === "single" ? "orders_date_mode_single" : "orders_date_mode_range")}
              </button>
            ))}
          </div>

          {dateMode === "single" ? (
            <>
              <label htmlFor="orders-date-single" className="sr-only">
                {t("orders_date_single")}
              </label>
              <Input
                id="orders-date-single"
                type="date"
                value={value.from ?? ""}
                onChange={(e) => setSingleDate(e.target.value)}
                className="w-auto"
              />
            </>
          ) : (
            <>
              <label htmlFor="orders-date-from" className="sr-only">
                {t("orders_date_from")}
              </label>
              <Input
                id="orders-date-from"
                type="date"
                value={value.from ?? ""}
                onChange={(e) => onChange({ ...value, from: e.target.value || undefined })}
                className="w-auto"
              />
              <span aria-hidden className="text-ink-600">
                –
              </span>
              <label htmlFor="orders-date-to" className="sr-only">
                {t("orders_date_to")}
              </label>
              <Input
                id="orders-date-to"
                type="date"
                value={value.to ?? ""}
                onChange={(e) => onChange({ ...value, to: e.target.value || undefined })}
                className="w-auto"
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
