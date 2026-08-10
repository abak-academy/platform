"use client";

import Link from "next/link";
import {
  Store,
  Receipt,
  Package,
  Clock,
  PackageCheck,
  AlertTriangle,
  ShoppingBag,
  CheckCircle2,
} from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { TopProductsTable } from "@/components/admin/TopProductsTable";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminOrderSummary } from "@/lib/hooks/admin-orders";
import { useTranslation } from "@/lib/i18n";

function BandLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="mb-4 flex items-center gap-4">
      <h2 className="text-label uppercase" style={{ letterSpacing: "0.08em" }}>
        {children}
      </h2>
      <span className="h-px flex-1" style={{ backgroundColor: "var(--md-sys-color-outline-variant)" }} aria-hidden />
    </div>
  );
}

// Each queue card opens its own queue. All three pointing at the unfiltered
// list made them a count with nowhere to go — the whole point of the band is
// that clicking a number shows you the work behind it.
function QueueCardLink({ status, children }: { status: string; children: React.ReactNode }) {
  return (
    <Link
      href={`/admin/orders?status=${status}`}
      className="block rounded-[20px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
      style={{ outlineColor: "var(--md-sys-color-primary)" }}
    >
      {children}
    </Link>
  );
}

export default function StoreDashboardPage() {
  const { t } = useTranslation();
  const { data: summary, isLoading } = useAdminOrderSummary({ status: "all" });

  const buckets = summary?.buckets;

  const quickActions = [
    { icon: Package, label: t("store_action_manage_products"), href: "/admin/products" },
    { icon: Receipt, label: t("store_action_view_orders"), href: "/admin/orders" },
  ];

  return (
    <div className="space-y-8 fade-in">
      <AdminPageHeader icon={Store} title={t("store_title")} description={t("store_subtitle")} />

      {/* Work queue — the only band allowed to turn red, and only on real work. */}
      <section>
        <BandLabel>{t("store_needs_action")}</BandLabel>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-3">
          {isLoading ? (
            <>
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
            </>
          ) : (
            <>
              <QueueCardLink status="pending">
                <StatCard
                  label={t("store_stat_needs_confirm")}
                  value={String(buckets?.needs_confirm ?? 0)}
                  accent="primary"
                  icon={Clock}
                />
              </QueueCardLink>
              <QueueCardLink status="ready_to_ship">
                <StatCard
                  label={t("store_stat_ready_to_ship")}
                  value={String(buckets?.ready_to_ship ?? 0)}
                  accent="tertiary"
                  icon={PackageCheck}
                />
              </QueueCardLink>
              <QueueCardLink status="shipment_failed">
                <StatCard
                  label={t("store_stat_shipment_failed")}
                  value={String(buckets?.shipment_failed ?? 0)}
                  accent={(buckets?.shipment_failed ?? 0) > 0 ? "error" : "secondary"}
                  icon={AlertTriangle}
                />
              </QueueCardLink>
            </>
          )}
        </div>
      </section>

      <section>
        <BandLabel>{t("store_volume_month")}</BandLabel>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
          {isLoading ? (
            <>
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
            </>
          ) : (
            <>
              <StatCard
                label={t("store_stat_orders_this_month")}
                value={String(buckets?.created_this_month ?? 0)}
                accent="secondary"
                icon={ShoppingBag}
              />
              <StatCard
                label={t("store_stat_completed_this_month")}
                value={String(buckets?.completed_this_month ?? 0)}
                accent="secondary"
                icon={CheckCircle2}
              />
            </>
          )}
        </div>
      </section>

      <div className="md-card-outlined">
        <h3 className="text-title-medium mb-4">{t("store_top_products")}</h3>
        {isLoading ? (
          <Skeleton className="h-40 w-full" />
        ) : (
          <TopProductsTable products={summary?.top_products ?? []} showRevenue={false} />
        )}
      </div>

      <div className="md-card-outlined">
        <h3 className="text-title-large mb-6">{t("admin_home_quick_actions")}</h3>
        <div className="grid gap-3 sm:grid-cols-2">
          {quickActions.map((action) => (
            <Link
              key={action.href}
              href={action.href}
              className="flex items-center gap-3 p-4 rounded-[12px] border border-line hover:bg-surface-container transition-colors"
            >
              <div
                className="flex size-10 items-center justify-center rounded-[10px]"
                style={{
                  backgroundColor: "var(--md-sys-color-primary-container)",
                  color: "var(--md-sys-color-primary)",
                }}
              >
                <action.icon size={20} />
              </div>
              <span className="text-body font-medium">{action.label}</span>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}
