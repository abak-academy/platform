"use client";

import { useEffect, useRef, useState } from "react";
import { Search, SlidersHorizontal, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { PAGE_SIZES, type ListParams } from "@/lib/hooks/use-list-params";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { formatAffDate } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

export interface ToolbarFilter {
  key: "status" | "type";
  label: string;
  options: { value: string; label: string }[];
}

interface ListToolbarProps {
  params: ListParams;
  onChange: (patch: Partial<ListParams>) => void;
  onReset: () => void;
  searchPlaceholder?: string;
  filters?: ToolbarFilter[];
  // Label of the date the range applies to (e.g. "Tanggal bayar"); omit for no date filter.
  dateLabel?: string;
}

const ALL = "__all__";

// One toolbar for every affiliator list: search, a filter popover (like the admin
// products page), a date range, and the page size. The state itself lives in the
// URL (useListParams); this component only renders and reports changes.
export function ListToolbar({ params, onChange, onReset, searchPlaceholder, filters = [], dateLabel }: ListToolbarProps) {
  const { t } = useTranslation();
  const [text, setText] = useState(params.q);
  const debounced = useDebouncedValue(text);
  const lastSent = useRef(params.q);

  // A search typed here is reported once the typing pauses; a change that comes
  // from the URL (reset, back button) is shown without being sent back.
  useEffect(() => {
    if (debounced !== lastSent.current) {
      lastSent.current = debounced;
      onChange({ q: debounced });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debounced]);
  useEffect(() => {
    if (params.q !== lastSent.current) {
      lastSent.current = params.q;
      setText(params.q);
    }
  }, [params.q]);

  const chips: { key: string; label: string; clear: () => void }[] = [];
  for (const filter of filters) {
    const value = params[filter.key];
    if (!value) continue;
    const option = filter.options.find((o) => o.value === value);
    chips.push({
      key: filter.key,
      label: `${filter.label}: ${option?.label ?? value}`,
      clear: () => onChange({ [filter.key]: "" }),
    });
  }
  if (dateLabel && (params.from || params.to)) {
    const range =
      params.from && params.to
        ? t("aff_filter_range").replace("{from}", formatAffDate(params.from)).replace("{to}", formatAffDate(params.to))
        : params.from
          ? t("aff_filter_range_from").replace("{from}", formatAffDate(params.from))
          : t("aff_filter_range_to").replace("{to}", formatAffDate(params.to));
    chips.push({ key: "date", label: `${dateLabel}: ${range}`, clear: () => onChange({ from: "", to: "" }) });
  }
  const hasPopover = filters.length > 0 || Boolean(dateLabel);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        {searchPlaceholder && (
          <div className="relative w-full min-w-0 sm:w-auto sm:min-w-64 sm:flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
            <Input
              type="search"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder={searchPlaceholder}
              aria-label={searchPlaceholder}
              className="pr-10 pl-9"
            />
            {text && (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute top-0 right-0 size-9"
                aria-label={t("aff_search_clear")}
                onClick={() => setText("")}
              >
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </div>
        )}

        {hasPopover && (
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("aff_filter_button")}${chips.length ? ` (${chips.length})` : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filter_button")}
                {chips.length > 0 && <span className="tabular-nums">{chips.length}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent
              align="end"
              className="admin-shell max-h-[var(--radix-popover-content-available-height)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto"
              aria-label={t("aff_filter_title")}
            >
              <div className="space-y-3">
                <h2 className="font-semibold">{t("aff_filter_title")}</h2>
                {filters.map((filter) => (
                  <div key={filter.key} className="space-y-1.5">
                    <span className="block text-sm font-medium">{filter.label}</span>
                    <Select
                      value={params[filter.key] || ALL}
                      onValueChange={(v) => onChange({ [filter.key]: v === ALL ? "" : v })}
                    >
                      <SelectTrigger className="w-full" aria-label={filter.label}>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={ALL}>{t("aff_filter_all")}</SelectItem>
                        {filter.options.map((o) => (
                          <SelectItem key={o.value} value={o.value}>
                            {o.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ))}
                {dateLabel && (
                  <div className="space-y-1.5">
                    <span className="block text-sm font-medium">{dateLabel}</span>
                    <div className="grid grid-cols-2 gap-2">
                      <label className="space-y-1 text-xs text-ink-600">
                        {t("aff_filter_date_from")}
                        <Input type="date" value={params.from} max={params.to || undefined} onChange={(e) => onChange({ from: e.target.value })} />
                      </label>
                      <label className="space-y-1 text-xs text-ink-600">
                        {t("aff_filter_date_to")}
                        <Input type="date" value={params.to} min={params.from || undefined} onChange={(e) => onChange({ to: e.target.value })} />
                      </label>
                    </div>
                  </div>
                )}
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button variant="ghost" onClick={onReset}>
                    {t("aff_filter_reset")}
                  </Button>
                  <PopoverClose asChild>
                    <Button>{t("aff_filter_show")}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        )}

        <Select value={String(params.per_page)} onValueChange={(v) => onChange({ per_page: Number(v) })}>
          <SelectTrigger className="ml-auto w-40 min-w-0" aria-label={t("aff_per_page")}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {PAGE_SIZES.map((n) => (
              <SelectItem key={n} value={String(n)}>
                {t("aff_per_page_option").replace("{n}", String(n))}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_filter_active")}>
          {chips.map((chip) => (
            <span key={chip.key} className="md-chip md-chip-primary gap-1">
              {chip.label}
              <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_filter_remove")}: ${chip.label}`} onClick={chip.clear}>
                <X className="size-4" aria-hidden />
              </Button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
