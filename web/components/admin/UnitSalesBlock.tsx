"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { Button } from "@/components/ui/button";
import { useAdminUnitSales } from "@/lib/hooks/admin-unit-sales";
import { explicitUnitRange, parseUnitPeriod, unitPeriodSearch } from "@/lib/unit-sales-period";
import { useTranslation } from "@/lib/i18n";

const TYPES = ["book", "merchandise", "medal", "course", "exam"] as const;
const UNITS = ["paid_units", "refund_units", "net_units"] as const;

export function UnitSalesBlock() {
  return <Suspense><UnitSalesOverview /></Suspense>;
}

function UnitSalesOverview() {
  const { t } = useTranslation();
  const params = useSearchParams();
  const router = useRouter();
  const search = params.toString();
  const previousSearch = useRef(search);
  const [period, setPeriod] = useState(() => parseUnitPeriod(params));
  const [range, setRange] = useState<PeriodRange>(period.range);
  const [preset, setPreset] = useState<PeriodPreset>(period.preset);
  const query = useAdminUnitSales(period.range);

  useEffect(() => {
    if (previousSearch.current === search) return;
    previousSearch.current = search;
    const parsed = parseUnitPeriod(new URLSearchParams(search));
    if (parsed.preset === period.preset && parsed.range.from === period.range.from && parsed.range.to === period.range.to) return;
    setPeriod(parsed);
    setPreset(parsed.preset);
    setRange(parsed.range);
  }, [search, period]);

  function handlePeriodChange(next: PeriodPreset, nextRange: PeriodRange) {
    const resolved = explicitUnitRange(next, nextRange);
    setPreset(next);
    setRange(resolved);
    const empty = next === "custom" && !resolved.from && !resolved.to;
    const reversed = !!resolved.from && !!resolved.to && resolved.from > resolved.to;
    if (empty || reversed) return;
    const applied = { preset: next, range: resolved };
    setPeriod(applied);
    router.replace(`/admin/store?${unitPeriodSearch(applied)}`, { scroll: false });
  }

  return <section className="md-card-elevated space-y-4" aria-labelledby="unit-sales-title">
    <h2 id="unit-sales-title" className="text-title-medium">{t("admin_unit_title")}</h2>
    <p className="text-sm" style={{ color: "var(--md-sys-color-on-surface)" }}>{t("admin_unit_explanation")}</p>
    <PeriodBar preset={preset} range={range} allowAllTime resolvedFrom={period.range.from} resolvedTo={period.range.to}
      onChange={handlePeriodChange} />
    {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
      <div role="alert"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> :
      <div><table className="block w-full text-sm sm:table">
        <thead className="sr-only sm:not-sr-only sm:table-header-group"><tr><th scope="col" className="p-3 text-left">{t("admin_unit_type")}</th>
          {UNITS.map((key) => <th key={key} scope="col" className="p-3 text-right">{t(`admin_unit_${key}`)}</th>)}
        </tr></thead>
        <tbody className="block sm:table-row-group">{TYPES.map((type) => {
          const row = query.data?.types.find((item) => item.type === type);
          return <tr key={type} className="group relative grid grid-cols-3 sm:table-row border-t border-line hover:bg-muted focus-within:outline focus-within:outline-2 focus-within:outline-offset-[-2px]" style={{ outlineColor: "var(--md-sys-color-primary)" }}>
            <td className="col-span-3 block p-3 text-left font-medium sm:table-cell"><Link
              href={`/admin/store/units/${type}?${unitPeriodSearch(period)}`}
              aria-label={t("admin_unit_open").replace("{type}", t(`admin_unit_${type}`))}
              className="after:absolute after:inset-0 after:z-10 focus-visible:outline-none">
              {t(`admin_unit_${type}`)}
            </Link></td>
            {UNITS.map((key) => <td key={key} className={`block min-w-0 px-1 py-3 sm:table-cell sm:p-3 text-right tabular-nums ${key === "net_units" ? "font-bold text-title-large" : ""}`}><span aria-hidden="true" className="block text-xs font-normal sm:hidden" style={{ color: "var(--md-sys-color-on-surface)" }}>{t(`admin_unit_${key}`)}</span>{row?.[key] ?? 0}</td>)}
          </tr>;
        })}</tbody>
      </table></div>}
  </section>;
}
