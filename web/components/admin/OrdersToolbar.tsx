"use client";

import { useEffect, useRef, useState } from "react";
import { ActiveChip, FILTER_PANEL_CLASS, FilterGrid, FilterPanelFooter, FilterSelect, ToolbarFilterButton, ToolbarSearch, ToolbarSort } from "@/components/admin/ListToolbar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
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
  "shipped",
  "completed",
  "payment_expired",
  "cancelled",
];

type DateMode = "single" | "range";

export interface OrdersToolbarProps {
  value: AdminOrderQuery;
  onChange: (q: AdminOrderQuery) => void;
  couriers?: string[];
  count?: number;
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
    case "shipped":
      return counts.shipped;
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

export function OrdersToolbar({ value, onChange, couriers = [], count }: OrdersToolbarProps) {
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

  // A deep-linked value (paid, in_transit...) has no entry in the offered list; keep it so the trigger can show it.
  const statusOptions = [...FILTER_OPTIONS.filter((f) => f !== "all"), ...(selected !== "all" && !FILTER_OPTIONS.includes(selected) ? [selected] : [])]
    .map((f) => ({ value: f, label: filterLabel(f) }));

  const chips = [
    ...(selected !== "all" ? [{ key: "status", label: `${t("th_status")}: ${filterLabel(selected)}`, clear: () => onChange({ ...value, status: "all", queue: undefined }) }] : []),
    ...(value.type ? [{ key: "type", label: `${t("orders_product_type")}: ${t(`product_type_${value.type}`)}`, clear: () => onChange({ ...value, type: undefined }) }] : []),
    ...(value.courier ? [{ key: "courier", label: `${t("orders_courier")}: ${value.courier.toUpperCase()}`, clear: () => onChange({ ...value, courier: undefined }) }] : []),
    ...(value.from || value.to ? [{
      key: "date",
      label: `${t("orders_date_single")}: ${value.from && value.from === value.to ? value.from : `${value.from ?? "…"} – ${value.to ?? "…"}`}`,
      clear: () => onChange({ ...value, from: undefined, to: undefined }),
    }] : []),
  ];

  return (
    <div className="flex flex-col gap-2.5 border-b border-line px-4 py-3.5">
      <div className="flex flex-wrap items-center gap-2.5">
        <ToolbarSearch value={search} onChange={setSearch} placeholder={t("orders_search_placeholder")} clearLabel={t("orders_clear_search")} />

        <Popover>
          <PopoverTrigger asChild>
            <ToolbarFilterButton label={t("orders_filters")} count={chips.length} />
          </PopoverTrigger>
          <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("orders_filter_orders")}>
            <div className="flex flex-col gap-3.5">
              <h2 className="sr-only">{t("orders_filter_orders")}</h2>
              <FilterGrid>
                <div data-testid="orders-status-filter" className="contents">
                  <FilterSelect
                    label={t("th_status")}
                    value={selected === "all" ? "" : selected}
                    options={statusOptions}
                    onChange={(v) => selectFilter((v || "all") as FilterOption)}
                  />
                </div>
                <FilterSelect
                  label={t("orders_product_type")}
                  value={value.type ?? ""}
                  options={(["book", "merchandise", "medal", "course", "exam"] as const).map((type) => ({ value: type, label: t(`product_type_${type}`) }))}
                  onChange={(v) => onChange({ ...value, type: (v || undefined) as ProductType | undefined })}
                />
                <FilterSelect
                  label={t("orders_courier")}
                  value={value.courier ?? ""}
                  options={couriers.map((code) => ({ value: code, label: code.toUpperCase() }))}
                  onChange={(v) => onChange({ ...value, courier: v || undefined })}
                />
                <FilterSelect
                  label={t("orders_date_single")}
                  value={dateMode}
                  allowAll={false}
                  options={(["single", "range"] as const).map((m) => ({ value: m, label: t(m === "single" ? "orders_date_mode_single" : "orders_date_mode_range") }))}
                  onChange={(v) => v && switchMode(v as DateMode)}
                />
                <div className="flex min-w-0 items-center gap-2 sm:col-span-2">
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
                        className="min-w-0 flex-1"
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
                        className="min-w-0 flex-1"
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
                        className="min-w-0 flex-1"
                      />
                    </>
                  )}
                </div>
              </FilterGrid>
              <FilterPanelFooter
                onReset={() => onChange({ ...value, status: "all", queue: undefined, type: undefined, courier: undefined, from: undefined, to: undefined })}
                resetLabel={t("orders_reset_filters")}
                showLabel={count === undefined ? t("orders_show_orders") : t("list_toolbar_show_n").replace("{n}", String(count))}
              />
            </div>
          </PopoverContent>
        </Popover>

        <ToolbarSort
          value={`${value.sort ?? "order_at"}:${value.dir ?? "desc"}`}
          onChange={(v) => {
            const [sort, dir] = v.split(":") as [AdminOrderSort, "asc" | "desc"];
            onChange({ ...value, sort, dir });
          }}
          label={t("orders_sort")}
          options={(["order_at", "paid_at", "item_qty"] as const).flatMap((sort) =>
            (["desc", "asc"] as const).map((dir) => ({ value: `${sort}:${dir}`, label: t(`orders_sort_${sort}_${dir}`) })),
          )}
        />
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t("orders_active_filters")}>
          {chips.map((chip) => (
            <ActiveChip key={chip.key} label={chip.label} removeLabel={`${t("orders_remove_filter")}: ${chip.label}`} onRemove={chip.clear} />
          ))}
        </div>
      )}
    </div>
  );
}
