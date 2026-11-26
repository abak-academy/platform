"use client";

import { BookOpen, LayoutGrid, Medal, PlayCircle, Shirt, type LucideIcon } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export type CatalogCategory = "all" | "book" | "course" | "merchandise" | "medal";

export const CATALOG_CATEGORIES: { value: CatalogCategory; labelKey: string; icon: LucideIcon }[] = [
  { value: "all", labelKey: "catalog_tab_all", icon: LayoutGrid },
  { value: "book", labelKey: "catalog_tab_book", icon: BookOpen },
  { value: "course", labelKey: "catalog_tab_course", icon: PlayCircle },
  { value: "merchandise", labelKey: "catalog_tab_merchandise", icon: Shirt },
  { value: "medal", labelKey: "catalog_tab_medal", icon: Medal },
];

export interface CategoryRailProps {
  value: CatalogCategory;
  onChange: (value: CatalogCategory) => void;
  counts?: Partial<Record<CatalogCategory, number>>;
}

export function CategoryRail({ value, onChange, counts }: CategoryRailProps) {
  const { t } = useTranslation();

  return (
    <div
      data-testid="category-rail"
      role="group"
      aria-label={t("catalog_category_heading" as any)}
      className="-mx-0.5 flex gap-2 overflow-x-auto p-0.5 [scrollbar-width:none]"
    >
      {CATALOG_CATEGORIES.map((c) => {
        const active = c.value === value;
        const Icon = c.icon;
        return (
          <button
            key={c.value}
            type="button"
            aria-pressed={active}
            onClick={() => onChange(c.value)}
            className={cn(
              "type-label inline-flex h-11 shrink-0 items-center gap-2 whitespace-nowrap rounded-[var(--radius-pill)] border-[1.5px] px-4 transition-transform duration-300 hover:-translate-y-0.5",
              active ? "border-brand-600 bg-brand-600 text-white" : "border-line bg-paper text-ink-900",
            )}
          >
            <Icon className="size-4" aria-hidden="true" />
            {t(c.labelKey as any)}
            {counts?.[c.value] !== undefined && (
              <span
                className={cn(
                  "rounded-[var(--radius-pill)] px-[7px] py-0.5 text-[11px]",
                  active ? "bg-white/20 text-white" : "bg-surface-sunken text-ink-600",
                )}
              >
                {counts[c.value]}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
