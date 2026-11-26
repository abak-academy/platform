"use client";

import Link from "next/link";
import { CheckCircle2, ShoppingBag, Store } from "lucide-react";
import { useAuthStore } from "@/stores/auth";
import { useMe } from "@/lib/hooks/auth";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { UnitSalesBlock } from "@/components/admin/UnitSalesBlock";
import { StatCard } from "@/components/admin/StatCard";
import { Skeleton } from "@/components/ui/skeleton";
import { useAdminOrderSummary } from "@/lib/hooks/admin-orders";
import { useAdminPromoCodes } from "@/lib/hooks/admin-promos";
import { useTranslation } from "@/lib/i18n";
import type { UserRole } from "@/lib/nav-config";

function BandLabel({ children }: { children: React.ReactNode }) {
  return <h2 className="mb-3 font-mono text-xs font-semibold tracking-[.14em] text-ink-600 uppercase">{children}</h2>;
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
      className="block rounded-[20px] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-brand-600"
    >
      {children}
    </Link>
  );
}

function CompactQueue({ label, value, error = false }: { label: string; value: number; error?: boolean }) {
  return <div className="flex items-center justify-between gap-3 rounded-[20px] border border-line bg-surface px-4 py-3.5 transition-transform duration-300 ease-spring hover:-translate-y-0.5">
    <span className="type-label-sm text-ink-700">{label}</span>
    <strong className={`font-[family-name:var(--font-display)] text-2xl font-extrabold tabular-nums ${error ? "text-danger-600" : "text-ink-900"}`}>{value}</strong>
  </div>;
}

export default function StoreDashboardPage() {
  const { t } = useTranslation();
  const user = useAuthStore((s) => s.user);
  const storeRole = user?.role as UserRole | undefined;
  const me = useMe({ enabled: !storeRole });
  const name = user?.name ?? me.data?.name ?? t("store_home_default_name");
  const canRead = useHasCapability("orders:read");
  const { data: summary, isLoading } = useAdminOrderSummary({ status: "all" });
  const { data: promos = [] } = useAdminPromoCodes();

  const buckets = summary?.buckets;
  const typeLabels: Record<string, string> = {
    book: t("admin_unit_book"), merchandise: t("admin_unit_merchandise"), medal: t("admin_unit_medal"),
    course: t("admin_unit_course"), exam: t("admin_unit_exam"),
  };

  // admin_store holds no revenue:read capability (2026-08-04 split) — this page must never surface an aggregate figure.
  const now = Date.now();
  const weekAway = now + 7 * 24 * 60 * 60 * 1000;
  const activePromos = promos.filter((p) => {
    const notExpired = !p.expires_at || new Date(p.expires_at).getTime() > now;
    // The server sends null, not undefined, for an absent limit/expiry — loose equality catches both.
    const hasUsesLeft = p.max_uses == null || p.used_count < p.max_uses;
    return notExpired && hasUsesLeft;
  });
  const expiringPromos = activePromos.filter(
    (p) => p.expires_at != null && new Date(p.expires_at).getTime() <= weekAway,
  );

  if (!canRead) return <NoAccess />;

  return (
    <div className="flex flex-col gap-5 fade-in">
      <DashboardHero icon={Store} badge={t("store_home_badge")} name={name} subtitle={t("store_subtitle")} className="" />

      {/* Work queue — the only band allowed to turn red, and only on real work. */}
      <section>
        <BandLabel>{t("store_needs_action")}</BandLabel>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
          {isLoading ? (
            <>
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
              <Skeleton className="h-28 w-full" />
            </>
          ) : (
            <>
              <QueueCardLink status="pending"><CompactQueue label={t("store_stat_needs_confirm")} value={buckets?.needs_confirm ?? 0} /></QueueCardLink>
              <QueueCardLink queue="ready_to_ship"><CompactQueue label={t("store_stat_ready_to_ship")} value={buckets?.ready_to_ship ?? 0} /></QueueCardLink>
              <QueueCardLink queue="shipment_failed"><CompactQueue label={t("store_stat_shipment_failed")} value={buckets?.shipment_failed ?? 0} error={(buckets?.shipment_failed ?? 0) > 0} /></QueueCardLink>
            </>
          )}
        </div>
      </section>

      <section aria-label={t("store_volume_month")} className="grid grid-cols-2 gap-3">
        {isLoading ? <>
          <Skeleton className="h-28 w-full" />
          <Skeleton className="h-28 w-full" />
        </> : <>
          <StatCard tone="ember" icon={ShoppingBag} label={t("store_stat_orders_this_month")} value={String(buckets?.created_this_month ?? 0)} />
          <StatCard tone="teal" icon={CheckCircle2} label={t("store_stat_completed_this_month")} value={String(buckets?.completed_this_month ?? 0)} />
        </>}
      </section>

      <UnitSalesBlock />

      <div className="grid gap-4 lg:grid-cols-[1fr_2fr]">
        <section aria-labelledby="store-promos-title" className="md-card-outlined flex flex-col gap-3">
          <h2 id="store-promos-title" className="type-h2">{t("store_promos")}</h2>
          <p className="flex justify-between border-t border-line pt-3"><span>{t("store_promos_active")}</span><strong data-testid="store-promos-active" className="tabular-nums">{activePromos.length}</strong></p>
          <p className="flex justify-between border-t border-line pt-3"><span>{t("store_promos_expiring")}</span><strong data-testid="store-promos-expiring" className="tabular-nums">{expiringPromos.length}</strong></p>
          <Link href="/admin/promos" className="inline-block type-label-sm text-brand-ink hover:underline focus-visible:outline">{t("store_view_promos")}</Link>
        </section>
        <section aria-labelledby="store-top-title" className="md-card-outlined">
          <h2 id="store-top-title" className="type-h2 mb-4">{t("store_top_products")}</h2>
          {isLoading ? <Skeleton className="h-40 w-full" /> : !summary?.top_products.length ? <p className="py-6 text-center text-sm text-ink-600">{t("store_top_empty")}</p> :
            <ol>{summary.top_products.map((product, index) => <li key={product.product_id} className="grid grid-cols-[2rem_1fr_auto] items-center gap-3 border-t border-line py-3">
              <span className="grid size-[26px] place-items-center rounded-[9px] bg-surface-sunken text-xs font-bold text-ink-700">{index + 1}</span>
              <div><p className="text-sm font-bold text-ink-900">{product.name}</p><p className="type-label-sm text-ink-600">{typeLabels[product.product_type] ?? product.product_type}</p></div>
              <div className="text-right"><strong className="font-[family-name:var(--font-display)] text-lg font-extrabold tabular-nums">{product.qty_sold}</strong><p className="type-label-sm text-ink-600">{t("admin_unit_net_units")}</p></div>
            </li>)}</ol>}
        </section>
      </div>
    </div>
  );
}
