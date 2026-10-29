"use client";

import { useState } from "react";
import { Download, Search, SlidersHorizontal, X } from "lucide-react";
import { toast } from "sonner";
import { AffiliatorDetailDialog } from "@/components/admin/AffiliatorDialogs";
import type { PeriodRange } from "@/components/admin/PeriodBar";
import { TablePager } from "@/components/admin/TablePager";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { downloadAffiliateReportCsv, useAffiliateReport, useAffiliateTrend, type ReportParams } from "@/lib/hooks/admin-affiliators";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useOffsetPaging } from "@/lib/hooks/use-offset-paging";
import { formatRupiah } from "@/lib/format";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type {
  AffiliateReportName,
  AffiliatorReportRow,
  ProductReportRow,
  PromoReportRow,
  SchoolReportRow,
  TrendReportRow,
} from "@/lib/types";

function ExportButton({ name, params }: { name: AffiliateReportName; params: ReportParams }) {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  return (
    <Button
      variant="outline"
      disabled={busy}
      onClick={async () => {
        setBusy(true);
        try {
          await downloadAffiliateReportCsv(name, params);
        } catch {
          toast.error(t("aff_report_export_failed"));
        } finally {
          setBusy(false);
        }
      }}
    >
      <Download className="size-4" aria-hidden />
      {t("aff_report_export")}
    </Button>
  );
}

function ReportError({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="md-card-outlined py-12 text-center text-ink-500">
      <p className="mb-4">{t("dash_load_failed")}</p>
      <button type="button" className="md-btn-tonal" onClick={onRetry}>
        {t("admin_home_reload")}
      </button>
    </div>
  );
}

interface PagedProps<T> {
  name: Exclude<AffiliateReportName, "trend">;
  period: PeriodRange;
  columns: DataTableColumn<T>[];
  rowKey: (row: T) => string;
  empty: string;
  /** Search text and the extra flag, when the report has them. */
  q?: string;
  inactive?: boolean;
  toolbar?: React.ReactNode;
  /** Removable chips for the active filters, shown under the toolbar. */
  chips?: React.ReactNode;
  /** What this report counts and over which time, shown above the table. */
  intro: string;
}

function PagedReport<T>({ name, period, columns, rowKey, empty, q, inactive, toolbar, chips, intro }: PagedProps<T>) {
  const { t } = useTranslation();
  const [limit, setLimit] = useState(20);
  const paging = useOffsetPaging([name, period.from, period.to, q, inactive, limit].join("|"), limit);
  const params: ReportParams = { ...period, q: q || undefined, inactive: inactive || undefined, limit, offset: paging.offset };
  const report = useAffiliateReport<T>(name, params);
  const rows = report.data?.data ?? [];
  const total = report.data?.total ?? 0;

  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-600">{intro}</p>
      <div className="flex flex-wrap items-center gap-3">
        {toolbar}
        <div className="ml-auto">
          <ExportButton name={name} params={{ ...period, q: q || undefined, inactive: inactive || undefined }} />
        </div>
      </div>
      {chips}
      {report.isError ? (
        <ReportError onRetry={() => report.refetch()} />
      ) : (
        <DataTable
          columns={columns}
          rows={report.isLoading ? [] : rows}
          rowKey={rowKey}
          empty={report.isLoading ? t("aff_loading") : empty}
          footer={
            <TablePager
              total={total}
              offset={paging.offset}
              count={rows.length}
              limit={limit}
              onLimitChange={setLimit}
              canPrev={paging.canPrev}
              canNext={paging.offset + rows.length < total}
              onPrev={paging.prev}
              onNext={paging.next}
            />
          }
        />
      )}
    </div>
  );
}

function SearchBox({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) {
  const { t } = useTranslation();
  return (
    <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:max-w-md sm:flex-1">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
      <Input type="search" value={value} onChange={(e) => onChange(e.target.value)} placeholder={label} aria-label={label} className="pr-10 pl-9" />
      {value && (
        <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => onChange("")}>
          <X className="size-4" aria-hidden />
        </Button>
      )}
    </div>
  );
}

