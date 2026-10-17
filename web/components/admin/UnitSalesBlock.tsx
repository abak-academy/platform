"use client";

import { useRef, useState } from "react";
import { PeriodBar, presetRange, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { Dialog, DialogClose, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useAdminUnitSales, downloadUnitSalesCsv } from "@/lib/hooks/admin-unit-sales";
import { useTranslation } from "@/lib/i18n";
import type { UnitSalesFigures, UnitSalesProductType } from "@/lib/types";

const FIGURES = ["paid_orders", "refund_orders", "paid_units", "refund_units", "net_units"] as const;

function explicitRange(preset: PeriodPreset, range: PeriodRange): PeriodRange {
  if (preset === "all_time") return {};
  if (preset === "custom") return range;
  const date = (days: number) => new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(new Date(Date.now() - days * 86400000));
  return { from: preset === "30d" ? date(29) : presetRange(preset).from, to: date(0) };
}

export function UnitSalesBlock() {
  const { t } = useTranslation();
  const [preset, setPreset] = useState<PeriodPreset>("30d");
  const [range, setRange] = useState<PeriodRange>(() => explicitRange("30d", {}));
  const [applied, setApplied] = useState<PeriodRange>(range);
  const [openType, setOpenType] = useState<UnitSalesProductType | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [downloadError, setDownloadError] = useState(false);
  const opener = useRef<HTMLButtonElement | null>(null);
  const query = useAdminUnitSales(applied);
  const selected = query.data?.types.find((row) => row.type === openType);

  function figures(values: UnitSalesFigures, id?: string) {
    return <span id={id} className="grid grid-cols-2 gap-x-5 gap-y-2 sm:grid-cols-5">
      {FIGURES.map((key) => <span key={key} className="block">
        <span className="block text-label color-on-surface-variant">{t(`admin_unit_${key}`)}</span>
        <span className="block text-title-medium tabular-nums">{values[key]}</span>
      </span>)}
    </span>;
  }

  function handlePeriodChange(next: PeriodPreset, nextRange: PeriodRange) {
    const resolved = explicitRange(next, nextRange);
    setPreset(next);
    setRange(resolved);
    const empty = next === "custom" && !resolved.from && !resolved.to;
    const reversed = !!resolved.from && !!resolved.to && resolved.from > resolved.to;
    if (!empty && !reversed) setApplied(resolved);
  }

  async function download() {
    if (!selected) return;
    setDownloading(true);
    setDownloadError(false);
    try { await downloadUnitSalesCsv(selected.type, applied); }
    catch { setDownloadError(true); }
    finally { setDownloading(false); }
  }

  return <section className="space-y-4" aria-labelledby="unit-sales-title">
    <h2 id="unit-sales-title" className="text-title-medium">{t("admin_unit_title")}</h2>
    <PeriodBar preset={preset} range={range} allowAllTime resolvedFrom={applied.from} resolvedTo={applied.to}
      onChange={handlePeriodChange} />
    {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
      <div role="alert"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> :
      <div className="divide-y border border-line rounded-xl overflow-hidden">
        {query.data?.types.map((row) => <button key={row.type} type="button"
          className="w-full p-4 text-left space-y-3 hover:bg-muted focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px]"
          aria-label={t("admin_unit_open").replace("{type}", t(`admin_unit_${row.type}`))}
          aria-describedby={`unit-sales-${row.type}-figures`}
          onClick={(event) => { opener.current = event.currentTarget; setDownloadError(false); setOpenType(row.type); }}>
          <span className="text-title-small">{t(`admin_unit_${row.type}`)}</span>
          {figures(row, `unit-sales-${row.type}-figures`)}
        </button>)}
      </div>}
    <Dialog open={openType !== null} onOpenChange={(open) => { if (!open) setOpenType(null); }}>
      <DialogContent className="sm:max-w-4xl" showCloseButton={false}
        onCloseAutoFocus={(event) => { event.preventDefault(); opener.current?.focus(); }}>
        <DialogTitle>{openType ? t(`admin_unit_${openType}`) : t("admin_unit_title")}</DialogTitle>
        <DialogDescription>{t("admin_unit_description")}</DialogDescription>
        {selected && <>
          <div data-testid="unit-sales-total" className="space-y-2">
            <h3 className="text-label">{t("admin_unit_total")}</h3>
            {figures(selected)}
          </div>
          {selected.products.length === 0 ? <p>{t("admin_unit_empty")}</p> :
            <div className="overflow-x-auto"><table className="w-full text-sm">
              <thead><tr><th scope="col" className="p-2 text-left">{t("admin_unit_product")}</th>
                {FIGURES.map((key) => <th key={key} scope="col" className="p-2 text-right">{t(`admin_unit_${key}`)}</th>)}
              </tr></thead>
              <tbody>{selected.products.map((product) => <tr key={product.product_id} className="border-t border-line">
                <th scope="row" className="p-2 text-left font-normal">{product.name}</th>
                {FIGURES.map((key) => <td key={key} className="p-2 text-right tabular-nums">{product[key]}</td>)}
              </tr>)}</tbody>
            </table></div>}
          {downloadError && <p role="alert">{t("admin_unit_download_error")}</p>}
          <Button disabled={selected.products.length === 0 || downloading} onClick={download}>
            {t(downloading ? "admin_unit_downloading" : "admin_unit_download")}
          </Button>
        </>}
        <DialogClose asChild><Button variant="outline">{t("admin_unit_close")}</Button></DialogClose>
      </DialogContent>
    </Dialog>
  </section>;
}
