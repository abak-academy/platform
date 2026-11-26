"use client";

import Link from "next/link";
import { Suspense, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { ShoppingBag } from "lucide-react";
import { useProducts } from "@/lib/hooks/products";
import { useProductGroupings } from "@/lib/hooks/product-groupings";
import { useTranslation } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import {
  CategoryRail,
  type CatalogCategory,
} from "@/components/catalog/CategoryRail";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ProductCard } from "@/components/catalog/ProductCard";
import type { Product } from "@/lib/types";

const ALL_GROUPINGS = "all";
const PRIORITY_CARDS = 4;
const SKELETON_CARDS = 10;
const wrap = "mx-auto w-full max-w-public px-6";
const CATEGORIES: CatalogCategory[] = [
  "book",
  "course",
  "merchandise",
  "medal",
];

function CatalogGrid({
  products,
  isFiltered,
}: {
  products: Product[] | undefined;
  isFiltered: boolean;
}) {
  const { t } = useTranslation();
  const token = useAuthStore((s) => s.token);
  if (!products || products.length === 0) {
    if (isFiltered) {
      return (
        <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
          <p className="text-sm text-ink-600">{t("catalog_empty")}</p>
        </div>
      );
    }
    return (
      <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
        <div className="mx-auto mb-6 flex size-24 items-center justify-center rounded-full bg-brand-50">
          <ShoppingBag className="size-12 text-brand-400" strokeWidth={1.5} />
        </div>
        <h2 className="font-serif text-2xl font-bold text-ink-900">
          {t("store_setup_title")}
        </h2>
        <p className="mx-auto mt-3 max-w-sm text-sm text-ink-600">
          {t("store_setup_desc")}
        </p>
        {token && (
          <div className="mt-8 flex justify-center">
            <Button asChild variant="ghost">
              <Link href="/">{t("store_back_dashboard")}</Link>
            </Button>
          </div>
        )}
      </div>
    );
  }
  return (
    <div
      data-reveal-grid
      className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
    >
      {products.map((p, i) => (
        <div key={p.id} data-tilt className="h-full rounded-lg">
          <ProductCard product={p} priority={i < PRIORITY_CARDS} className="hover:translate-y-0 hover:shadow-[var(--sh-sm)]" />
        </div>
      ))}
    </div>
  );
}

function CatalogSkeleton() {
  return (
    <div
      data-testid="catalog-skeleton"
      className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
    >
      {Array.from({ length: SKELETON_CARDS }).map((_, i) => (
        <div
          key={i}
          className="flex flex-col overflow-hidden rounded-lg border border-line bg-surface"
        >
          <Skeleton className="aspect-[3/4] rounded-none" />
          <div className="flex flex-col gap-1 p-3">
            <Skeleton className="h-10 w-3/4" />
            <Skeleton className="mt-2 h-6 w-1/3" />
          </div>
        </div>
      ))}
    </div>
  );
}

function CatalogContent() {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const [category, setCategory] = useState<CatalogCategory>(() => {
    const q = searchParams.get("type") as CatalogCategory;
    return CATEGORIES.includes(q) ? q : "all";
  });
  const [groupingId, setGroupingId] = useState(
    searchParams.get("grouping") ?? ALL_GROUPINGS,
  );
  const [search, setSearch] = useState("");

  const type = category === "all" ? undefined : category;
  const { data, isLoading, isError, error, refetch } = useProducts(
    type,
    groupingId === ALL_GROUPINGS ? undefined : groupingId,
  );
  const { data: groupings } = useProductGroupings();

  function changeGrouping(value: string) {
    setGroupingId(value);
    const params = new URLSearchParams(searchParams.toString());
    if (value === ALL_GROUPINGS) params.delete("grouping");
    else params.set("grouping", value);
    const qs = params.toString();
    router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
  }

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    const shop = data?.filter((p) => p.type !== "exam");
    if (!shop || q === "") return shop;
    return shop.filter((p) => p.name.toLowerCase().includes(q));
  }, [data, search]);

  const isFiltered =
    category !== "all" || groupingId !== ALL_GROUPINGS || search.trim() !== "";

  return (
    <>
        <div className="mb-8 flex flex-col gap-3 sm:flex-row sm:items-center">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("catalog_search_placeholder")}
            aria-label={t("catalog_search_placeholder")}
            className="sm:max-w-xs"
          />
          {(groupings?.length ?? 0) > 0 && (
            <Select value={groupingId} onValueChange={changeGrouping}>
              <SelectTrigger
                aria-label={t("catalog_grouping_label")}
                className="sm:w-56"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_GROUPINGS}>
                  {t("catalog_grouping_all")}
                </SelectItem>
                {groupings!.map((g) => (
                  <SelectItem key={g.id} value={g.id}>
                    {g.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className="flex flex-col gap-4 md:flex-row md:gap-6">
          <CategoryRail value={category} onChange={setCategory} />

          <div className="min-h-[32rem] min-w-0 flex-1">
            {isError ? (
              <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
                <p>
                  {t("catalog_load_failed")} {(error as Error)?.message}
                </p>
                <button onClick={() => refetch()} className="mt-2 underline">
                  {t("retry")}
                </button>
              </div>
            ) : isLoading ? (
              <CatalogSkeleton />
            ) : (
              <CatalogGrid products={filtered} isFiltered={isFiltered} />
            )}
          </div>
        </div>
    </>
  );
}

function CatalogFallback() {
  return (
    <>
      <Skeleton className="mb-8 h-10 w-full sm:max-w-xs" />
      <div className="min-h-[32rem]">
        <CatalogSkeleton />
      </div>
    </>
  );
}

function CatalogHero() {
  const { t } = useTranslation();
  return (
    <section className="relative overflow-hidden pt-32 pb-12">
      <div
        aria-hidden
        className="page-hero-fade absolute inset-0 -z-10 bg-brand-50"
      />
      <div className={wrap}>
        <span className="type-overline text-ember-700">{t("shop_eyebrow")}</span>
        <h1 className="mt-2 type-display">{t("shop_title")}</h1>
        <p className="mt-3 max-w-2xl type-body text-ink-700">
          {t("shop_tagline")}
        </p>
      </div>
    </section>
  );
}

export default function CatalogPage() {
  return (
    <div>
      <CatalogHero />
      <section data-reveal className={`${wrap} pt-12 pb-20`}>
        <Suspense fallback={<CatalogFallback />}>
          <CatalogContent />
        </Suspense>
      </section>
    </div>
  );
}
