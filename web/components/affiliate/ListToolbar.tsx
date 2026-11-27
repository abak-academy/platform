"use client";

import { useEffect, useRef, useState } from "react";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { ActiveChip, FILTER_PANEL_CLASS, FilterGrid, FilterPanelFooter, FilterSelect, ToolbarFilterButton, ToolbarSearch, ToolbarSort } from "@/components/admin/ListToolbar";
import { PAGE_SIZES, type ListParams } from "@/lib/hooks/use-list-params";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { formatAffDate } from "@/lib/affiliate-labels";
import { useTranslation } from "@/lib/i18n";

export interface ToolbarFilter {
  key: "status" | "type" | "school";
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
  // Drawn as the top band of a table card instead of its own card.
  embedded?: boolean;
}

// One toolbar for every affiliator list: search, a filter popover (like the admin
// products page), a date range, and the page size. The state itself lives in the
// URL (useListParams); this component only renders and reports changes.
export function ListToolbar({ params, onChange, onReset, searchPlaceholder, filters = [], dateLabel, embedded }: ListToolbarProps) {
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
    <div
      className={
        embedded
          ? "flex flex-col gap-2.5 border-b border-line px-4 py-3.5"
          : "flex flex-col gap-2.5 rounded-[var(--radius-card)] border border-line bg-surface px-3.5 py-3 shadow-card"
      }
    >
      <div className="flex flex-wrap items-center gap-2.5">
        {searchPlaceholder && (
          <ToolbarSearch value={text} onChange={setText} placeholder={searchPlaceholder} clearLabel={t("aff_search_clear")} />
        )}

        {hasPopover && (
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarFilterButton label={t("aff_filter_button")} count={chips.length} />
            </PopoverTrigger>
            <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("aff_filter_title")}>
              <div className="flex flex-col gap-3.5">
                <FilterGrid>
                  {filters.map((filter) => (
                    <FilterSelect
                      key={filter.key}
                      label={filter.label}
                      value={params[filter.key] ?? ""}
                      options={filter.options}
                      onChange={(v) => onChange({ [filter.key]: v })}
                    />
                  ))}
                  {dateLabel && (
                    <div className="space-y-1.5 sm:col-span-2">
                      <span className="block font-mono text-[11px] font-semibold tracking-[.06em] text-ink-600 uppercase">{dateLabel}</span>
                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
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
                </FilterGrid>
                <FilterPanelFooter onReset={onReset} resetLabel={t("aff_filter_reset")} showLabel={t("aff_filter_show")} />
              </div>
            </PopoverContent>
          </Popover>
        )}

        <div className="ml-auto">
          <ToolbarSort
            value={String(params.per_page)}
            onChange={(v) => onChange({ per_page: Number(v) })}
            options={PAGE_SIZES.map((n) => ({ value: String(n), label: t("aff_per_page_option").replace("{n}", String(n)) }))}
            label={t("aff_per_page")}
          />
        </div>
      </div>

      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t("aff_filter_active")}>
          {chips.map((chip) => (
            <ActiveChip key={chip.key} label={chip.label} removeLabel={`${t("aff_filter_remove")}: ${chip.label}`} onRemove={chip.clear} />
          ))}
        </div>
      )}
    </div>
  );
}
