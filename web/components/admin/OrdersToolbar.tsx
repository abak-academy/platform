"use client";

import { useEffect, useRef, useState } from "react";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
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

export interface OrdersToolbarProps {
  value: AdminOrderQuery;
  onChange: (q: AdminOrderQuery) => void;
  counts?: OrderBucketCounts;
}

/** Only the buckets the API actually counts — the rest of the chips stay bare rather than guess. */
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
      <div className="flex flex-wrap items-end gap-3">
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

        <div className="flex items-end gap-2">
          <div className="space-y-1">
            <label
              htmlFor="orders-date-from"
              className="block text-[11px] font-medium tracking-wide text-ink-600 uppercase"
            >
              {t("orders_date_from")}
            </label>
            <Input
              id="orders-date-from"
              type="date"
              value={value.from ?? ""}
              onChange={(e) => onChange({ ...value, from: e.target.value || undefined })}
              className="w-auto"
            />
          </div>
          <div className="space-y-1">
            <label
              htmlFor="orders-date-to"
              className="block text-[11px] font-medium tracking-wide text-ink-600 uppercase"
            >
              {t("orders_date_to")}
            </label>
            <Input
              id="orders-date-to"
              type="date"
              value={value.to ?? ""}
              onChange={(e) => onChange({ ...value, to: e.target.value || undefined })}
              className="w-auto"
            />
          </div>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {FILTER_OPTIONS.map((f) => {
          const count = chipCount(f, counts);
          const active = value.status === f;
          return (
            <button
              key={f}
              type="button"
              aria-pressed={active}
              className={active ? "md-btn-filled" : "md-btn-outlined"}
              onClick={() => onChange({ ...value, status: f })}
            >
              {filterLabel(f)}
              {count !== undefined && (
                <span className="ml-1.5 tabular-nums opacity-70">{count}</span>
              )}
            </button>
          );
        })}
      </div>
    </div>
  );
}
