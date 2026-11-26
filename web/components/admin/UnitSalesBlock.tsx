"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
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

  const href = (type: string) => `/admin/store/units/${type}?${unitPeriodSearch(period)}`;
  const rows = TYPES.map((type) => ({ type, row: query.data?.types.find((item) => item.type === type) }));

  return <section className="md-card-outlined flex flex-col gap-4" aria-labelledby="unit-sales-title">
    <h2 id="unit-sales-title" className="type-h2">{t("admin_unit_title")}</h2>
    <p className="text-sm text-ink-600">{t("admin_unit_explanation")}</p>
    <PeriodBar preset={preset} range={range} allowAllTime resolvedFrom={period.range.from} resolvedTo={period.range.to}
      onChange={handlePeriodChange} />
    {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
      <div role="alert"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> :
      <DataTable
        mobileCards
        rows={rows}
        rowKey={(r) => r.type}
        empty={t("admin_unit_empty")}
        onRowClick={(r) => router.push(href(r.type))}
        columns={[
          { key: "type", header: t("admin_unit_type"), strong: true, cell: (r) => <Link href={href(r.type)}
            aria-label={t("admin_unit_open").replace("{type}", t(`admin_unit_${r.type}`))}>{t(`admin_unit_${r.type}`)}</Link> },
          ...UNITS.map((key) => ({ key, header: t(`admin_unit_${key}`), align: "right" as const, strong: key === "net_units",
            className: "tabular-nums", cell: (r: (typeof rows)[number]) => r.row?.[key] ?? 0 })),
        ]}
      />}
  </section>;
}
