"use client";

import { use } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { useProductGrouping } from "@/lib/hooks/product-groupings";
import { useTranslation } from "@/lib/i18n";
import { Skeleton } from "@/components/ui/skeleton";
import { ProductCard } from "@/components/catalog/ProductCard";

export default function ProductGroupingDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { t } = useTranslation();
  const { id } = use(params);
  const { data: grouping, isLoading, isError, error, refetch } = useProductGrouping(id);

  return (
    <>
      <Link href="/catalog/groupings" className="mb-4 inline-flex items-center gap-1 text-sm text-ink-500 hover:text-ink-900">
        <ArrowLeft className="size-4" aria-hidden />
        {t("grouping_back_to_list")}
      </Link>

      {isLoading && <Skeleton className="h-10 w-1/2" />}

      {isError && (
        <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
          <p>
            {t("grouping_detail_load_failed")} {(error as Error)?.message}
          </p>
          <button onClick={() => refetch()} className="mt-2 underline">
            {t("retry")}
          </button>
        </div>
      )}

      {grouping && (
        <>
          <header className="mb-6">
            <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">{grouping.name}</h1>
            {grouping.description && <p className="mt-1 text-sm text-ink-500">{grouping.description}</p>}
          </header>

          {grouping.products.length === 0 ? (
            <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
              <p className="text-sm text-ink-500">{t("grouping_detail_empty")}</p>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
              {grouping.products.map((p) => (
                <ProductCard key={p.id} product={p} />
              ))}
            </div>
          )}
        </>
      )}
    </>
  );
}
