"use client";

import Link from "next/link";
import { GraduationCap, ShoppingCart, Ticket, Package } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { Skeleton } from "@/components/ui/skeleton";
import { usePartnerDashboard } from "@/lib/hooks/partner";
import { useTranslation, type I18nKey } from "@/lib/i18n";

const SHORTCUTS: { href: string; icon: typeof GraduationCap; title: I18nKey; desc: I18nKey }[] = [
  { href: "/affiliate/partner/orders/new", icon: ShoppingCart, title: "partner_dash_shortcut_orders", desc: "partner_dash_shortcut_orders_desc" },
  { href: "/affiliate/partner/students", icon: GraduationCap, title: "partner_dash_shortcut_students", desc: "partner_dash_shortcut_students_desc" },
  { href: "/affiliate/partner/orders", icon: Package, title: "partner_dash_shortcut_history", desc: "partner_dash_shortcut_history_desc" },
];

export default function PartnerDashboardPage() {
  const { t } = useTranslation();
  const dash = usePartnerDashboard();
  const d = dash.data;

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={GraduationCap} title={t("partner_dash_title")} description={t("partner_dash_desc")} />
      {dash.isLoading ? (
        <Skeleton className="h-28 w-full" />
      ) : dash.isError ? (
        <p role="alert" className="text-sm text-danger">
          {t("partner_load_failed")}
        </p>
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <StatCard label={t("partner_stat_students")} value={String(d?.students_registered ?? 0)} icon={GraduationCap} />
          <StatCard label={t("partner_stat_orders_paid")} value={String(d?.orders_paid ?? 0)} icon={ShoppingCart} accent="secondary" />
          <StatCard label={t("partner_stat_orders_unpaid")} value={String(d?.orders_unpaid ?? 0)} icon={ShoppingCart} accent="tertiary" />
          <StatCard label={t("partner_stat_vouchers")} value={String(d?.vouchers_available ?? 0)} icon={Ticket} />
          <StatCard label={t("partner_stat_products")} value={String(d?.products_open ?? 0)} icon={Package} accent="secondary" />
        </div>
      )}
      <section aria-label={t("partner_dash_shortcuts")} className="space-y-3">
        <h2 className="font-semibold text-ink-900">{t("partner_dash_shortcuts")}</h2>
        <div className="grid gap-3 sm:grid-cols-3">
          {SHORTCUTS.map((s) => (
            <Link key={s.href} href={s.href} className="md-card-outlined flex items-start gap-3 p-4 transition-colors hover:bg-surface-2">
              <s.icon className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden />
              <span>
                <span className="block font-medium text-ink-900">{t(s.title)}</span>
                <span className="block text-xs text-ink-500">{t(s.desc)}</span>
              </span>
            </Link>
          ))}
        </div>
      </section>
    </div>
  );
}
