"use client";

import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import type { AdminOrderFilterStatus, AdminOrderQueue, AdminOrderQuery, OrderBucketCounts, ProductType, AdminOrderSort } from "@/lib/types";

// One flat dropdown mixing real statuses and queues — the admin picks a
// filter, not a filter *kind*. The two concepts stay split in the query
// object underneath (value.status vs value.queue); this list is just what's
// offered on screen.
type FilterOption = AdminOrderFilterStatus | AdminOrderQueue;

const FILTER_OPTIONS: FilterOption[] = [
  "all",
  "pending",
  "ready_to_ship",
  "shipment_failed",
  "in_transit",
  "completed",
  "payment_expired",
  "cancelled",
];

type DateMode = "single" | "range";

export interface OrdersToolbarProps {
  value: AdminOrderQuery;
  onChange: (q: AdminOrderQuery) => void;
  counts?: OrderBucketCounts;
  couriers?: string[];
}

export function chipCount(
  status: FilterOption,
  counts: OrderBucketCounts | undefined,
): number | undefined {
  if (!counts) return undefined;
  switch (status) {
    case "all":
      return counts.total;
    case "pending":
      return counts.needs_confirm;
    case "ready_to_ship":
      return counts.ready_to_ship;
    case "in_transit":
      return counts.in_transit;
    case "completed":
      return counts.completed;
    case "payment_expired":
      return counts.payment_expired;
    case "cancelled":
      return counts.cancelled;
    case "shipment_failed":
      return counts.shipment_failed;
    default:
      return undefined;
  }
}

const QUEUE_VALUES: AdminOrderQueue[] = ["ready_to_ship", "shipment_failed", "in_transit"];

function isQueueOption(option: FilterOption): option is AdminOrderQueue {
  return (QUEUE_VALUES as string[]).includes(option);
}

export function OrdersToolbar({ value, onChange, counts, couriers = [] }: OrdersToolbarProps) {
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

  const debouncedSearch = useDebouncedValue(search);

  useEffect(() => {
    if (debouncedSearch === (latest.current.value.q ?? "")) return;
    latest.current.onChange({ ...latest.current.value, q: debouncedSearch || undefined });
  }, [debouncedSearch]);

  // The selected option is whichever dimension is currently set — the two
  // are mutually exclusive on the query, but the dropdown only shows one.
  const selected: FilterOption = value.queue ?? value.status;

  function selectFilter(next: FilterOption) {
    if (isQueueOption(next)) {
      onChange({ ...value, status: "all", queue: next });
    } else {
      onChange({ ...value, status: next, queue: undefined });
    }
  }

  const filterLabel = (f: FilterOption): string => {
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
      case "in_transit":
        return t("filter_in_transit");
      case "completed":
        return t("filter_completed");
      case "payment_expired":
        return t("filter_payment_expired");
      case "cancelled":
        return t("filter_cancelled");
      case "shipment_failed":
        return t("shipment_failed_badge");
      case "ready_to_ship":
        // Reuses the dashboard/store card's label — same bucket, same wording.
        return t("admin_home_ready_to_ship");
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
          value={selected}
          onValueChange={(v) => selectFilter(v as FilterOption)}
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

        <Select value={value.type ?? "all"} onValueChange={(v) => onChange({ ...value, type: v === "all" ? undefined : v as ProductType })}>
          <SelectTrigger className="w-40" aria-label={t("orders_product_type")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("orders_all_types")}</SelectItem>
            {(["book", "merchandise", "medal", "course", "exam"] as const).map((type) => (
              <SelectItem key={type} value={type}>{t(`product_type_${type}`)}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={value.courier ?? "all"} onValueChange={(v) => onChange({ ...value, courier: v === "all" ? undefined : v })}>
          <SelectTrigger className="w-40" aria-label={t("orders_courier")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("orders_all_couriers")}</SelectItem>
            {couriers.map((code) => <SelectItem key={code} value={code}>{code.toUpperCase()}</SelectItem>)}
          </SelectContent>
        </Select>

        <div className="flex items-center gap-1">
          <Select value={value.sort ?? "order_at"} onValueChange={(v) => onChange({ ...value, sort: v as AdminOrderSort })}>
            <SelectTrigger className="w-44" aria-label={t("orders_sort")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {(["order_at", "paid_at", "item_qty"] as const).map((sort) => (
                <SelectItem key={sort} value={sort}>{t(`orders_sort_${sort}`)}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" size="sm" aria-label={`${t("orders_sort_direction")}: ${t(value.dir === "asc" ? "orders_sort_asc" : "orders_sort_desc")}`}
            onClick={() => onChange({ ...value, dir: value.dir === "asc" ? "desc" : "asc" })}>
            {value.dir === "asc" ? "↑" : "↓"} {t(value.dir === "asc" ? "orders_sort_asc" : "orders_sort_desc")}
          </Button>
        </div>

        <Select value={String(value.limit ?? 20)} onValueChange={(v) => onChange({ ...value, limit: Number(v) as 20 | 50 | 100 })}>
          <SelectTrigger className="w-24" aria-label={t("orders_page_size")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {[20, 50, 100].map((limit) => <SelectItem key={limit} value={String(limit)}>{limit}</SelectItem>)}
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
