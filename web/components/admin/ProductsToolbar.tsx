"use client";

import { Button } from "@/components/ui/button";
import { ActiveChip, FilterChip, ToolbarFilterButton, ToolbarSearch, ToolbarSort } from "@/components/admin/ListToolbar";
import { Popover, PopoverTrigger, PopoverContent, PopoverClose } from "@/components/ui/popover";
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

export function ProductsToolbar({ value, onChange, types }: {
  value: ProductsQuery;
  onChange: (q: ProductsQuery) => void;
  types: ProductType[];
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
          <PopoverContent align="end" className="admin-shell max-h-[var(--radix-popover-content-available-height)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto" aria-label={t("products_filter_products")}>
            <div className="flex flex-col gap-3.5">
              <h2 className="sr-only">{t("products_filter_products")}</h2>
              <div role="group" aria-label={t("products_type")}>
                <span className="mb-2 block type-overline text-ink-600">{t("products_type")}</span>
                <div className="flex flex-wrap gap-2">
                  <FilterChip on={!value.type} onClick={() => onChange({ ...value, type: undefined })}>{t("products_all_types")}</FilterChip>
                  {types.map((type) => <FilterChip key={type} on={value.type === type} onClick={() => onChange({ ...value, type })}>{t(`product_type_${type}`)}</FilterChip>)}
                </div>
              </div>
              <div role="group" aria-label={t("th_status")}>
                <span className="mb-2 block type-overline text-ink-600">{t("th_status")}</span>
                <div className="flex flex-wrap gap-2">
                  <FilterChip on={!value.status} onClick={() => onChange({ ...value, status: undefined })}>{t("tab_all")}</FilterChip>
                  {STATUSES.map((status) => <FilterChip key={status} on={value.status === status} onClick={() => onChange({ ...value, status })}>{t(`products_status_${status}`)}</FilterChip>)}
                </div>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2 border-t border-dashed border-line pt-3">
                <Button variant="ghost" size="sm" onClick={() => onChange({ ...value, type: undefined, status: undefined })}>{t("products_reset_filters")}</Button>
                <PopoverClose asChild><Button variant="secondary" size="sm">{t("products_show_products")}</Button></PopoverClose>
              </div>
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
