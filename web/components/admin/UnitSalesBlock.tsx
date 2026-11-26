"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { BookOpen, PlayCircle, Tag, Trophy } from "lucide-react";
import { StatCard } from "@/components/admin/StatCard";
import { PeriodBar, type PeriodPreset, type PeriodRange } from "@/components/admin/PeriodBar";
import { Button } from "@/components/ui/button";
import { DataTable } from "@/components/ui/data-table";
import { useAdminUnitSales } from "@/lib/hooks/admin-unit-sales";
import { explicitUnitRange, parseUnitPeriod, unitPeriodSearch } from "@/lib/unit-sales-period";
import { useTranslation } from "@/lib/i18n";

const TYPES = ["book", "merchandise", "medal", "course", "exam"] as const;
const UNITS = ["paid_units", "refund_units", "net_units"] as const;

interface UnitSalesBlockProps {
  promoUsed?: number;
  promoActive?: number;
}

export function UnitSalesBlock(props: UnitSalesBlockProps) {
  return <Suspense><UnitSalesOverview {...props} /></Suspense>;
}

function UnitSalesOverview({ promoUsed = 0, promoActive = 0 }: UnitSalesBlockProps) {
  const { t, lang } = useTranslation();
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

  const fmt = (iso: string) => new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso));
  const href = (type: string) => `/admin/store/units/${type}?${unitPeriodSearch(period)}`;
  const rows = TYPES.map((type) => ({ type, row: query.data?.types.find((item) => item.type === type) }));
  const net = (type: string) => rows.find((r) => r.type === type)?.row?.net_units ?? 0;
  const BAR_ORDER = ["exam", "course", "book", "merchandise", "medal"] as const;
  const barMax = Math.max(1, ...BAR_ORDER.map(net)) * 1.15;
  const hi = BAR_ORDER.reduce((best, type) => (net(type) > net(best) ? type : best), BAR_ORDER[0]);

  return <>
    <PeriodBar preset={preset} range={range} allowAllTime resolvedFrom={period.range.from && fmt(period.range.from)} resolvedTo={period.range.to && fmt(period.range.to)}
      onChange={handlePeriodChange} />
    {!query.isLoading && !query.isError && <div className="grid grid-cols-2 gap-3 min-[681px]:grid-cols-3 min-[1201px]:grid-cols-4">
      <StatCard tone="ember" icon={Trophy} label={t("store_kpi_exam_sold")} value={String(net("exam"))} trend={t("store_kpi_exam_sub")} />
      <StatCard tone="teal" icon={PlayCircle} label={t("store_kpi_course_sold")} value={String(net("course"))} trend={t("store_kpi_course_sub")} />
      <StatCard tone="gold" icon={BookOpen} label={t("store_kpi_physical_sold")} value={String(net("book") + net("merchandise") + net("medal"))} trend={t("store_kpi_physical_sub")} />
      <StatCard tone="sky" icon={Tag} label={t("store_kpi_promo_used")} value={String(promoUsed)} trend={t("store_kpi_promo_sub").replace("{count}", String(promoActive))} />
    </div>}
    <section className="md-card-outlined" aria-labelledby="unit-sales-title">
      <div className="mb-3.5">
        <h2 id="unit-sales-title" className="type-h2">{t("admin_unit_title")}</h2>
        <p className="text-[13px] leading-[1.55] text-ink-600">{t("admin_unit_explanation")}</p>
      </div>
      {query.isLoading ? <p role="status">{t("admin_unit_loading")}</p> : query.isError ?
        <div role="alert"><p>{t("admin_unit_error")}</p><Button variant="outline" onClick={() => query.refetch()}>{t("retry")}</Button></div> :
        <>
          <div aria-hidden className="flex h-[170px] items-end gap-3 pt-[34px]">
            {BAR_ORDER.map((type) => (
              <div key={type} className="flex h-full flex-1 flex-col items-center justify-end gap-2">
                <div
                  data-testid="unit-bar"
                  className={`relative w-full max-w-[46px] rounded-[12px_12px_6px_6px] ${type === hi ? "bg-[repeating-linear-gradient(135deg,var(--color-ember-500)_0_6px,var(--color-ember-600)_6px_11px)]" : "bg-brand-600"}`}
                  style={{ height: `${(net(type) / barMax) * 100}%` }}
                >
                  {type === hi && <span className="absolute bottom-[calc(100%+6px)] left-1/2 -translate-x-1/2 rounded-full bg-deep px-[9px] py-[5px] font-[family-name:var(--font-display)] text-[11px] font-bold whitespace-nowrap text-white">{t("store_units_tip").replace("{count}", String(net(type)))}</span>}
                </div>
                <span className={`text-xs ${type === hi ? "font-bold text-ink-900" : "text-ink-600"}`}>{t(`admin_unit_${type}`)}</span>
              </div>
            ))}
          </div>
          <div className="mt-4">
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
            />
          </div>
        </>}
    </section>
  </>;
}
