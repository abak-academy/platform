"use client";

import { useMemo, useState } from "react";
import { useTranslation } from "@/lib/i18n";
import { BookOpen, PlayCircle, Trophy, Wallet } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { TopProductsTable } from "@/components/admin/TopProductsTable";
import { PeriodBar, presetRange, type PeriodPreset } from "@/components/admin/PeriodBar";
import { DataTable } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminRevenue, type RevenueRange } from "@/lib/hooks/admin-revenue";
import { hasCapability, useResolvedRole } from "@/lib/hooks/use-capability";
import { formatRupiah } from "@/lib/format";
import type { AdminRevenue, ProductType, RevenueByTypeItem } from "@/lib/types";

function averageOrderValue(revenue: AdminRevenue): number {
  if (!revenue.order_count) return 0;
  return revenue.total / revenue.order_count;
}

function typeEntries(revenue?: AdminRevenue): [string, RevenueByTypeItem][] {
  if (!revenue) return [];
  return Object.entries(revenue.by_type).sort((a, b) => b[1].total - a[1].total);
}

function maxTypeTotal(revenue?: AdminRevenue): number {
  const entries = typeEntries(revenue);
  if (entries.length === 0) return 1;
  return Math.max(...entries.map(([, item]) => item.total));
}

function StatsSkeleton() {
  return (
    <div className="grid grid-cols-2 gap-3 min-[1201px]:grid-cols-4">
      {Array.from({ length: 4 }).map((_, i) => (
        <Skeleton key={i} className="h-28 w-full" />
      ))}
    </div>
  );
}

