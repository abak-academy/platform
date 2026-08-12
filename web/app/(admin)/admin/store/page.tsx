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
  Tag,
  Bell,
} from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { QuickActionTiles, type QuickAction } from "@/components/admin/QuickActionTiles";
import { StatCard } from "@/components/admin/StatCard";
import { TopProductsTable } from "@/components/admin/TopProductsTable";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminOrderSummary } from "@/lib/hooks/admin-orders";
import { useAdminPromoCodes } from "@/lib/hooks/admin-promos";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";

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
//
// "status" is a real orders.status value (e.g. "pending"); "queue" is one of
// the two synthetic buckets that has no single matching status — see
// AdminOrderQueue.
function QueueCardLink({
  status,
  queue,
  children,
}: {
  status?: string;
  queue?: string;
  children: React.ReactNode;
}) {
  const param = queue ? `queue=${queue}` : `status=${status}`;
  return (
    <Link
      href={`/admin/orders?${param}`}
      className="block rounded-[20px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2"
      style={{ outlineColor: "var(--md-sys-color-primary)" }}
    >
      {children}
    </Link>
  );
}

export default function StoreDashboardPage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const name = user?.name ?? me.data?.name ?? t("store_home_default_name");
  const { data: summary, isLoading } = useAdminOrderSummary({ status: "all" });
  const { data: promos = [] } = useAdminPromoCodes();

  const buckets = summary?.buckets;

  // admin_store holds no revenue:read capability (2026-08-04 split) — this page must never surface an aggregate figure.
  const now = Date.now();
  const weekAway = now + 7 * 24 * 60 * 60 * 1000;
  const activePromos = promos.filter((p) => {
    const notExpired = !p.expires_at || new Date(p.expires_at).getTime() > now;
    const hasUsesLeft = p.max_uses === undefined || p.used_count < p.max_uses;
    return notExpired && hasUsesLeft;
  });
  const expiringPromos = activePromos.filter(
    (p) => p.expires_at !== undefined && new Date(p.expires_at).getTime() <= weekAway,
  );

  const quickActions: QuickAction[] = [
    { icon: Package, label: t("store_action_manage_products"), href: "/admin/products" },
    { icon: Receipt, label: t("store_action_view_orders"), href: "/admin/orders" },
    { icon: Tag, label: t("store_action_new_promo"), href: "/admin/promos" },
    { icon: Bell, label: t("store_action_announce"), href: "/admin/notifications" },
  ];

  return (
    <div className="space-y-8 fade-in">
      <DashboardHero icon={Store} badge={t("store_home_badge")} name={name} subtitle={t("store_subtitle")} />

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
              <QueueCardLink queue="ready_to_ship">
                <StatCard
                  label={t("store_stat_ready_to_ship")}
                  value={String(buckets?.ready_to_ship ?? 0)}
                  accent="tertiary"
                  icon={PackageCheck}
                />
              </QueueCardLink>
              <QueueCardLink queue="shipment_failed">
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

      <section>
        <BandLabel>{t("store_promos")}</BandLabel>
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2">
          <Link href="/admin/promos" className="block rounded-[20px]">
            <div data-testid="store-promos-active">
              <StatCard label={t("store_promos_active")} value={String(activePromos.length)} accent="primary" icon={Tag} />
            </div>
          </Link>
          <Link href="/admin/promos" className="block rounded-[20px]">
            <div data-testid="store-promos-expiring">
              <StatCard
                label={t("store_promos_expiring")}
                value={String(expiringPromos.length)}
                accent={expiringPromos.length > 0 ? "tertiary" : "secondary"}
                icon={Clock}
              />
            </div>
          </Link>
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

      <QuickActionTiles title={t("admin_home_quick_actions")} actions={quickActions} />
    </div>
  );
}
