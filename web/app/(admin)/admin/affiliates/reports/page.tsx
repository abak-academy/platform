"use client";

import { Suspense, useState } from "react";
import { ChevronDown, ChevronRight, FileBarChart } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AdminCommissionsPanel } from "@/components/affiliate/AdminCommissionsPanel";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { affLabel } from "@/lib/affiliate-labels";
import { useAffiliateLiabilityReport, useAffiliateReportSummary } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateReportRow } from "@/lib/types";

const COMMISSION_STATUSES = ["pending", "available", "paid", "held", "cancelled"] as const;
const LEVELS = ["bronze", "silver", "gold"] as const;

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
  const { params, update, reset, hasFilters } = useListParams();
  const [expanded, setExpanded] = useState<string | null>(null);
  // The report's own period is not a list filter, so it stays out of the URL
  // params the toolbar owns (from/to there would hide the orders' date range).
  const [from, setFrom] = useState(() => {
    const now = new Date();
    return isoDay(new Date(now.getFullYear(), now.getMonth(), 1));
  });
  const [to, setTo] = useState(() => isoDay(new Date()));
  const rangeValid = from !== "" && to !== "" && from <= to;
  const summary = useAffiliateReportSummary({
    from: rangeValid ? from : "",
    to: rangeValid ? to : "",
    q: params.q,
    level: params.type,
    page: params.page,
    per_page: params.per_page,
  });
  const liability = useAffiliateLiabilityReport();

  if (!canManage) return <NoAccess />;

  const commissionTotal = (r: AffiliateReportRow) =>
    Object.entries(r.commissions ?? {}).reduce((sum, [status, n]) => (status === "cancelled" ? sum : sum + n), 0);

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
    { key: "clicks", header: t("admin_aff_rep_clicks"), align: "right", cell: (r) => r.clicks },
    { key: "revenue", header: t("admin_aff_col_revenue"), align: "right", cell: (r) => formatRupiah(r.net_revenue) },
    { key: "commission", header: t("admin_aff_rep_commission"), align: "right", cell: (r) => formatRupiah(commissionTotal(r)) },
    {
      key: "orders",
      header: t("admin_aff_rep_orders"),
      cell: (r) => {
        const open = expanded === r.affiliator_id;
        return (
          <Button
            size="sm"
            variant="outline"
            aria-expanded={open}
            onClick={() => setExpanded(open ? null : r.affiliator_id)}
          >
            {open ? <ChevronDown className="size-4" aria-hidden /> : <ChevronRight className="size-4" aria-hidden />}
            {open ? t("admin_aff_rep_hide") : t("admin_aff_rep_show")}
          </Button>
        );
      },
    },
  ];

  const totals = summary.data?.totals;

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={FileBarChart} title={t("admin_aff_rep_title")} description={t("admin_aff_rep_desc")} />

      <section className="md-card-outlined space-y-2 p-5">
        <h2 className="font-semibold text-ink-900">{t("admin_aff_rep_liability")}</h2>
        {liability.data && (
          <>
            <p className="text-sm text-ink-600">
              {t("admin_aff_rep_payout_liability")}:{" "}
              <strong className="text-ink-900">{formatRupiah(liability.data.payout_liability)}</strong>
            </p>
            <ul className="flex flex-wrap gap-x-6 gap-y-1 text-sm text-ink-600">
              {COMMISSION_STATUSES.map((s) => (
                <li key={s}>
                  {affLabel(t, s)}: {formatRupiah(liability.data?.by_status?.[s] ?? 0)}
                </li>
              ))}
            </ul>
          </>
        )}
      </section>

      <div className="flex flex-wrap items-end gap-4">
        <div>
          <Label htmlFor="rep-from">{t("admin_aff_rep_from")}</Label>
          <Input id="rep-from" type="date" value={from} onChange={(e) => setFrom(e.target.value)} />
        </div>
        <div>
          <Label htmlFor="rep-to">{t("admin_aff_rep_to")}</Label>
          <Input id="rep-to" type="date" value={to} onChange={(e) => setTo(e.target.value)} />
        </div>
      </div>
      {!rangeValid && from !== "" && to !== "" && (
        <p role="alert" className="text-sm text-danger">
          {t("admin_aff_rep_range_invalid")}
        </p>
      )}

      {rangeValid && (
        <>
          <ListToolbar
            params={params}
            onChange={update}
            onReset={reset}
            searchPlaceholder={t("admin_aff_search_queue")}
            filters={[{ key: "type", label: t("admin_aff_col_level"), options: LEVELS.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
          />
          <AffiliateList
            query={summary}
            columns={columns}
            rowKey={(r) => r.affiliator_id}
            page={params.page}
            onPageChange={(page) => update({ page })}
            empty={t("admin_aff_rep_empty")}
            filtered={hasFilters}
            expandedKey={expanded}
            renderExpanded={(r) => <AdminCommissionsPanel affiliatorId={r.affiliator_id} from={from} to={to} />}
          />
          {totals && (
            <p className="text-sm text-ink-700">
              <strong>{t("admin_aff_rep_totals")}</strong>: {totals.students_new} · {totals.clicks} ·{" "}
              {formatRupiah(totals.net_revenue)} · {formatRupiah(commissionTotal(totals))}
            </p>
          )}
        </>
      )}
    </div>
  );
}
