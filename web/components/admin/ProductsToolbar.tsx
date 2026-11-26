"use client";

import { ActiveChip, FILTER_PANEL_CLASS, FilterGrid, FilterPanelFooter, FilterSelect, ToolbarFilterButton, ToolbarSearch, ToolbarSort } from "@/components/admin/ListToolbar";
import { Popover, PopoverTrigger, PopoverContent } from "@/components/ui/popover";
import { useTranslation } from "@/lib/i18n";
import type { Product, ProductType, ProductStatus } from "@/lib/types";

export type ProductsSort = "default" | "name_asc" | "name_desc" | "price_asc" | "price_desc" | "stock_asc" | "stock_desc";

export interface ProductsQuery {
  q?: string;
  type?: ProductType;
  status?: ProductStatus;
  sort: ProductsSort;
  limit: 20 | 50 | 100;
}

export const DEFAULT_PRODUCTS_QUERY: ProductsQuery = { sort: "default", limit: 20 };

export function productStock(product: Product): number | undefined {
  if (product.type === "book" || product.type === "merchandise" || product.type === "medal") return product.stock;
  if (product.type === "exam" && product.stock_limited) return product.stock ?? 0;
  return undefined;
}

export function applyProductsQuery(products: Product[], query: ProductsQuery): Product[] {
  const search = query.q?.trim().toLowerCase() ?? "";
  const rows = products.filter((product) =>
    product.name.toLowerCase().includes(search) &&
    (!query.type || product.type === query.type) &&
    (!query.status || (product.status ?? "draft") === query.status),
  );
  const direction = query.sort.endsWith("desc") ? -1 : 1;
  return rows.sort((a, b) => {
    switch (query.sort) {
      case "name_asc":
      case "name_desc":
        return direction * a.name.localeCompare(b.name, undefined, { sensitivity: "base" });
      case "price_asc":
      case "price_desc":
        return direction * (a.price - b.price);
      case "stock_asc":
      case "stock_desc": {
        const left = productStock(a);
        const right = productStock(b);
        if (left === undefined) return right === undefined ? 0 : 1;
        if (right === undefined) return -1;
        return direction * (left - right);
      }
      default:
        return 0;
    }
  });
}

const SORTS: ProductsSort[] = ["default", "name_asc", "name_desc", "price_asc", "price_desc", "stock_asc", "stock_desc"];
const STATUSES: ProductStatus[] = ["draft", "published", "hidden", "archived"];

export function ProductsToolbar({ value, onChange, types, count }: {
  value: ProductsQuery;
  onChange: (q: ProductsQuery) => void;
  types: ProductType[];
  count?: number;
}) {
  const { t } = useTranslation();
  const chips = [
    ...(value.type ? [{ key: "type", label: `${t("products_type")}: ${t(`product_type_${value.type}`)}`, clear: () => onChange({ ...value, type: undefined }) }] : []),
    ...(value.status ? [{ key: "status", label: `${t("th_status")}: ${t(`products_status_${value.status}`)}`, clear: () => onChange({ ...value, status: undefined }) }] : []),
  ];

  return (
    <div className="flex flex-col gap-2.5 border-b border-line px-4 py-3.5">
      <div className="flex flex-wrap items-center gap-2.5">
        <ToolbarSearch value={value.q ?? ""} onChange={(q) => onChange({ ...value, q: q || undefined })}
          placeholder={t("products_search_placeholder")} clearLabel={t("products_clear_search")} />
        <Popover>
          <PopoverTrigger asChild>
            <ToolbarFilterButton label={t("products_filters")} count={chips.length} />
          </PopoverTrigger>
          <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("products_filter_products")}>
            <div className="flex flex-col gap-3.5">
              <h2 className="sr-only">{t("products_filter_products")}</h2>
              <FilterGrid>
                <FilterSelect
                  label={t("products_type")}
                  value={value.type ?? ""}
                  options={types.map((type) => ({ value: type, label: t(`product_type_${type}`) }))}
                  onChange={(v) => onChange({ ...value, type: (v || undefined) as ProductType | undefined })}
                />
                <FilterSelect
                  label={t("th_status")}
                  value={value.status ?? ""}
                  options={STATUSES.map((status) => ({ value: status, label: t(`products_status_${status}`) }))}
                  onChange={(v) => onChange({ ...value, status: (v || undefined) as ProductStatus | undefined })}
                />
              </FilterGrid>
              <FilterPanelFooter
                onReset={() => onChange({ ...value, type: undefined, status: undefined })}
                resetLabel={t("products_reset_filters")}
                showLabel={count === undefined ? t("products_show_products") : t("list_toolbar_show_n").replace("{n}", String(count))}
              />
            </div>
          </PopoverContent>
        </Popover>
        <ToolbarSort value={value.sort} onChange={(sort) => onChange({ ...value, sort: sort as ProductsSort })}
          options={SORTS.map((sort) => ({ value: sort, label: t(`products_sort_${sort}`) }))} label={t("products_sort")} />
      </div>
      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t("products_active_filters")}>
          {chips.map((chip) => (
            <ActiveChip key={chip.key} label={chip.label} removeLabel={`${t("products_remove_filter")}: ${chip.label}`} onRemove={chip.clear} />
          ))}
        </div>
      )}
    </div>
  );
}
