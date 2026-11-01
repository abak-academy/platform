"use client";

import { Suspense, use, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { NoAccess } from "@/components/admin/NoAccess";
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
  if (!TYPES.includes(type)) return <div className="md-card-elevated space-y-4">
    <Link href={`/admin/store?${unitPeriodSearch(period)}`} className="color-primary focus-visible:outline">{t("admin_unit_back")}</Link>
    <h1 className="text-title-large">{t("admin_unit_not_found")}</h1>
  </div>;
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

  return <div className="space-y-6 fade-in">
    <Link href={`/admin/store?${unitPeriodSearch(period)}`} className="inline-block text-label color-primary hover:underline focus-visible:outline">{t("admin_unit_back")}</Link>
    <header className="flex flex-wrap items-end justify-between gap-4">
      <div><h1 className="text-headline">{t(`admin_unit_${type}`)}</h1><p className="mt-2 text-sm" style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_subtitle")}</p></div>
      <p className="md-card-elevated !p-3 text-label" style={{ color: "var(--md-sys-color-on-surface)" }}>{periodLabel}</p>
    </header>
    {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
      <div role="alert" className="md-card-elevated"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> : <>
        <div className="grid gap-4 lg:grid-cols-[1.4fr_1fr]">
          <section aria-labelledby="unit-quantities-title" className="md-card-elevated">
            <h2 id="unit-quantities-title" className="text-title-medium">{t("admin_unit_quantities")}</h2>
            <div className="mt-4 flex items-baseline gap-3 border-b border-line pb-4"><strong className="text-5xl font-bold tabular-nums">{selected?.net_units ?? 0}</strong><span className="text-label" style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_net_units")}</span></div>
            <dl className="mt-4 flex gap-8">{(["paid_units", "refund_units"] as const).map((key) => <div key={key}><dt className="text-label" style={{ color: "var(--md-sys-color-on-surface)" }}>{t(`admin_unit_${key}`)}</dt><dd className="mt-1 text-title-large tabular-nums">{selected?.[key] ?? 0}</dd></div>)}</dl>
          </section>
          <section aria-labelledby="unit-orders-title" className="md-card-elevated">
            <h2 id="unit-orders-title" className="text-title-medium">{t("admin_unit_orders")}</h2>
            <dl className="mt-4 flex gap-8">{(["paid_orders", "refund_orders"] as const).map((key) => <div key={key}><dt className="text-label" style={{ color: "var(--md-sys-color-on-surface)" }}>{t(`admin_unit_${key}`)}</dt><dd className="mt-1 text-title-large tabular-nums">{selected?.[key] ?? 0}</dd></div>)}</dl>
            <p className="mt-4 text-sm" style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_order_hint")}</p>
          </section>
        </div>
        <section aria-labelledby="unit-products-title" className="md-card-elevated space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-4">
            <div><h2 id="unit-products-title" className="text-title-medium">{t("admin_unit_breakdown")}</h2><p className="mt-1 text-sm" style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_product_hint")}</p></div>
            <Button disabled={!selected?.products.length || downloading} onClick={download}>{t(downloading ? "admin_unit_downloading" : "admin_unit_download")}</Button>
          </div>
          {downloadError && <p role="alert">{t("admin_unit_download_error")}</p>}
          {!selected?.products.length ? <p style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_empty")}</p> :
            <div><table className="block w-full text-sm sm:table">
              <thead className="sr-only sm:not-sr-only sm:table-header-group"><tr><th scope="col" className="p-3 text-left">{t("admin_unit_product")}</th>{UNITS.map((key) => <th key={key} scope="col" className="p-3 text-right">{t(`admin_unit_${key}`)}</th>)}</tr></thead>
              <tbody className="block sm:table-row-group">{selected.products.map((product) => <tr key={product.product_id} className="grid grid-cols-3 sm:table-row border-t border-line"><th scope="row" className="col-span-3 block p-3 text-left font-medium sm:table-cell">{product.name}</th>{UNITS.map((key) => <td key={key} className={`block min-w-0 px-1 py-3 sm:table-cell sm:p-3 text-right tabular-nums ${key === "net_units" ? "font-bold text-xl sm:text-sm" : ""}`}><span aria-hidden="true" className="block text-xs font-normal sm:hidden" style={{ color: "var(--md-sys-color-on-surface)" }}>{t(`admin_unit_${key}`)}</span>{product[key]}</td>)}</tr>)}</tbody>
            </table></div>}
        </section>
      </>}
  </div>;
}
