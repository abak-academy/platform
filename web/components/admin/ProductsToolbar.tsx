"use client";

import { Search, SlidersHorizontal, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Popover, PopoverTrigger, PopoverContent, PopoverClose } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
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
    <div className="space-y-3">
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:flex-1">
          <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
          <Input type="search" value={value.q ?? ""} onChange={(e) => onChange({ ...value, q: e.target.value || undefined })}
            placeholder={t("products_search_placeholder")} aria-label={t("products_search_placeholder")} className="pr-10 pl-9" />
          {value.q && (
            <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9"
              aria-label={t("products_clear_search")} onClick={() => onChange({ ...value, q: undefined })}>
              <X className="size-4" aria-hidden />
            </Button>
          )}
        </div>
        <Popover>
          <PopoverTrigger asChild>
            <Button variant="outline" className="h-9" aria-label={`${t("products_filters")}${chips.length ? ` (${chips.length})` : ""}`}>
              <SlidersHorizontal className="size-4" aria-hidden />
              {t("products_filters")}
              {chips.length > 0 && <span className="tabular-nums">{chips.length}</span>}
            </Button>
          </PopoverTrigger>
          <PopoverContent align="end" className="admin-shell max-h-[var(--radix-popover-content-available-height)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto" aria-label={t("products_filter_products")}>
            <div className="space-y-3">
              <h2 className="font-semibold">{t("products_filter_products")}</h2>
              <span className="block text-sm font-medium">{t("products_type")}</span>
              <Select value={value.type ?? "all"} onValueChange={(v) => onChange({ ...value, type: v === "all" ? undefined : v as ProductType })}>
                <SelectTrigger className="w-full" aria-label={t("products_type")}><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("products_all_types")}</SelectItem>
                  {types.map((type) => <SelectItem key={type} value={type}>{t(`product_type_${type}`)}</SelectItem>)}
                </SelectContent>
              </Select>
              <span className="block text-sm font-medium">{t("th_status")}</span>
              <Select value={value.status ?? "all"} onValueChange={(v) => onChange({ ...value, status: v === "all" ? undefined : v as ProductStatus })}>
                <SelectTrigger className="w-full" aria-label={t("th_status")}><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">{t("tab_all")}</SelectItem>
                  {STATUSES.map((status) => <SelectItem key={status} value={status}>{t(`products_status_${status}`)}</SelectItem>)}
                </SelectContent>
              </Select>
              <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                <Button variant="ghost" onClick={() => onChange({ ...value, type: undefined, status: undefined })}>{t("products_reset_filters")}</Button>
                <PopoverClose asChild><Button>{t("products_show_products")}</Button></PopoverClose>
              </div>
            </div>
          </PopoverContent>
        </Popover>
        <Select value={value.sort} onValueChange={(sort) => onChange({ ...value, sort: sort as ProductsSort })}>
          <SelectTrigger className="w-60 min-w-0 flex-1 sm:flex-none" aria-label={t("products_sort")}><SelectValue /></SelectTrigger>
          <SelectContent>
            {SORTS.map((sort) => <SelectItem key={sort} value={sort}>{t(`products_sort_${sort}`)}</SelectItem>)}
          </SelectContent>
        </Select>
      </div>
      {chips.length > 0 && (
        <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("products_active_filters")}>
          {chips.map((chip) => (
            <span key={chip.key} className="md-chip md-chip-primary gap-1">
              {chip.label}
              <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("products_remove_filter")}: ${chip.label}`} onClick={chip.clear}>
                <X className="size-4" aria-hidden />
              </Button>
            </span>
          ))}
        </div>
      )}
    </div>
  );
}
