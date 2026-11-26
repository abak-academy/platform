"use client";

import { Filter, Search, X } from "lucide-react";
import * as React from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

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

export function ToolbarSearch({
  value,
  onChange,
  placeholder,
  clearLabel,
  className,
}: {
  value: string;
  onChange: (q: string) => void;
  placeholder: string;
  clearLabel: string;
  className?: string;
}) {
  return (
    <div className={cn("relative w-full min-w-0 sm:w-80", className)}>
      <Search className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
      <Input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={placeholder}
        className="h-[var(--spacing-control-md)] rounded-full pr-9 pl-9 text-[15px]"
      />
      {value && (
        <button
          type="button"
          aria-label={clearLabel}
          onClick={() => onChange("")}
          className="absolute top-1/2 right-3 grid -translate-y-1/2 place-items-center p-0.5 text-ink-600 hover:text-ink-900"
        >
          <X className="size-4" aria-hidden />
        </button>
      )}
    </div>
  );
}

export const ToolbarFilterButton = React.forwardRef<
  HTMLButtonElement,
  Omit<React.ComponentProps<typeof Button>, "children"> & { label: string; count?: number }
>(({ label, count = 0, className, ...props }, ref) => (
  <Button
    ref={ref}
    variant="outline"
    className={cn("data-[state=open]:border-brand-600 data-[state=open]:text-brand-ink", className)}
    aria-label={`${label}${count ? ` (${count})` : ""}`}
    {...props}
  >
    <Filter className="size-4" aria-hidden />
    {label}
    {count > 0 && (
      <span className="grid h-5 min-w-5 place-items-center rounded-full bg-brand-600 px-1.5 text-[11px] leading-5 font-extrabold text-on-brand tabular-nums">
        {count}
      </span>
    )}
  </Button>
));
ToolbarFilterButton.displayName = "ToolbarFilterButton";

export const FILTER_PANEL_CLASS =
  "admin-shell fpop-sheet w-[min(420px,90vw)] max-h-[var(--radix-popover-content-available-height)] overflow-y-auto";

const ALL = "__all__";

export function FilterGrid({ children, className }: { children: React.ReactNode; className?: string }) {
  return <div className={cn("grid grid-cols-1 gap-3 sm:grid-cols-2", className)}>{children}</div>;
}

export function FilterSelect({
  label,
  value,
  options,
  onChange,
  wide,
  placeholder,
}: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (v: string) => void;
  wide?: boolean;
  placeholder?: string;
}) {
  const { t } = useTranslation();
  return (
    <div className={cn("flex min-w-0 flex-col gap-1.5", wide && "sm:col-span-2")}>
      <span className="font-mono text-[11px] font-semibold tracking-[.06em] text-ink-600 uppercase">{label}</span>
      <Select value={value || ALL} onValueChange={(v) => onChange(v === ALL ? "" : v)}>
        <SelectTrigger aria-label={label}>
          <SelectValue placeholder={placeholder} />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>{t("list_toolbar_all")}</SelectItem>
          {options.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
        </SelectContent>
      </Select>
    </div>
  );
}

export function FilterPanelFooter({ onReset, showLabel, resetLabel }: { onReset: () => void; showLabel: string; resetLabel?: string }) {
  const { t } = useTranslation();
  return (
    <div className="flex flex-wrap items-center justify-between gap-2 border-t border-dashed border-line pt-3">
      <Button variant="ghost" size="sm" onClick={onReset}>{resetLabel ?? t("list_toolbar_reset")}</Button>
      <PopoverClose asChild>
        <Button variant="dark" size="sm">{showLabel}</Button>
      </PopoverClose>
    </div>
  );
}

export function ToolbarSort({
  value,
  onChange,
  options,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  options: { value: string; label: string }[];
  label: string;
}) {
  return (
    <Select value={value} onValueChange={onChange}>
      <SelectTrigger size="pill" className="h-[var(--spacing-control-md)] min-w-0 flex-1 sm:flex-none" aria-label={label}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((o) => <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>)}
      </SelectContent>
    </Select>
  );
}

export function ToolbarCount({ children }: { children: React.ReactNode }) {
  return (
    <span className="ml-auto font-mono text-xs font-semibold tracking-[.06em] text-ink-600 uppercase tabular-nums max-narrow:ml-0" aria-live="polite">
      {children}
    </span>
  );
}

export function FilterChip({ on, className, ...props }: React.ComponentProps<"button"> & { on: boolean }) {
  return <button type="button" aria-pressed={on} className={cn("md-fchip", on && "on", className)} {...props} />;
}

export function ActiveChip({ label, removeLabel, onRemove }: { label: string; removeLabel: string; onRemove: () => void }) {
  return (
    <span className="md-lchip">
      {label}
      <button
        type="button"
        aria-label={removeLabel}
        onClick={onRemove}
        className="grid size-[22px] place-items-center rounded-full hover:bg-black/5"
      >
        <X className="size-3.5" aria-hidden />
      </button>
    </span>
  );
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
    <div
      className={cn(
        "flex flex-col gap-2.5",
        variant === "card-header"
          ? "border-b border-line px-4 py-3.5"
          : "rounded-[var(--radius-card)] border border-line bg-surface px-3.5 py-3 shadow-card"
      )}
    >
      <div className="flex flex-wrap items-center gap-2.5">
        <ToolbarSearch
          value={value.q}
          onChange={(q) => onChange({ ...value, q })}
          placeholder={placeholder}
          clearLabel={t("list_toolbar_clear_search")}
        />

        {filters.length > 0 && (
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarFilterButton label={t("list_toolbar_filter")} count={active.length} />
            </PopoverTrigger>
            <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("list_toolbar_filter")}>
              <div className="flex flex-col gap-3.5">
                <FilterGrid>
                  {filters.map((f) => (
                    <FilterSelect key={f.key} label={f.label} value={value.filters[f.key] ?? ""} options={f.options} onChange={(v) => setFilter(f.key, v)} />
                  ))}
                </FilterGrid>
                <FilterPanelFooter onReset={clearAll} showLabel={t("list_toolbar_show_n").replace("{n}", String(count))} />
              </div>
            </PopoverContent>
          </Popover>
        )}

        {sorts && sorts.length > 1 && (
          <ToolbarSort
            value={value.sort ?? sorts[0].value}
            onChange={(sort) => onChange({ ...value, sort })}
            options={sorts}
            label={t("list_toolbar_sort")}
          />
        )}

        <ToolbarCount>{count} {unit}</ToolbarCount>
      </div>

      {active.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t("list_toolbar_active")}>
          {active.map((c) => (
            <ActiveChip key={c.key} label={c.label} removeLabel={`${t("list_toolbar_remove")}: ${c.label}`} onRemove={() => setFilter(c.key, "")} />
          ))}
          <Button variant="ghost" size="sm" onClick={clearAll}>{t("list_toolbar_clear_all")}</Button>
        </div>
      )}
    </div>
  );
}
