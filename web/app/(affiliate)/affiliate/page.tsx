"use client";

import { useState } from "react";
import { Copy, HandCoins, Landmark, Link2, UserCog, Users, Wallet } from "lucide-react";
import { toast } from "sonner";
import { DashboardHero } from "@/components/admin/DashboardHero";
import { PeriodBar, presetRange, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { QuickActionTiles } from "@/components/admin/QuickActionTiles";
import { StatCard } from "@/components/admin/StatCard";
import { AffiliatePromos } from "@/components/affiliate/AffiliatePromos";
import { LevelBadge } from "@/components/affiliate/LevelBadge";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAffiliateLedger, useAffiliateMe, useAffiliateStats } from "@/lib/hooks/affiliate";
import { useMe } from "@/lib/hooks/auth";
import { ApiError } from "@/lib/api";
import { affLabel } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";

// The stats endpoint is all-time without a range, so the default 30 days
// (which PeriodBar leaves empty) must be spelled out here, in Jakarta time.
function thirtyDaysAgo(): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(new Date(Date.now() - 29 * 86400000));
}

export default function AffiliateDashboardPage() {
  const { t } = useTranslation();
  const me = useAffiliateMe();
  const user = useMe();
  const [preset, setPreset] = useState<PeriodPreset>("30d");
  const [range, setRange] = useState<PeriodRange>({});
  // A half-typed custom range would query all time under a "custom" label, so
  // the query keeps the last complete range until both dates are set.
  const [resolved, setResolved] = useState<PeriodRange>({ from: thirtyDaysAgo() });
  const stats = useAffiliateStats(resolved);
  const ledger = useAffiliateLedger({ per_page: 1 });

  const onPeriod = (next: PeriodPreset, nextRange: PeriodRange) => {
    setPreset(next);
    setRange(nextRange);
    if (next === "custom") {
      if (nextRange.from && nextRange.to) setResolved(nextRange);
      return;
    }
    setResolved(next === "30d" ? { from: thirtyDaysAgo() } : presetRange(next));
  };

  if (me.isLoading) {
    return (
      <div className="space-y-6" role="status" aria-label={t("aff_loading")}>
        <Skeleton className="h-32 w-full rounded-[20px]" />
        <Skeleton className="h-14 w-full" />
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} className="h-28 w-full" />
          ))}
        </div>
        <span className="sr-only">{t("aff_loading")}</span>
      </div>
    );
  }
  if (me.isError || !me.data) {
    if (me.error instanceof ApiError && me.error.status === 404) {
      return <div className="md-card-outlined p-6 text-sm text-ink-600">{t("aff_dash_unavailable")}</div>;
    }
    return (
      <div role="alert" className="md-card-outlined space-y-3 p-6 text-sm text-ink-600">
        <p>{t("aff_error")}</p>
        <Button variant="outline" size="sm" onClick={() => me.refetch()}>
          {t("aff_retry")}
        </Button>
      </div>
    );
  }

  const profile = me.data;
  const active = profile.status === "active";
  const link = profile.referral_link ? `${window.location.origin}${profile.referral_link}` : "";
  const progress = profile.level_progress;
  const noticeKey = `aff_inactive_notice_${profile.status}`;
  const stat = stats.data;
  const remaining = progress?.rise_threshold !== undefined ? progress.rise_threshold - progress.net_revenue : 0;
  const name = user.data?.name ?? user.data?.email ?? "";

  const copyLink = async () => {
    try {
      await navigator.clipboard.writeText(link);
      toast.success(t("aff_dash_copied"));
    } catch {
      toast.error(t("aff_error"));
    }
  };

  return (
    <div className="space-y-6">
      <DashboardHero
        icon={HandCoins}
        badge={t("aff_dash_badge")}
        name={t("aff_dash_hello").replace("{name}", name)}
        subtitle={t("aff_dash_hero_sub")
          .replace("{level}", affLabel(t, profile.level))
          .replace("{code}", profile.code || t("aff_dash_no_code"))}
        className=""
      />

      {!active && (
        <div role="status" className="rounded-lg border border-warn/30 bg-warn-bg px-5 py-4 text-sm text-warn">
          <p>{t(noticeKey as Parameters<typeof t>[0])}</p>
          {profile.rejection_reason && (
            <p className="mt-1">{t("aff_inactive_reason").replace("{reason}", profile.rejection_reason)}</p>
          )}
        </div>
      )}

      <section className="md-card-outlined space-y-3 p-5">
        <div className="flex flex-wrap items-center gap-3">
          <h2 className="font-semibold text-ink-900">{t("aff_dash_link_section")}</h2>
          <StatusBadge status={profile.status} />
          <LevelBadge level={profile.level} />
        </div>
        <div>
          <div className="text-xs text-ink-500">{t("aff_dash_code")}</div>
          <div className="font-mono text-xl font-bold tracking-wider text-ink-900">
            {profile.code || t("aff_dash_no_code")}
          </div>
        </div>
        {link && (
          <div className="flex flex-wrap items-center gap-3">
            <div className="min-w-0 flex-1">
              <div className="text-xs text-ink-500">{t("aff_dash_link")}</div>
              <div className="truncate text-sm text-ink-700">{link}</div>
            </div>
            <Button variant="outline" size="sm" onClick={copyLink}>
              <Copy className="size-4" />
              <span className="ml-2">{t("aff_dash_copy")}</span>
            </Button>
          </div>
        )}
      </section>

      <PeriodBar preset={preset} range={range} onChange={onPeriod} />

      {stats.isError && !stat ? (
        <div role="alert" className="md-card-outlined space-y-3 p-6 text-sm text-ink-600">
          <p>{t("aff_error")}</p>
          <Button variant="outline" size="sm" onClick={() => stats.refetch()}>
            {t("aff_retry")}
          </Button>
        </div>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <StatCard label={t("aff_dash_balance")} value={ledger.data ? formatRupiah(ledger.data.saldo) : "—"} icon={Wallet} accent="primary" />
          <StatCard label={t("aff_dash_commission")} value={stat ? formatRupiah(stat.commission_earned) : "—"} icon={Landmark} accent="tertiary" />
          <StatCard label={t("aff_dash_new_students")} value={stat ? String(stat.new_students) : "—"} icon={Users} accent="secondary" />
          <StatCard label={t("aff_dash_active_bindings")} value={stat ? String(stat.active_bindings) : "—"} icon={Users} accent="tertiary" />
        </div>
      )}

      {active && <AffiliatePromos />}

      <div className="grid gap-4 lg:grid-cols-2">
        <section className="md-card-outlined space-y-1 p-5 text-sm text-ink-700">
          <div className="text-xs text-ink-500">{t("aff_dash_level")}</div>
          <div><LevelBadge level={profile.level} /></div>
          {progress && (
            <>
              <p>
                {t("aff_dash_progress")
                  .replace("{quarter}", progress.quarter)
                  .replace("{amount}", formatRupiah(progress.net_revenue))}
              </p>
              {progress.next_level && progress.rise_threshold !== undefined && (
                <p>
                  {(remaining > 0 ? t("aff_dash_rise") : t("aff_dash_rise_done"))
                    .replace("{level}", affLabel(t, progress.next_level))
                    .replace("{amount}", formatRupiah(Math.max(0, remaining)))}
                </p>
              )}
              {progress.hold_threshold !== undefined && (
                <p>{t("aff_dash_hold").replace("{amount}", formatRupiah(progress.hold_threshold))}</p>
              )}
            </>
          )}
          {profile.warn_quarters > 0 && (
            <p className="text-warn">{t("aff_dash_warn").replace("{n}", String(profile.warn_quarters))}</p>
          )}
        </section>

        <QuickActionTiles
          title={t("aff_dash_actions")}
          actions={[
            { icon: Wallet, label: t("aff_dash_action_payout"), href: "/affiliate/wallet" },
            { icon: Link2, label: t("aff_dash_action_commissions"), href: "/affiliate/commissions" },
            { icon: Users, label: t("aff_dash_action_students"), href: "/affiliate/students" },
            { icon: UserCog, label: t("aff_dash_action_account"), href: "/affiliate/account" },
          ]}
        />
      </div>
    </div>
  );
}
