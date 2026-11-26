"use client";

import { Suspense, use, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { DataTable } from "@/components/ui/data-table";
import { Button } from "@/components/ui/button";
import { useAdminUnitSales, downloadUnitSalesCsv } from "@/lib/hooks/admin-unit-sales";
import { parseUnitPeriod, unitPeriodSearch, type UnitSalesPeriod } from "@/lib/unit-sales-period";
import { useTranslation } from "@/lib/i18n";
import type { UnitSalesProductType } from "@/lib/types";

const TYPES: readonly string[] = ["book", "merchandise", "medal", "course", "exam"];
const UNITS = ["paid_units", "refund_units", "net_units"] as const;

export default function UnitSalesDetailPage({ params }: { params: Promise<{ type: string }> }) {
  return <Suspense><UnitSalesDetailRoute params={params} /></Suspense>;
}

function UnitSalesDetailRoute({ params }: { params: Promise<{ type: string }> }) {
  const { type } = use(params);
  const search = useSearchParams();
  const canRead = useHasCapability("orders:read");
  const { t } = useTranslation();
  const period = parseUnitPeriod(search);
  if (!canRead) return <NoAccess />;
  if (!TYPES.includes(type)) return <AdminPageHeader title={t("admin_unit_not_found")} back={{ href: `/admin/store?${unitPeriodSearch(period)}`, label: t("admin_unit_back") }} />;
  return <UnitSalesDetail type={type as UnitSalesProductType} period={period} />;
}

function UnitSalesDetail({ type, period }: { type: UnitSalesProductType; period: UnitSalesPeriod }) {
  const { t } = useTranslation();
  const query = useAdminUnitSales(period.range);
  const selected = query.data?.types.find((row) => row.type === type);
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState(false);
  const periodLabel = period.preset === "all_time" ? t("admin_unit_all_time") :
    `${t(`admin_home_period_${period.preset}`)} · ${period.range.from ?? "…"} – ${period.range.to ?? "…"} · WIB`;

  async function download() {
    setDownloading(true);
    setDownloadError(false);
    try { await downloadUnitSalesCsv(type, period.range); }
    catch { setDownloadError(true); }
    finally { setDownloading(false); }
  }

  return <div className="flex flex-col gap-5 fade-in">
    <AdminPageHeader title={t(`admin_unit_${type}`)} description={t("admin_unit_subtitle")}
      actions={<p className="md-lchip">{periodLabel}</p>}
      back={{ href: `/admin/store?${unitPeriodSearch(period)}`, label: t("admin_unit_back") }} />
    {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
      <div role="alert" className="md-card-outlined"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> : <>
        <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
          <section aria-labelledby="unit-quantities-title" className="md-card-outlined">
            <h2 id="unit-quantities-title" className="type-h2">{t("admin_unit_quantities")}</h2>
            <div className="mt-4 flex items-baseline gap-3 border-b border-line pb-4"><strong className="text-5xl font-bold tabular-nums">{selected?.net_units ?? 0}</strong><span className="type-label-sm text-ink-600">{t("admin_unit_net_units")}</span></div>
            <dl className="mt-4 flex gap-8">{(["paid_units", "refund_units"] as const).map((key) => <div key={key}><dt className="type-label-sm text-ink-600">{t(`admin_unit_${key}`)}</dt><dd className="mt-1 font-[family-name:var(--font-display)] text-2xl font-extrabold tabular-nums">{selected?.[key] ?? 0}</dd></div>)}</dl>
          </section>
          <section aria-labelledby="unit-orders-title" className="md-card-outlined">
            <h2 id="unit-orders-title" className="type-h2">{t("admin_unit_orders")}</h2>
            <dl className="mt-4 flex gap-8">{(["paid_orders", "refund_orders"] as const).map((key) => <div key={key}><dt className="type-label-sm text-ink-600">{t(`admin_unit_${key}`)}</dt><dd className="mt-1 font-[family-name:var(--font-display)] text-2xl font-extrabold tabular-nums">{selected?.[key] ?? 0}</dd></div>)}</dl>
            <p className="mt-4 text-sm text-ink-600">{t("admin_unit_order_hint")}</p>
          </section>
        </div>
        <section aria-labelledby="unit-products-title" className="md-card-outlined flex flex-col gap-4">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div><h2 id="unit-products-title" className="type-h2">{t("admin_unit_breakdown")}</h2><p className="mt-1 text-sm text-ink-600">{t("admin_unit_product_hint")}</p></div>
            <Button disabled={!selected?.products.length || downloading} onClick={download}>{t(downloading ? "admin_unit_downloading" : "admin_unit_download")}</Button>
          </div>
          {downloadError && <p role="alert">{t("admin_unit_download_error")}</p>}
          {!selected?.products.length ? <p className="text-ink-600">{t("admin_unit_empty")}</p> :
            <DataTable rows={selected.products} rowKey={(p) => p.product_id} empty={t("admin_unit_empty")} columns={[
              { key: "name", header: t("admin_unit_product"), strong: true, cell: (p) => p.name },
              ...UNITS.map((key) => ({ key, header: t(`admin_unit_${key}`), align: "right" as const, strong: key === "net_units",
                className: "tabular-nums", cell: (p: (typeof selected.products)[number]) => p[key] })),
            ]} />}
        </section>
      </>}
  </div>;
}