export default function RevenuePage() {
  const { t, lang } = useTranslation();
  const [preset, setPreset] = useState<PeriodPreset>("30d");
  const [custom, setCustom] = useState<RevenueRange>({});

  const range: RevenueRange = preset === "all_time" ? { from: "2000-01-01" } : preset === "custom" ? custom : presetRange(preset);
  const { data: revenue, isLoading, isError, error } = useAdminRevenue(range);

  // The role is undefined for one render while the auth store rehydrates, so
  // falling straight through to NoAccess flashes it at every admin on load.
  const { role, hydrated } = useResolvedRole();
  const canReadRevenue = hasCapability(role, "revenue:read");

  const formatDate = useMemo(() => {
    const fmt = new Intl.DateTimeFormat(lang === "en" ? "en-GB" : "id-ID", {
      day: "numeric",
      month: "short",
      year: "numeric",
      timeZone: "UTC",
    });
    return (iso: string) => fmt.format(new Date(`${iso}T00:00:00Z`));
  }, [lang]);

  const entries = typeEntries(revenue);
  const max = maxTypeTotal(revenue);
  const typeSum = entries.reduce((sum, [, item]) => sum + item.total, 0);
  const share = (types: string[]) =>
    typeSum > 0 ? Math.round((entries.filter(([type]) => types.includes(type)).reduce((sum, [, item]) => sum + item.total, 0) / typeSum) * 100) : 0;
  const typeLabel = (type: string) =>
    (["book", "merchandise", "medal", "course", "exam"] as const).includes(type as ProductType) ? t(`product_type_${type as ProductType}`) : type;

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  if (!hydrated) {
    return (
      <div className="flex flex-col gap-5 fade-in">
        <AdminPageHeader
            title={t("revenue_page_title")}
          description={t("revenue_page_description")}
        />
        <StatsSkeleton />
      </div>
    );
  }

  if (!canReadRevenue) return <NoAccess />;

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        title={t("revenue_page_title")}
        description={t("revenue_page_description")}
      />

      <PeriodBar
        preset={preset}
        range={custom}
        allowAllTime
        onChange={(next, nextRange) => {
          setPreset(next);
          setCustom(nextRange);
        }}
        resolvedFrom={revenue ? formatDate(revenue.from) : undefined}
        resolvedTo={revenue ? formatDate(revenue.to) : undefined}
      />

      {isLoading && <StatsSkeleton />}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("revenue_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && revenue && (
        <>
          <div className="grid grid-cols-2 gap-3 min-[681px]:grid-cols-3 min-[1201px]:grid-cols-4" data-testid="revenue-stats">
            <StatCard
              label={t("revenue_total")}
              value={formatRupiah(revenue.total)}
              tone="teal" icon={Wallet}
              trend={t("revenue_total_sub").replace("{n}", String(revenue.order_count)).replace("{aov}", formatRupiah(averageOrderValue(revenue)))}
            />
            <StatCard label={t("revenue_share_exam")} value={`${share(["exam"])}%`} tone="ember" icon={Trophy} />
            <StatCard label={t("revenue_share_course")} value={`${share(["course"])}%`} tone="sky" icon={PlayCircle} />
            <StatCard label={t("revenue_share_physical")} value={`${share(["book", "merchandise", "medal"])}%`} tone="gold" icon={BookOpen} />
          </div>

          <section className="md-card-outlined flex flex-col gap-3.5">
            <h2 className="type-h2">{t("revenue_by_type")}</h2>
            {entries.length === 0 ? (
              <div className="text-sm text-ink-600">{t("revenue_no_data")}</div>
            ) : (
              <>
                <div aria-hidden className="flex h-[220px] items-end gap-3 pt-[34px]">
                  {entries.map(([type, item]) => (
                    <div key={type} className="flex h-full flex-1 flex-col items-center justify-end gap-2">
                      <div
                        data-testid={`bar-${type}`}
                        className={`relative w-full max-w-[46px] rounded-[12px_12px_6px_6px] ${item.total === max ? "bg-[repeating-linear-gradient(135deg,var(--color-ember-500)_0_6px,var(--color-ember-600)_6px_11px)]" : "bg-brand-600"}`}
                        style={{ height: `${max > 0 ? (item.total / (max * 1.15)) * 100 : 0}%` }}
                      >
                        {item.total === max && <span className="absolute bottom-[calc(100%+6px)] left-1/2 -translate-x-1/2 rounded-full bg-deep px-[9px] py-[5px] font-[family-name:var(--font-display)] text-[11px] font-bold whitespace-nowrap text-white">{formatRupiah(item.total)}</span>}
                      </div>
                      <span className={`text-xs ${item.total === max ? "font-bold text-ink-900" : "text-ink-600"}`}>{typeLabel(type)}</span>
                    </div>
                  ))}
                </div>
                <DataTable
                  surface="plain"
                  rows={entries}
                  empty={t("revenue_no_data")}
                  rowKey={([type]) => type}
                  columns={[
                    { key: "type", header: t("th_type"), cell: ([type]) => typeLabel(type) },
                    { key: "count", header: t("revenue_order_label"), align: "right", className: "tabular-nums", cell: ([, item]) => item.count },
                    { key: "total", header: t("revenue_total"), align: "right", strong: true, className: "tabular-nums", cell: ([, item]) => formatRupiah(item.total) },
                  ]}
                />
              </>
            )}
          </section>

          <div className="md-card-outlined">
            <h3 className="type-h2 mb-4">{t("revenue_top_products")}</h3>
            <TopProductsTable
              products={revenue.top_products}
              showRevenue
              emptyLabel={t("revenue_top_empty")}
            />
          </div>

          <div className="md-card-outlined" data-testid="revenue-reconciliation">
            <h3 className="type-h2 mb-4">{t("revenue_reconciliation_title")}</h3>
            {/* Full width, values flush right — it reads as a receipt against
                the Top products table above it, whose revenue column lands on
                the same edge. */}
            <dl className="space-y-2.5 text-[15px]">
              <div className="flex items-baseline justify-between gap-6">
                <dt>{t("revenue_product_revenue")}</dt>
                <dd className="tabular-nums">{formatRupiah(revenue.product_revenue)}</dd>
              </div>
              <div className="flex items-baseline justify-between gap-6">
                <dt>{t("revenue_shipping_total")}</dt>
                <dd className="tabular-nums">+ {formatRupiah(revenue.shipping_total)}</dd>
              </div>
              <div className="flex items-baseline justify-between gap-6">
                <dt>{t("revenue_discount_total")}</dt>
                <dd className="tabular-nums">− {formatRupiah(revenue.discount_total)}</dd>
              </div>
              <div className="flex items-baseline justify-between gap-6 border-t border-line pt-3 text-base font-semibold">
                <dt>{t("revenue_total")}</dt>
                <dd className="tabular-nums">{formatRupiah(revenue.total)}</dd>
              </div>
            </dl>
          </div>
        </>
      )}
    </div>
  );
}
