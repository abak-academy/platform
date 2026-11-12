"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMe } from "@/lib/hooks/auth";
import { useAffiliateStats } from "@/lib/hooks/affiliate";
import { usePartnerDashboard } from "@/lib/hooks/partner";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { Skeleton } from "@/components/ui/skeleton";
import type { ProgramState } from "@/lib/types";

function StateNotice({ state }: { state: ProgramState }) {
  const { t } = useTranslation();
  if (state === "pending") return <p className="text-sm text-ink-600">{t("prog_pending_desc")}</p>;
  if (state === "rejected") return <p className="text-sm text-ink-600">{t("prog_rejected_desc")}</p>;
  return <p className="text-sm text-ink-600">{t("prog_not_enrolled_desc")}</p>;
}

function Figure({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <dt className="text-xs text-ink-500">{label}</dt>
      <dd className="text-lg font-semibold text-ink-900">{value}</dd>
    </div>
  );
}

function AffiliateFigures() {
  const { t } = useTranslation();
  const stats = useAffiliateStats();
  if (stats.isLoading) return <Skeleton className="h-12 w-full" />;
  return (
    <dl className="grid grid-cols-2 gap-4">
      <Figure label={t("prog_card_bound")} value={String(stats.data?.active_bindings ?? 0)} />
      <Figure label={t("prog_card_earned")} value={formatRupiah(stats.data?.commission_earned ?? 0)} />
    </dl>
  );
}

function PartnerFigures() {
  const { t } = useTranslation();
  const dash = usePartnerDashboard();
  if (dash.isLoading) return <Skeleton className="h-12 w-full" />;
  return (
    <dl className="grid grid-cols-2 gap-4">
      <Figure label={t("prog_card_students")} value={String(dash.data?.students_registered ?? 0)} />
      <Figure label={t("prog_card_orders")} value={String(dash.data?.orders_paid ?? 0)} />
    </dl>
  );
}

function ProgramCard({ title, href, state, children }: { title: string; href: string; state: ProgramState; children: React.ReactNode }) {
  const { t } = useTranslation();
  return (
    <section className="md-card-outlined space-y-4 p-6">
      <h2 className="font-serif text-lg font-bold text-ink-900">{title}</h2>
      {state === "active" ? children : <StateNotice state={state} />}
      <Link href={href} className="inline-block text-sm font-semibold text-success underline">
        {t("prog_card_open")}
      </Link>
    </section>
  );
}

// The portal's front door. An account in one program goes straight to it; an
// account in both sees one card each; the menu lists both either way.
export default function AffiliateLandingPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const me = useMe();
  const states = me.data?.affiliator_programs;

  const inAffiliate = states ? states.affiliate !== "not_enrolled" : false;
  const inPartner = states ? states.partner !== "not_enrolled" : false;

  useEffect(() => {
    if (!states) return;
    if (inAffiliate && !inPartner) router.replace("/affiliate/overview");
    else if (inPartner && !inAffiliate) router.replace("/affiliate/partner");
  }, [states, inAffiliate, inPartner, router]);

  if (me.isLoading || (states && inAffiliate !== inPartner)) {
    return <Skeleton className="h-40 w-full" data-testid="landing-loading" />;
  }
  if (!states || (!inAffiliate && !inPartner)) {
    return (
      <div role="status" className="md-card-outlined mx-auto mt-10 max-w-xl p-8 text-center text-sm text-ink-600">
        {t("prog_landing_none")}
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-serif text-2xl font-bold text-ink-900">{t("prog_landing_title")}</h1>
        <p className="text-sm text-ink-600">{t("prog_landing_desc")}</p>
      </div>
      <div className="grid gap-4 md:grid-cols-2">
        <ProgramCard title={t("aff_nav_group_affiliator")} href="/affiliate/overview" state={states.affiliate}>
          <AffiliateFigures />
        </ProgramCard>
        <ProgramCard title={t("aff_nav_group_partner")} href="/affiliate/partner" state={states.partner}>
          <PartnerFigures />
        </ProgramCard>
      </div>
    </div>
  );
}
