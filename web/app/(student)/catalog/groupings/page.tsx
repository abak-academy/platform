"use client";

import Link from "next/link";
import { Layers } from "lucide-react";
import { useProductGroupings } from "@/lib/hooks/product-groupings";
import { useTranslation } from "@/lib/i18n";
import { Skeleton } from "@/components/ui/skeleton";

export default function ProductGroupingsPage() {
  const { t } = useTranslation();
  const { data: groupings, isLoading, isError, error, refetch } = useProductGroupings();

  return (
    <>
      <header className="mb-6">
        <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">{t("grouping_landing_title")}</h1>
        <p className="mt-1 text-sm text-ink-500">{t("grouping_landing_description")}</p>
      </header>

      {isError && (
        <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
          <p>
            {t("grouping_load_failed")} {(error as Error)?.message}
          </p>
          <button onClick={() => refetch()} className="mt-2 underline">
            {t("retry")}
          </button>
        </div>
      )}

      {isLoading && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-28 w-full" />
          ))}
        </div>
      )}

      {!isLoading && !isError && (groupings?.length ?? 0) === 0 && (
        <div className="rounded-2xl border border-line bg-surface px-8 py-16 text-center">
          <p className="text-sm text-ink-500">{t("grouping_landing_empty")}</p>
        </div>
      )}

      {!isLoading && !isError && (groupings?.length ?? 0) > 0 && (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {groupings!.map((grouping) => (
            <Link
              key={grouping.id}
              href={`/catalog/groupings/${grouping.id}`}
              className="flex flex-col gap-2 rounded-lg border border-line bg-surface p-4 transition hover:border-brand-400 hover:shadow-sm"
            >
              <div className="flex items-center gap-2">
                <Layers className="size-5 text-brand-400" aria-hidden />
                <span className="font-medium text-ink-900">{grouping.name}</span>
              </div>
              {grouping.description && <p className="text-sm text-ink-500">{grouping.description}</p>}
            </Link>
          ))}
        </div>
      )}
    </>
  );
}
