"use client";

import { useMemo, useState } from "react";
import { useTranslation } from "@/lib/i18n";
import { BarChart3 } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { TopProductsTable } from "@/components/admin/TopProductsTable";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminRevenue, type RevenueRange } from "@/lib/hooks/admin-revenue";
import { hasCapability, useResolvedRole } from "@/lib/hooks/use-capability";
import { formatRupiah } from "@/lib/format";
import type { AdminRevenue, RevenueByTypeItem } from "@/lib/types";

const PRESETS = [
  { id: "7d", labelKey: "revenue_period_7d" },
  { id: "30d", labelKey: "revenue_period_30d" },
  { id: "this_month", labelKey: "revenue_period_this_month" },
  { id: "all", labelKey: "revenue_period_all" },
] as const;

type Preset = (typeof PRESETS)[number]["id"];

// KNOWN BUG (out of this PR's scope, separate ticket): UTC-based, not
// Asia/Jakarta, and off by one day for an N-day window against an exclusive
// `to`. web/components/admin/PeriodBar.tsx fixed the same pair — see its
// jakartaDateString/isoDaysAgo before assuming this copy agrees with it.
function isoDaysAgo(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  return d.toISOString().slice(0, 10);
}

function firstOfThisMonth(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-01`;
}

// 30d sends nothing on purpose: the server default is already now-30d..now, so
// an explicit range would only add a client/server "today" mismatch.
function presetRange(preset: Preset): RevenueRange {
  switch (preset) {
    case "7d":
      return { from: isoDaysAgo(7) };
    case "this_month":
      return { from: firstOfThisMonth() };
    case "all":
      return { from: "2000-01-01" };
    default:
      return {};
  }
}

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
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} className="h-28 w-full" />
      ))}
    </div>
  );
}

export default function RevenuePage() {
  const { t, lang } = useTranslation();
  const [preset, setPreset] = useState<Preset | null>("30d");
  const [custom, setCustom] = useState<RevenueRange>({});

  const range = preset ? presetRange(preset) : custom;
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

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  function pickPreset(next: Preset) {
    setPreset(next);
    setCustom({});
  }

  function pickCustom(next: RevenueRange) {
    setPreset(null);
    setCustom(next);
  }

  if (!hydrated) {
    return (
      <div className="space-y-6 fade-in">
        <AdminPageHeader
          icon={BarChart3}
          title={t("revenue_page_title")}
          description={t("revenue_page_description")}
        />
        <StatsSkeleton />
      </div>
    );
  }

  if (!canReadRevenue) return <NoAccess />;

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={BarChart3}
        title={t("revenue_page_title")}
        description={t("revenue_page_description")}
      />

      <div className="md-card-outlined">
        <div className="flex flex-wrap items-end justify-between gap-4">
          <div className="flex flex-wrap gap-2">
            {PRESETS.map((p) => {
              const active = preset === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  aria-pressed={active}
                  onClick={() => pickPreset(p.id)}
                  className={`md-chip cursor-pointer transition-colors ${
                    active ? "md-chip-primary ring-1 ring-[var(--md-sys-color-primary)]" : ""
                  }`}
                >
                  {t(p.labelKey)}
                </button>
              );
            })}
          </div>

          {/* One inline row, same baseline as the presets. Stacked uppercase
              labels made this group taller than the chips beside it, so the
              two never lined up. The dash carries the "range" meaning the
              labels were carrying. */}
          <div className="flex items-center gap-2">
            <label htmlFor="revenue-date-from" className="sr-only">
              {t("orders_date_from")}
            </label>
            <Input
              id="revenue-date-from"
              type="date"
              value={custom.from ?? ""}
              onChange={(e) => pickCustom({ ...custom, from: e.target.value || undefined })}
              className="w-auto"
            />
            <span aria-hidden className="text-ink-600">
              –
            </span>
            <label htmlFor="revenue-date-to" className="sr-only">
              {t("orders_date_to")}
            </label>
            <Input
              id="revenue-date-to"
              type="date"
              value={custom.to ?? ""}
              onChange={(e) => pickCustom({ ...custom, to: e.target.value || undefined })}
              className="w-auto"
            />
          </div>
        </div>

        {revenue && (
          <p
            className="text-label mt-4 tabular-nums"
            data-testid="revenue-period-label"
          >
            {formatDate(revenue.from)} – {formatDate(revenue.to)}
          </p>
        )}
      </div>

      {isLoading && <StatsSkeleton />}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("revenue_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && revenue && (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="revenue-stats">
            <StatCard
              label={t("revenue_total")}
              value={formatRupiah(revenue.total)}
              accent="primary"
            />
            <div data-testid="stat-order-count">
              <StatCard
                label={t("orders")}
                value={String(revenue.order_count)}
                accent="secondary"
              />
            </div>
            <StatCard
              label={t("revenue_avg_order")}
              value={formatRupiah(averageOrderValue(revenue))}
              accent="tertiary"
            />
          </div>

          <div className="md-card-outlined">
            <h3 className="text-title-medium mb-4">{t("revenue_by_type")}</h3>
            <div>
              {entries.length === 0 ? (
                <div className="text-sm text-muted-foreground">{t("revenue_no_data")}</div>
              ) : (
                <div className="space-y-4">
                  {entries.map(([type, item]) => {
                    const pct = max > 0 ? (item.total / max) * 100 : 0;
                    return (
                      <div key={type} className="space-y-1">
                        <div className="flex items-center justify-between text-sm">
                          <span className="capitalize font-medium">{type}</span>
                          <span className="text-muted-foreground">
                            {formatRupiah(item.total)} · {item.count} {t("revenue_order_label")}
                          </span>
                        </div>
                        <div className="h-3 w-full overflow-hidden rounded-full bg-muted">
                          <div
                            className="h-full rounded-full bg-primary"
                            style={{ width: `${pct}%` }}
                            data-testid={`bar-${type}`}
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          <div className="md-card-outlined">
            <h3 className="text-title-medium mb-4">{t("revenue_top_products")}</h3>
            <TopProductsTable
              products={revenue.top_products}
              showRevenue
              emptyLabel={t("revenue_top_empty")}
            />
          </div>

          <div className="md-card-outlined" data-testid="revenue-reconciliation">
            <h3 className="text-title-medium mb-4">{t("revenue_reconciliation_title")}</h3>
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
              <div className="flex items-baseline justify-between gap-6 border-t border-[var(--md-sys-color-outline-variant)] pt-3 text-base font-semibold">
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
