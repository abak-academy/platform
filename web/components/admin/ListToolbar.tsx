"use client";

import { Search, SlidersHorizontal, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";

export interface ListToolbarValue {
  q: string;
  filters: Record<string, string>;
  sort?: string;
}

export interface ListToolbarFilter {
  key: string;
  label: string;
  options: { value: string; label: string }[];
}

export interface ListToolbarProps {
  value: ListToolbarValue;
  onChange: (next: ListToolbarValue) => void;
  filters: ListToolbarFilter[];
  sorts?: { value: string; label: string }[];
  count: number;
  unit: string;
  variant: "card-header" | "standalone";
  searchPlaceholder?: string;
}

export function ListToolbar({ value, onChange, filters, sorts, count, unit, variant, searchPlaceholder }: ListToolbarProps) {
  const { t } = useTranslation();
  const placeholder = searchPlaceholder ?? t("list_toolbar_search");

  const active = filters.flatMap((f) => {
    const opt = f.options.find((o) => o.value === value.filters[f.key]);
    return opt ? [{ key: f.key, label: `${f.label}: ${opt.label}` }] : [];
  });

  const setFilter = (key: string, v: string) => {
    const next = { ...value.filters };
    if (v) next[key] = v;
    else delete next[key];
    onChange({ ...value, filters: next });
  };
  const clearAll = () => onChange({ ...value, filters: {} });

  return (
    <div className={`space-y-3 ${variant === "card-header" ? "border-b p-4" : ""}`}>
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
          <Input
            type="search"
            value={value.q}
            onChange={(e) => onChange({ ...value, q: e.target.value })}
            placeholder={placeholder}
            aria-label={placeholder}
            className="rounded-full pr-10 pl-9"
          />
          {value.q && (
            <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9"
              aria-label={t("list_toolbar_clear_search")} onClick={() => onChange({ ...value, q: "" })}>
              <X className="size-4" aria-hidden />
            </Button>
          )}
        </div>

        {filters.length > 0 && (
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("list_toolbar_filter")}${active.length ? ` (${active.length})` : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("list_toolbar_filter")}
                {active.length > 0 && <span className="tabular-nums">{active.length}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="admin-shell max-h-[var(--radix-popover-content-available-height)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto" aria-label={t("list_toolbar_filter")}>
              <div className="space-y-4">
                {filters.map((f) => (
                  <div key={f.key} role="group" aria-label={f.label} className="space-y-2">
                    <span className="block text-sm font-medium">{f.label}</span>
                    <div className="flex flex-wrap gap-2">
                      {[{ value: "", label: t("list_toolbar_all") }, ...f.options].map((o) => {
                        const on = (value.filters[f.key] ?? "") === o.value;
                        return (
                          <button
                            key={o.value}
                            type="button"
                            aria-pressed={on}
                            onClick={() => setFilter(f.key, o.value)}
                            className={`md-chip cursor-pointer text-[13px] transition-colors ${on ? "md-chip-primary ring-1 ring-[var(--md-sys-color-primary)]" : ""}`}
                          >
                            {o.label}
                          </button>
                        );
                      })}
                    </div>
                  </div>
                ))}
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button variant="ghost" onClick={clearAll}>{t("list_toolbar_reset")}</Button>
                  <PopoverClose asChild>
                    <Button>{t("list_toolbar_show_n").replace("{n}", String(count))}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        )}

        {sorts && sorts.length > 1 && (
          <Select value={value.sort ?? sorts[0].value} onValueChange={(sort) => onChange({ ...value, sort })}>
            <SelectTrigger className="w-60 min-w-0 flex-1 sm:flex-none" aria-label={t("list_toolbar_sort")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {sorts.map((s) => <SelectItem key={s.value} value={s.value}>{s.label}</SelectItem>)}
            </SelectContent>
          </Select>
        )}

        <span className="ml-auto text-sm font-medium tracking-wide text-ink-600 uppercase tabular-nums" aria-live="polite">
          {count} {unit}
        </span>
      </div>

      {active.length > 0 && (
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("list_toolbar_active")}>
          {active.map((c) => (
            <span key={c.key} className="md-chip md-chip-primary gap-1">
              {c.label}
              <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("list_toolbar_remove")}: ${c.label}`} onClick={() => setFilter(c.key, "")}>
                <X className="size-4" aria-hidden />
              </Button>
            </span>
          ))}
          <Button variant="ghost" onClick={clearAll}>{t("list_toolbar_clear_all")}</Button>
        </div>
      )}
    </div>
  );
}
