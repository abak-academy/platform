"use client";

import Link from "next/link";
import { useState } from "react";
import { Bell, ShoppingBag } from "lucide-react";
import { useProducts } from "@/lib/hooks/products";
import { useTranslation } from "@/lib/i18n";
import { CategoryRail, type CatalogCategory } from "@/components/catalog/CategoryRail";
import { Skeleton } from "@/components/ui/skeleton";
import { Button } from "@/components/ui/button";
import { ProductCard } from "@/components/catalog/ProductCard";

function CatalogGrid({ products, tab }: { products: ReturnType<typeof useProducts>["data"]; tab: CatalogCategory }) {
  const { t } = useTranslation();
  if (!products || products.length === 0) {
    if (tab !== "all") {
      return (
        <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
          <p className="text-sm text-ink-500">{t("catalog_empty")}</p>
        </div>
      );
    }
    return (
      <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
        <div className="mx-auto mb-6 flex size-24 items-center justify-center rounded-full bg-brand-50">
          <ShoppingBag className="size-12 text-brand-400" strokeWidth={1.5} />
        </div>
        <h2 className="font-serif text-2xl font-bold text-ink-900">{t("store_setup_title")}</h2>
        <p className="mx-auto mt-3 max-w-sm text-sm text-ink-500">{t("store_setup_desc")}</p>
        <div className="mt-8 flex flex-wrap items-center justify-center gap-3">
          <Button>
            <Bell className="mr-2 size-4" />
            {t("store_notify_me")}
          </Button>
          <Button asChild variant="ghost">
            <Link href="/">{t("store_back_dashboard")}</Link>
          </Button>
        </div>
      </div>
    );
  }
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {products.map((p) => (
        <ProductCard key={p.id} product={p} />
      ))}
    </div>
  );
}

function CatalogSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex flex-col overflow-hidden rounded-lg border border-line bg-surface">
          <Skeleton className="aspect-[3/4] rounded-none" />
          <div className="flex flex-col gap-2 p-3">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="mt-1 h-5 w-1/3" />
          </div>
        </div>
      ))}
    </div>
  );
}

export default function CatalogPage() {
  const { t } = useTranslation();
  const [category, setCategory] = useState<CatalogCategory>("all");
  const type = category === "all" ? undefined : category;
  const { data, isLoading, isError, error, refetch } = useProducts(type);

  return (
    <>
      <header className="mb-6">
        <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">{t("nav_store")}</h1>
        <p className="mt-1 text-sm text-ink-500">{t("catalog_subtitle")}</p>
      </header>

      <div className="flex flex-col gap-4 md:flex-row md:gap-6">
        <CategoryRail value={category} onChange={setCategory} />

        <div className="min-w-0 flex-1">
          {isError ? (
            <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
              <p>{t("catalog_load_failed")} {(error as Error)?.message}</p>
              <button onClick={() => refetch()} className="mt-2 underline">
                {t("retry")}
              </button>
            </div>
          ) : isLoading ? (
            <CatalogSkeleton />
          ) : (
            <CatalogGrid products={data} tab={category} />
          )}
        </div>
      </div>
    </>
  );
}