"use client";

import { Suspense } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Download, FileBarChart, Hourglass, Wallet, XCircle } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { StatCard } from "@/components/admin/StatCard";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import {
  downloadAffiliateCsv,
  useAdminAffiliateReportOrders,
  useAffiliateLiabilityReport,
  useAffiliateReportSummary,
} from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import { formatCommissionRule } from "@/lib/affiliate-rules";
import type { AffiliateReportOrder, AffiliateReportRow } from "@/lib/types";

const COMMISSION_STATUSES = ["pending", "available", "cancelled"] as const;
const CATEGORIES = ["exam", "course", "book", "merchandise", "medal"] as const;
const LEVELS = ["bronze", "silver", "gold"] as const;

const STATUS_CARD = {
  pending: { icon: Hourglass, accent: "secondary" },
  available: { icon: Wallet, accent: "primary" },
  cancelled: { icon: XCircle, accent: "error" },
} as const;

function isoDay(d: Date): string {
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

export default function AdminAffiliateReportsPage() {
  return (
    <Suspense fallback={null}>
      <ReportsContent />
    </Suspense>
  );
}

function ReportsContent() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const tab = search.get("tab") === "orders" ? "orders" : "summary";

  if (!canManage) return <NoAccess />;

  const changeTab = (value: string) => router.replace(value === "orders" ? `${pathname}?tab=orders` : pathname, { scroll: false });

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={FileBarChart} title={t("admin_aff_rep_title")} description={t("admin_aff_rep_desc")} />
      <Liability />
      <Tabs value={tab} onValueChange={changeTab}>
        <TabsList>
          <TabsTrigger value="summary">{t("admin_aff_rep_tab_summary")}</TabsTrigger>
          <TabsTrigger value="orders">{t("admin_aff_rep_tab_orders")}</TabsTrigger>
        </TabsList>
      </Tabs>
      {tab === "summary" ? <SummaryTab /> : <OrdersTab />}
    </div>
  );
}

function Liability() {
  const { t } = useTranslation();
  const liability = useAffiliateLiabilityReport();
  const data = liability.data;
  return (
    <section className="space-y-3" aria-label={t("admin_aff_rep_liability")}>
      <div>
        <h2 className="text-title-large">{t("admin_aff_rep_liability")}</h2>
        <p className="text-sm text-ink-600">{t("admin_aff_rep_liability_desc")}</p>
      </div>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard
          label={t("admin_aff_rep_payout_liability")}
          value={data ? formatRupiah(data.payout_liability) : "—"}
          icon={Wallet}
          accent="error"
        />
        {COMMISSION_STATUSES.map((status) => {
          const { icon, accent } = STATUS_CARD[status];
          return <StatCard key={status} label={affLabel(t, status)} value={data ? formatRupiah(data.by_status?.[status] ?? 0) : "—"} icon={icon} accent={accent} />;
        })}
      </div>
    </section>
  );
}

function SummaryTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  // Without a chosen period the report covers the current month.
  const now = new Date();
  const from = params.from || isoDay(new Date(now.getFullYear(), now.getMonth(), 1));
  const to = params.to || isoDay(now);
  const summary = useAffiliateReportSummary({ from, to, q: params.q, level: params.type, page: params.page, per_page: params.per_page });
  const router = useRouter();
  const pathname = usePathname();

  const commissionTotal = (r: AffiliateReportRow) =>
    Object.entries(r.commissions ?? {}).reduce((sum, [status, n]) => (status === "cancelled" ? sum : sum + n), 0);

  const showOrders = (code: string) => {
    const next = new URLSearchParams({ tab: "orders", q: code });
    if (params.from) next.set("from", params.from);
    if (params.to) next.set("to", params.to);
    router.replace(`${pathname}?${next.toString()}`, { scroll: false });
  };

  const columns: DataTableColumn<AffiliateReportRow>[] = [
    {
      key: "who",
      header: t("admin_aff_col_affiliator"),
      cell: (r) => (
        <div>
          <div className="font-medium text-ink-900">{r.affiliator_name}</div>
          <div className="text-xs text-ink-500">{r.affiliator_code}</div>
        </div>
      ),
    },
    { key: "level", header: t("admin_aff_col_level"), cell: (r) => affLabel(t, r.level) },
    { key: "students", header: t("admin_aff_rep_students"), align: "right", cell: (r) => r.students_new },
    { key: "revenue", header: t("admin_aff_col_revenue"), align: "right", cell: (r) => formatRupiah(r.net_revenue) },
    { key: "commission", header: t("admin_aff_rep_commission"), align: "right", cell: (r) => formatRupiah(commissionTotal(r)) },
    {
      key: "orders",
      header: "",
      align: "right",
      cell: (r) => (
        <Button size="sm" variant="outline" onClick={() => showOrders(r.affiliator_code)}>
          {t("admin_aff_rep_show")}
        </Button>
      ),
    },
  ];

  const totals = summary.data?.totals;

  return (
    <div className="space-y-4">
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("admin_aff_search_queue")}
        filters={[{ key: "type", label: t("admin_aff_col_level"), options: LEVELS.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("admin_aff_rep_period")}
      />
      <AffiliateList
        query={summary}
        columns={columns}
        rowKey={(r) => r.affiliator_id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_rep_empty")}
        filtered={hasFilters}
      />
      {totals && (
        <p className="text-sm text-ink-700">
          <strong>{t("admin_aff_rep_totals")}</strong>: {totals.students_new} · {formatRupiah(totals.net_revenue)} ·{" "}
          {formatRupiah(commissionTotal(totals))}
        </p>
      )}
    </div>
  );
}

function OrdersTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const filter = { q: params.q, status: params.status, type: params.type, from: params.from, to: params.to };
  const query = useAdminAffiliateReportOrders({ ...filter, page: params.page, per_page: params.per_page });

  const exportCsv = async () => {
    try {
      await downloadAffiliateCsv("report-orders", filter);
    } catch {
      toast.error(t("admin_aff_export_failed"));
    }
  };

  const columns: DataTableColumn<AffiliateReportOrder>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.paid_at ?? r.created_at) },
    {
      key: "who",
      header: t("admin_aff_col_affiliator"),
      cell: (r) => (
        <div>
          <div className="font-medium text-ink-900">{r.affiliator_name}</div>
          <div className="text-xs text-ink-500">{r.affiliator_code}</div>
        </div>
      ),
    },
    { key: "order", header: t("admin_aff_col_order"), cell: (r) => <code className="text-xs">{r.order_id.slice(0, 8)}</code> },
    { key: "student", header: t("aff_col_student"), cell: (r) => r.student_name ?? "—" },
    { key: "product", header: t("aff_col_product_name"), cell: (r) => r.product_name ?? "—" },
    { key: "category", header: t("aff_col_product"), cell: (r) => affLabel(t, r.product_type) },
    { key: "base", header: t("aff_col_purchase"), align: "right", cell: (r) => formatRupiah(r.base_amount) },
    { key: "pct", header: t("aff_col_rate"), align: "right", cell: (r) => formatCommissionRule(r.commission_type, r.commission_value, t("aff_per_unit")) },
    { key: "amount", header: t("aff_col_commission"), align: "right", cell: (r) => <span className="font-semibold">{formatRupiah(r.amount)}</span> },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
  ];

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        <Button variant="outline" onClick={exportCsv}>
          <Download className="size-4" aria-hidden />
          {t("admin_aff_export_csv")}
        </Button>
      </div>
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("admin_aff_rep_orders_search")}
        filters={[
          { key: "status", label: t("aff_col_status"), options: COMMISSION_STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "type", label: t("aff_filter_category"), options: CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
        dateLabel={t("admin_aff_rep_paid_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_rep_orders_empty_all")}
        filtered={hasFilters}
      />
    </div>
  );
}
