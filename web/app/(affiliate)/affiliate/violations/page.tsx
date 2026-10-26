"use client";

import { Suspense, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Gavel } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { AppealBadge, StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useAffiliateAdjustments, useAffiliateViolations, useAppealViolation } from "@/lib/hooks/affiliate";
import { useListParams } from "@/lib/hooks/use-list-params";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdjustment, AffiliateViolation } from "@/lib/types";

const SANCTIONS = ["warning", "commission_cancel", "balance_freeze", "level_change", "suspend", "deactivate"] as const;
const STATES = ["open", "resolved"] as const;
const CATEGORIES = ["correction", "offline", "bonus", "violation", "other"] as const;

function AppealCell({ violation }: { violation: AffiliateViolation }) {
  const { t } = useTranslation();
  const appeal = useAppealViolation();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");

  if (violation.appeal_status !== "none") return <AppealBadge status={violation.appeal_status} />;
  if (violation.status !== "open") return <span className="text-ink-500">—</span>;
  if (!open) {
    return (
      <Button variant="outline" size="sm" onClick={() => setOpen(true)}>
        {t("aff_appeal_button")}
      </Button>
    );
  }

  const submit = () => {
    if (!text.trim()) {
      toast.error(t("aff_appeal_required"));
      return;
    }
    appeal.mutate(
      { id: violation.id, text: text.trim() },
      {
        onSuccess: () => toast.success(t("aff_appeal_sent")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_error")),
      },
    );
  };

  return (
    <div className="space-y-2">
      <textarea
        aria-label={t("aff_appeal_placeholder")}
        placeholder={t("aff_appeal_placeholder")}
        value={text}
        onChange={(e) => setText(e.target.value)}
        rows={3}
        className="w-full rounded-md border border-line bg-surface p-2 text-sm"
      />
      <div className="flex gap-2">
        <Button size="sm" onClick={submit} disabled={appeal.isPending}>
          {t("aff_appeal_submit")}
        </Button>
        <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>
          {t("aff_appeal_cancel")}
        </Button>
      </div>
    </div>
  );
}

export default function AffiliateViolationsPage() {
  return (
    <Suspense fallback={null}>
      <ViolationsContent />
    </Suspense>
  );
}

function ViolationsContent() {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const tab = search.get("tab") === "adjustments" ? "adjustments" : "sanctions";
  const changeTab = (value: string) => router.replace(value === "adjustments" ? `${pathname}?tab=adjustments` : pathname, { scroll: false });

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={Gavel} title={t("aff_viol_title")} description={t("aff_viol_desc")} />
      <Tabs value={tab} onValueChange={changeTab}>
        <TabsList>
          <TabsTrigger value="sanctions">{t("aff_viol_tab_sanctions")}</TabsTrigger>
          <TabsTrigger value="adjustments">{t("aff_viol_tab_adjustments")}</TabsTrigger>
        </TabsList>
      </Tabs>
      <p className="text-sm text-ink-600">{tab === "sanctions" ? t("aff_viol_help_sanctions") : t("aff_viol_help_adjustments")}</p>
      {tab === "sanctions" ? <SanctionsTab /> : <AdjustmentsTab />}
    </div>
  );
}

function SanctionsTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliateViolations({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    type: params.type,
    status: params.status,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliateViolation>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.decided_at) },
    { key: "category", header: t("aff_col_category"), cell: (r) => affLabel(t, r.category) },
    { key: "description", header: t("aff_col_description"), cell: (r) => r.description },
    { key: "sanction", header: t("aff_col_sanction"), cell: (r) => affLabel(t, r.sanction) },
    { key: "state", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "until", header: t("aff_col_until"), cell: (r) => formatAffDate(r.suspension_until) },
    { key: "appeal", header: t("aff_col_appeal"), cell: (r) => <AppealCell violation={r} /> },
  ];

  return (
    <div className="space-y-4">
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_viol_search_sanctions")}
        filters={[
          { key: "type", label: t("aff_viol_filter_sanction"), options: SANCTIONS.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "status", label: t("aff_viol_filter_state"), options: STATES.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
        dateLabel={t("aff_viol_filter_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_viol_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}

function AdjustmentsTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliateAdjustments({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    type: params.type,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliateAdjustment>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "category", header: t("aff_col_category"), cell: (r) => affLabel(t, r.category) },
    {
      key: "amount",
      header: t("aff_col_amount"),
      align: "right",
      cell: (r) => <span className={r.amount < 0 ? "font-semibold text-danger" : "font-semibold text-ink-900"}>{formatRupiah(r.amount)}</span>,
    },
    { key: "reason", header: t("aff_col_reason_text"), cell: (r) => r.reason },
    { key: "applied", header: t("aff_col_applied"), cell: (r) => (r.applied ? t("aff_adj_applied_yes") : t("aff_adj_applied_no")) },
  ];

  return (
    <div className="space-y-4">
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_viol_search_adjustments")}
        filters={[{ key: "type", label: t("aff_viol_filter_category"), options: CATEGORIES.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("aff_viol_filter_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_adj_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
