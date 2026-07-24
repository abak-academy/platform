"use client";

import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export type CatalogCategory = "all" | "book" | "course" | "exam" | "merchandise" | "medal";

export const CATALOG_CATEGORIES: { value: CatalogCategory; labelKey: string }[] = [
  { value: "all", labelKey: "catalog_tab_all" },
  { value: "book", labelKey: "catalog_tab_book" },
  { value: "course", labelKey: "catalog_tab_course" },
  { value: "exam", labelKey: "catalog_tab_competition" },
  { value: "merchandise", labelKey: "catalog_tab_merchandise" },
  { value: "medal", labelKey: "catalog_tab_medal" },
];

export interface CategoryRailProps {
  value: CatalogCategory;
  onChange: (value: CatalogCategory) => void;
}

export function CategoryRail({ value, onChange }: CategoryRailProps) {
  const { t } = useTranslation();

  return (
    <nav
      data-testid="category-rail"
      aria-label={t("catalog_category_heading" as any)}
      className="md:sticky md:top-6 md:h-fit md:w-[200px] md:shrink-0 md:self-start"
    >
      <h2 className="mb-3 hidden text-xs font-semibold uppercase tracking-wide text-ink-500 md:block">
        {t("catalog_category_heading" as any)}
      </h2>
      <ul className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-2 md:mx-0 md:flex-col md:gap-0.5 md:overflow-visible md:px-0 md:pb-0">
        {CATALOG_CATEGORIES.map((c) => {
          const active = c.value === value;
          return (
            <li key={c.value} className="shrink-0 md:shrink">
              <button
                type="button"
                onClick={() => onChange(c.value)}
                aria-current={active ? "true" : undefined}
                className={cn(
                  "whitespace-nowrap rounded-full border px-3.5 py-1.5 text-sm transition-colors md:w-full md:rounded-md md:border-0 md:px-2.5 md:text-left",
                  active
                    ? "border-brand-400 bg-brand-50 font-semibold text-brand-600 md:bg-brand-50"
                    : "border-line bg-surface text-ink-600 hover:bg-paper md:bg-transparent",
                )}
              >
                {t(c.labelKey as any)}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
