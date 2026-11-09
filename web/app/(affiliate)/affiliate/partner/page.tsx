"use client";

import Link from "next/link";
import { GraduationCap, ShoppingCart, Ticket, Package } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { usePartnerDashboard } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";

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
      <div className="flex flex-wrap gap-3">
        <Button asChild>
          <Link href="/affiliate/partner/orders/new">{t("partner_dash_cta_order")}</Link>
        </Button>
        <Button asChild variant="outline">
          <Link href="/affiliate/partner/students">{t("partner_dash_cta_students")}</Link>
        </Button>
      </div>
    </div>
  );
}