export function AffiliatorsReport({ period }: { period: PeriodRange }) {
  const { t, lang } = useTranslation();
  const [search, setSearch] = useState("");
  const [inactive, setInactive] = useState(false);
  const [detail, setDetail] = useState<string | null>(null);
  const q = useDebouncedValue(search).trim();
  const date = (iso: string | null) =>
    iso ? new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso)) : "-";

  const columns: DataTableColumn<AffiliatorReportRow>[] = [
    {
      key: "name",
      header: t("aff_register_name"),
      cell: (r) => (
        <div>
          <div className="font-medium">{r.name}</div>
          <div className="text-xs text-ink-600">{r.email}</div>
        </div>
      ),
    },
    {
      key: "status",
      header: t("th_status"),
      cell: (r) => (
        <Badge variant="outline" className={r.status === "active" ? "border-transparent bg-success-bg text-success" : undefined}>
          {t(r.status === "active" ? "status_label_active" : "status_label_inactive")}
        </Badge>
      ),
    },
    { key: "students", header: t("aff_th_students"), align: "right", cell: (r) => r.student_count },
    { key: "paid_orders", header: t("aff_stat_paid_orders"), align: "right", cell: (r) => r.paid_orders },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.paid_units },
    { key: "amount", header: t("aff_th_amount"), align: "right", cell: (r) => formatRupiah(r.paid_amount) },
    { key: "discount", header: t("aff_stat_discount"), align: "right", cell: (r) => formatRupiah(r.discount_total) },
    { key: "pending", header: t("aff_stat_pending"), align: "right", cell: (r) => `${r.pending_orders} · ${formatRupiah(r.pending_amount)}` },
    { key: "expired", header: t("aff_report_expired"), align: "right", cell: (r) => r.expired_orders },
    { key: "last", header: t("aff_report_last_order"), cell: (r) => date(r.last_order_at) },
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (r) => (
        <Button size="sm" variant="outline" onClick={() => setDetail(r.affiliator_id)}>
          {t("aff_admin_detail")}
        </Button>
      ),
    },
  ];

  return (
    <>
      <PagedReport<AffiliatorReportRow>
        name="affiliators"
        period={period}
        columns={columns}
        rowKey={(r) => r.affiliator_id}
        empty={t("aff_admin_empty")}
        intro={t("aff_report_desc_affiliators")}
        q={q}
        inactive={inactive}
        toolbar={
          <>
            <SearchBox value={search} onChange={setSearch} label={t("aff_admin_search")} />
            <Popover>
              <PopoverTrigger asChild>
                <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${inactive ? " (1)" : ""}`}>
                  <SlidersHorizontal className="size-4" aria-hidden />
                  {t("aff_filters")}
                  {inactive && <span className="tabular-nums">1</span>}
                </Button>
              </PopoverTrigger>
              <PopoverContent align="start" className="admin-shell w-[min(24rem,calc(100vw-2rem))]" aria-label={t("aff_report_filter_title")}>
                <div className="space-y-3">
                  <h2 className="font-semibold">{t("aff_report_filter_title")}</h2>
                  <label
                    htmlFor="aff-report-inactive"
                    className="flex cursor-pointer items-start gap-3 rounded-[12px] border border-line p-3 transition-colors hover:bg-surface-2 has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-brand-50"
                  >
                    <Checkbox id="aff-report-inactive" checked={inactive} onCheckedChange={(v) => setInactive(v === true)} className="mt-0.5" />
                    <span className="grid gap-0.5">
                      <span className="text-sm font-medium">{t("aff_report_inactive_only")}</span>
                      <span className="text-xs text-ink-600">{t("aff_report_inactive_hint")}</span>
                    </span>
                  </label>
                  <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                    <Button variant="ghost" onClick={() => setInactive(false)}>
                      {t("aff_reset_filters")}
                    </Button>
                    <PopoverClose asChild>
                      <Button>{t("aff_report_show")}</Button>
                    </PopoverClose>
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </>
        }
        chips={
          inactive && (
            <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_active_filters")}>
              <span className="md-chip md-chip-primary gap-1">
                {t("aff_report_inactive_only")}
                <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${t("aff_report_inactive_only")}`} onClick={() => setInactive(false)}>
                  <X className="size-4" aria-hidden />
                </Button>
              </span>
            </div>
          )
        }
      />
      <AffiliatorDetailDialog id={detail} onClose={() => setDetail(null)} />
    </>
  );
}

export function SchoolsReport({ period }: { period: PeriodRange }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search).trim();
  const columns: DataTableColumn<SchoolReportRow>[] = [
    { key: "school", header: t("aff_th_school"), cell: (r) => <span className="font-medium">{r.school_name}</span> },
    { key: "affiliators", header: t("aff_report_affiliators"), align: "right", cell: (r) => r.affiliator_count },
    { key: "students", header: t("aff_th_students"), align: "right", cell: (r) => r.student_count },
    { key: "covered", header: t("aff_report_covered"), align: "right", cell: (r) => r.covered_students },
    { key: "unpurchased", header: t("aff_report_unpurchased"), align: "right", cell: (r) => r.unpurchased_students },
    { key: "orders", header: t("aff_stat_paid_orders"), align: "right", cell: (r) => r.paid_orders },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.paid_units },
    { key: "amount", header: t("aff_th_amount"), align: "right", cell: (r) => formatRupiah(r.paid_amount) },
    { key: "discount", header: t("aff_stat_discount"), align: "right", cell: (r) => formatRupiah(r.discount_total) },
  ];
  return (
    <PagedReport<SchoolReportRow>
      name="schools"
      period={period}
      columns={columns}
      rowKey={(r) => r.school_id}
      empty={t("aff_empty_schools")}
      intro={t("aff_report_desc_schools")}
      q={q}
      toolbar={<SearchBox value={search} onChange={setSearch} label={t("aff_report_search_school")} />}
    />
  );
}

