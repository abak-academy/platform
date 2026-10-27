"use client";

import { useState } from "react";
import Link from "next/link";
import { CircleDollarSign, Clock, Handshake, Package, ReceiptText, Tag, Users } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorNotice } from "@/components/admin/AffiliatorNotice";
import { NoAccess } from "@/components/admin/NoAccess";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { StatCard } from "@/components/admin/StatCard";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { useAffiliatorDashboard } from "@/lib/hooks/affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";
import type { AffiliatorMonthPoint, AffiliatorSchoolCount, AffiliatorSchoolProduct } from "@/lib/types";

function jakartaDaysAgo(days: number): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(new Date(Date.now() - days * 86400000));
}

// The affiliator endpoint reads no dates as all time, unlike the admin dashboards
// where no dates means the last 30 days, so the 30-day preset is spelled out.
function queryRange(preset: PeriodPreset, range: PeriodRange): PeriodRange {
  return preset === "30d" ? { from: jakartaDaysAgo(29) } : range;
}

export default function AffiliateDashboardPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("affiliates:self");
  const [preset, setPreset] = useState<PeriodPreset>("all_time");
  const [range, setRange] = useState<PeriodRange>({});
  // Hold the query back while a custom range is half typed, like the admin dashboards.
  const [resolved, setResolved] = useState<PeriodRange>({});

  const { data, isLoading, isError, refetch } = useAffiliatorDashboard(resolved);

  if (!allowed) return <NoAccess />;

  function handlePeriod(next: PeriodPreset, nextRange: PeriodRange) {
    setPreset(next);
    setRange(nextRange);
    if (next !== "custom" || (nextRange.from && nextRange.to)) setResolved(queryRange(next, nextRange));
  }

  const schoolColumns: DataTableColumn<AffiliatorSchoolCount>[] = [
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name },
    { key: "students", header: t("aff_th_students"), align: "right", cell: (r) => r.student_count },
  ];
  const monthColumns: DataTableColumn<AffiliatorMonthPoint>[] = [
    { key: "month", header: t("aff_th_month"), cell: (r) => r.month },
    { key: "orders", header: t("aff_th_orders"), align: "right", cell: (r) => r.orders },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.units },
    { key: "amount", header: t("aff_th_amount"), align: "right", cell: (r) => formatRupiah(r.amount) },
  ];
  const topColumns: DataTableColumn<AffiliatorSchoolProduct>[] = [
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name },
    { key: "product", header: t("aff_th_product"), cell: (r) => `${r.product_name} (${t(r.product_type === "exam" ? "product_type_exam" : "product_type_course")})` },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.units },
  ];

  const totals = data?.totals;
  const studentTotal = (data?.schools ?? []).reduce((sum, s) => sum + s.student_count, 0);

  return (
    <div className="fade-in space-y-8">
      <AdminPageHeader
        icon={Handshake}
        title={t("aff_home_title")}
        description={t("aff_home_subtitle")}
        actions={
          <Link href="/admin/affiliate/orders/new" className="md-btn-filled">
            {t("aff_order_new")}
          </Link>
        }
      />
      <AffiliatorNotice />

      <PeriodBar preset={preset} range={range} onChange={handlePeriod} allowAllTime />

      {isError ? (
        <div className="md-card-outlined py-12 text-center text-ink-500">
          <p className="mb-4">{t("dash_load_failed")}</p>
          <button type="button" className="md-btn-tonal" onClick={() => refetch()}>
            {t("admin_home_reload")}
          </button>
        </div>
      ) : isLoading || !data ? (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <Skeleton key={i} className="h-28 w-full" />
          ))}
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 xl:grid-cols-3">
            <StatCard label={t("aff_stat_students")} value={String(studentTotal)} icon={Users} />
            <StatCard label={t("aff_stat_paid_orders")} value={String(totals?.paid_orders ?? 0)} icon={ReceiptText} accent="secondary" />
            <StatCard label={t("aff_stat_units")} value={String(totals?.paid_units ?? 0)} icon={Package} accent="tertiary" />
            <StatCard label={t("aff_stat_paid_amount")} value={formatRupiah(totals?.paid_amount ?? 0)} icon={CircleDollarSign} />
            <StatCard label={t("aff_stat_discount")} value={formatRupiah(totals?.discount_total ?? 0)} icon={Tag} accent="secondary" />
            <StatCard label={t("aff_stat_pending")} value={String(totals?.pending_orders ?? 0)} icon={Clock} accent="tertiary" />
          </div>

          <div className="md-card-outlined flex flex-wrap items-center justify-between gap-3 px-4 py-3">
            <p className="text-sm">
              <strong>{data.unpurchased_students}</strong> {t("aff_stat_unpurchased")}
            </p>
            <Link href="/admin/affiliate/students?not_purchased=true" className="md-btn-tonal">
              {t("aff_view_unpurchased")}
            </Link>
          </div>

          <section aria-labelledby="aff-schools" className="space-y-3">
            <h2 id="aff-schools" className="text-title-large">{t("aff_section_schools")}</h2>
            <DataTable columns={schoolColumns} rows={data.schools} rowKey={(r) => r.school_id} empty={t("aff_empty_schools")} />
          </section>

          <section aria-labelledby="aff-trend" className="space-y-3">
            <h2 id="aff-trend" className="text-title-large">{t("aff_section_trend")}</h2>
            <DataTable columns={monthColumns} rows={data.monthly} rowKey={(r) => r.month} empty={t("aff_empty_sales")} />
          </section>

          <section aria-labelledby="aff-top" className="space-y-3">
            <h2 id="aff-top" className="text-title-large">{t("aff_section_top")}</h2>
            <DataTable columns={topColumns} rows={data.top_products} rowKey={(r) => `${r.school_id}:${r.product_id}`} empty={t("aff_empty_sales")} />
          </section>
        </>
      )}
    </div>
  );
}
