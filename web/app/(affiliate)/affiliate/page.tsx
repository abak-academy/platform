"use client";

import { Copy } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { useAffiliateLedger, useAffiliateMe, useAffiliateStats } from "@/lib/hooks/affiliate";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="md-card-outlined p-4">
      <div className="text-xs text-ink-500">{label}</div>
      <div className="mt-1 font-serif text-2xl font-bold text-ink-900">{value}</div>
    </div>
  );
}

export default function AffiliateDashboardPage() {
  const { t } = useTranslation();
  const me = useAffiliateMe();
  const stats = useAffiliateStats();
  const ledger = useAffiliateLedger({ per_page: 1 });

  if (me.isLoading) return <div className="text-sm text-ink-500">{t("aff_loading")}</div>;
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
      <div className="flex items-center gap-3">
        <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_dash_title")}</h1>
        <StatusBadge status={profile.status} />
      </div>

      {!active && (
        <div role="status" className="rounded-lg border border-warn/30 bg-warn-bg px-5 py-4 text-sm text-warn">
          <p>{t(noticeKey as Parameters<typeof t>[0])}</p>
          {profile.rejection_reason && (
            <p className="mt-1">{t("aff_inactive_reason").replace("{reason}", profile.rejection_reason)}</p>
          )}
          {profile.status === "suspended" && profile.suspended_until && (
            <p className="mt-1">{formatAffDate(profile.suspended_until)}</p>
          )}
        </div>
      )}

      <section className="md-card-outlined space-y-3 p-5">
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

      <div className="grid gap-4 sm:grid-cols-2">
        <section className="md-card-outlined space-y-1 p-5 text-sm text-ink-700">
          <div className="text-xs text-ink-500">{t("aff_dash_level")}</div>
          <div className="font-serif text-xl font-bold text-ink-900">{affLabel(t, profile.level)}</div>
          {progress && (
            <>
              <p>
                {t("aff_dash_progress")
                  .replace("{quarter}", progress.quarter)
                  .replace("{amount}", formatRupiah(progress.net_revenue))}
              </p>
              {progress.next_level && progress.rise_threshold !== undefined && (
                <p>
                  {t("aff_dash_rise")
                    .replace("{level}", affLabel(t, progress.next_level))
                    .replace("{amount}", formatRupiah(progress.rise_threshold))}
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
          {profile.level_locked_until && (
            <p>{t("aff_dash_locked").replace("{date}", formatAffDate(profile.level_locked_until))}</p>
          )}
        </section>

        <section className="md-card-outlined space-y-1 p-5 text-sm text-ink-700">
          <div className="text-xs text-ink-500">{t("aff_dash_balance")}</div>
          <div className="font-serif text-xl font-bold text-ink-900">
            {ledger.data ? formatRupiah(ledger.data.saldo) : "—"}
          </div>
          {profile.frozen_until && (
            <p className="text-warn">{t("aff_dash_frozen").replace("{date}", formatAffDate(profile.frozen_until))}</p>
          )}
        </section>
      </div>

      {stats.data && (
        <div className="grid grid-cols-2 gap-4 lg:grid-cols-5">
          <Stat label={t("aff_dash_clicks")} value={String(stats.data.clicks)} />
          <Stat label={t("aff_dash_registrations")} value={String(stats.data.registrations)} />
          <Stat label={t("aff_dash_conversion")} value={`${Math.round(stats.data.conversion * 100)}%`} />
          <Stat label={t("aff_dash_students")} value={String(stats.data.total_students)} />
          <Stat label={t("aff_dash_active_bindings")} value={String(stats.data.active_bindings)} />
        </div>
      )}
    </div>
  );
}