export function ProductsReport({ period }: { period: PeriodRange }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search).trim();
  const columns: DataTableColumn<ProductReportRow>[] = [
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name },
    { key: "product", header: t("aff_th_product"), cell: (r) => <span className="font-medium">{r.product_name}</span> },
    { key: "type", header: t("aff_report_type"), cell: (r) => t(r.product_type === "exam" ? "product_type_exam" : "product_type_course") },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.units },
    { key: "orders", header: t("aff_th_orders"), align: "right", cell: (r) => r.orders },
  ];
  return (
    <PagedReport<ProductReportRow>
      name="products"
      period={period}
      columns={columns}
      rowKey={(r) => `${r.school_id}:${r.product_id}`}
      empty={t("aff_empty_sales")}
      intro={t("aff_report_desc_products")}
      q={q}
      toolbar={<SearchBox value={search} onChange={setSearch} label={t("aff_report_search_product")} />}
    />
  );
}

export function PromosReport({ period }: { period: PeriodRange }) {
  const { t, lang } = useTranslation();
  const [search, setSearch] = useState("");
  const q = useDebouncedValue(search).trim();
  const date = (iso: string | null) =>
    iso ? new Intl.DateTimeFormat(lang === "en" ? "en-US" : "id-ID", { day: "numeric", month: "short", year: "numeric", timeZone: "Asia/Jakarta" }).format(new Date(iso)) : null;
  const discount = (p: PromoReportRow) =>
    p.discount_percent != null ? `${p.discount_percent}%` : p.discount_amount != null ? formatRupiah(p.discount_amount) : "-";
  const validity = (p: PromoReportRow) => {
    const from = date(p.starts_at);
    const to = date(p.expires_at);
    return !from && !to ? t("aff_promo_no_expiry") : `${from ?? "…"} – ${to ?? "…"}`;
  };

  const columns: DataTableColumn<PromoReportRow>[] = [
    { key: "code", header: t("th_code"), cell: (p) => <span className="font-medium">{p.code}</span> },
    { key: "scope", header: t("aff_report_scope"), cell: (p) => t(p.affiliator_only ? "promo_affiliator_only" : "th_public") },
    { key: "discount", header: t("th_discount"), cell: discount },
    { key: "validity", header: t("aff_th_validity"), cell: validity },
    {
      key: "state",
      header: t("th_status"),
      cell: (p) => (
        <Badge variant="outline" className={p.active_now ? "border-transparent bg-success-bg text-success" : undefined}>
          {t(p.active_now ? "aff_report_promo_active" : "aff_report_promo_inactive")}
        </Badge>
      ),
    },
    { key: "usage", header: t("th_usage"), align: "right", cell: (p) => `${p.used_count} / ${p.max_uses ?? "∞"}` },
    { key: "orders", header: t("aff_report_orders_in_period"), align: "right", cell: (p) => p.orders },
    { key: "total", header: t("aff_report_discount_in_period"), align: "right", cell: (p) => formatRupiah(p.discount_total) },
  ];
  return (
    <PagedReport<PromoReportRow>
      name="promos"
      period={period}
      columns={columns}
      rowKey={(p) => p.code}
      empty={t("aff_promos_empty")}
      intro={t("aff_report_desc_promos")}
      q={q}
      toolbar={<SearchBox value={search} onChange={setSearch} label={t("aff_report_search_promo")} />}
    />
  );
}

export function TrendReport({ period }: { period: PeriodRange }) {
  const { t } = useTranslation();
  const trend = useAffiliateTrend<TrendReportRow>(period);
  const rows = trend.data ?? [];
  const columns: DataTableColumn<TrendReportRow>[] = [
    { key: "month", header: t("aff_th_month"), cell: (r) => r.month },
    { key: "orders", header: t("aff_th_orders"), align: "right", cell: (r) => r.orders },
    { key: "units", header: t("aff_th_units"), align: "right", cell: (r) => r.units },
    { key: "amount", header: t("aff_th_amount"), align: "right", cell: (r) => formatRupiah(r.amount) },
    { key: "discount", header: t("aff_stat_discount"), align: "right", cell: (r) => formatRupiah(r.discount) },
    { key: "affiliators", header: t("aff_report_active_affiliators"), align: "right", cell: (r) => r.active_affiliators },
  ];
  return (
    <div className="space-y-4">
      <p className="text-sm text-ink-600">{t("aff_report_desc_trend")}</p>
      <div className="flex justify-end">
        <ExportButton name="trend" params={period} />
      </div>
      {trend.isError ? (
        <ReportError onRetry={() => trend.refetch()} />
      ) : (
        <DataTable columns={columns} rows={trend.isLoading ? [] : rows} rowKey={(r) => r.month} empty={trend.isLoading ? t("aff_loading") : t("aff_empty_sales")} />
      )}
    </div>
  );
}

export const AFFILIATE_REPORT_TABS: { id: AffiliateReportName; labelKey: I18nKey }[] = [
  { id: "affiliators", labelKey: "aff_report_tab_affiliators" },
  { id: "schools", labelKey: "aff_report_tab_schools" },
  { id: "products", labelKey: "aff_report_tab_products" },
  { id: "promos", labelKey: "aff_report_tab_promos" },
  { id: "trend", labelKey: "aff_report_tab_trend" },
];
