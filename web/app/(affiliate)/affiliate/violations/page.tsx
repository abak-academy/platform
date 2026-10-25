"use client";

import { useState } from "react";
import { toast } from "sonner";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { AppealBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import {
  useAffiliateAdjustments,
  useAffiliateViolations,
  useAppealViolation,
} from "@/lib/hooks/affiliate";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdjustment, AffiliateViolation } from "@/lib/types";

function AppealCell({ violation }: { violation: AffiliateViolation }) {
  const { t } = useTranslation();
  const appeal = useAppealViolation();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");

  if (violation.appeal_status !== "none") return <AppealBadge status={violation.appeal_status} />;
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
  const { t } = useTranslation();
  const [violationPage, setViolationPage] = useState(1);
  const [adjustmentPage, setAdjustmentPage] = useState(1);
  const violations = useAffiliateViolations({ page: violationPage });
  const adjustments = useAffiliateAdjustments({ page: adjustmentPage });

  const violationColumns: DataTableColumn<AffiliateViolation>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.decided_at) },
    { key: "category", header: t("aff_col_category"), cell: (r) => r.category },
    { key: "description", header: t("aff_col_description"), cell: (r) => r.description },
    { key: "sanction", header: t("aff_col_sanction"), cell: (r) => affLabel(t, r.sanction) },
    { key: "until", header: t("aff_col_until"), cell: (r) => formatAffDate(r.suspension_until) },
    { key: "appeal", header: t("aff_col_appeal"), cell: (r) => <AppealCell violation={r} /> },
  ];

  const adjustmentColumns: DataTableColumn<AffiliateAdjustment>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "category", header: t("aff_col_category"), cell: (r) => affLabel(t, r.category) },
    {
      key: "amount",
      header: t("aff_col_amount"),
      align: "right",
      cell: (r) => (
        <span className={r.amount < 0 ? "text-danger" : "text-ink-900"}>{formatRupiah(r.amount)}</span>
      ),
    },
    { key: "reason", header: t("aff_col_reason_text"), cell: (r) => r.reason },
    {
      key: "applied",
      header: t("aff_col_applied"),
      cell: (r) => (r.applied ? t("aff_adj_applied_yes") : t("aff_adj_applied_no")),
    },
  ];

  return (
    <div className="space-y-8">
      <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_viol_title")}</h1>

      <section className="space-y-3">
        <h2 className="font-semibold text-ink-900">{t("aff_viol_section")}</h2>
        <AffiliateList
          query={violations}
          columns={violationColumns}
          rowKey={(r) => r.id}
          page={violationPage}
          onPageChange={setViolationPage}
          empty={t("aff_viol_empty")}
        />
      </section>

      <section className="space-y-3">
        <h2 className="font-semibold text-ink-900">{t("aff_adj_section")}</h2>
        <AffiliateList
          query={adjustments}
          columns={adjustmentColumns}
          rowKey={(r) => r.id}
          page={adjustmentPage}
          onPageChange={setAdjustmentPage}
          empty={t("aff_adj_empty")}
        />
      </section>
    </div>
  );
}
