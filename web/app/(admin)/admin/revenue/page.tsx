"use client";

import { useTranslation } from "@/lib/i18n";
import { BarChart3 } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminRevenue } from "@/lib/hooks/admin-revenue";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { formatRupiah } from "@/lib/format";
import type { AdminRevenue, RevenueByTypeItem } from "@/lib/types";

function orderCount(revenue?: AdminRevenue): number {
  if (!revenue) return 0;
  return Object.values(revenue.by_type).reduce((sum, item) => sum + (item.count ?? 0), 0);
}

function averageOrderValue(revenue?: AdminRevenue): number {
  const count = orderCount(revenue);
  if (!count || !revenue) return 0;
  return revenue.total / count;
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

export default function RevenuePage() {
  const { t } = useTranslation();
  const { data: revenue, isLoading, isError, error } = useAdminRevenue();
  const canReadRevenue = useHasCapability("revenue:read");

  const entries = typeEntries(revenue);
  const max = maxTypeTotal(revenue);

  function errorMessage(error: unknown): string {
    if (error instanceof Error) return error.message;
    return t("error_generic");
  }

  if (!canReadRevenue) return <NoAccess />;

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={BarChart3}
        title={t("revenue_page_title")}
        description={t("revenue_page_description")}
      />

      {isLoading && (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-28 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {t("revenue_load_failed")}: {errorMessage(error)}
        </div>
      )}

      {!isLoading && !isError && revenue && (
        <>
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
            <StatCard
              label={t("revenue_total")}
              value={formatRupiah(revenue.total)}
              accent="primary"
            />
            <StatCard
              label={t("orders")}
              value={String(orderCount(revenue))}
              accent="secondary"
            />
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
            <div className="overflow-x-auto md-card-outlined">
                <table className="w-full text-sm">
                  <thead className="bg-muted">
                    <tr>
                      <th className="px-4 py-3 text-left font-medium">{t("th_product")}</th>
                      <th className="px-4 py-3 text-left font-medium">{t("orders")}</th>
                      <th className="px-4 py-3 text-right font-medium">{t("revenue")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr className="border-t">
                      <td colSpan={3} className="px-4 py-8 text-center text-muted-foreground">
                        {t("revenue_top_empty")}
                      </td>
                    </tr>
                  </tbody>
                </table>
            </div>
          </div>
        </>
      )}
    </div>
  );
}
