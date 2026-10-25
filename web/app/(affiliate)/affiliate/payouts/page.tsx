"use client";

import { useEffect, useState } from "react";
import { toast } from "sonner";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  useAffiliateMe,
  useAffiliatePayouts,
  useRequestAffiliatePayout,
  useUpdateAffiliateBank,
} from "@/lib/hooks/affiliate";
import { ApiError, fileUrl } from "@/lib/api";
import { formatAffDate } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliatePayout } from "@/lib/types";

const PAYOUT_ERRORS: Record<string, I18nKey> = {
  affiliate_frozen: "aff_payout_err_frozen",
  affiliate_bank_missing: "aff_payout_err_bank",
  affiliate_payout_below_min: "aff_payout_err_min",
  affiliate_payout_in_flight: "aff_payout_err_in_flight",
};

export default function AffiliatePayoutsPage() {
  const { t } = useTranslation();
  const me = useAffiliateMe();
  const [page, setPage] = useState(1);
  const payouts = useAffiliatePayouts({ page });
  const saveBank = useUpdateAffiliateBank();
  const requestPayout = useRequestAffiliatePayout();

  const [bankName, setBankName] = useState("");
  const [bankNumber, setBankNumber] = useState("");
  const [bankHolder, setBankHolder] = useState("");
  const [amount, setAmount] = useState("");

  useEffect(() => {
    if (!me.data) return;
    setBankName(me.data.bank_name ?? "");
    setBankNumber(me.data.bank_account_number ?? "");
    setBankHolder(me.data.bank_account_holder ?? "");
  }, [me.data]);

  const onSaveBank = (e: React.FormEvent) => {
    e.preventDefault();
    if (!bankName.trim() || !bankNumber.trim() || !bankHolder.trim()) {
      toast.error(t("aff_bank_required"));
      return;
    }
    saveBank.mutate(
      {
        bank_name: bankName.trim(),
        bank_account_number: bankNumber.trim(),
        bank_account_holder: bankHolder.trim(),
      },
      {
        onSuccess: () => toast.success(t("aff_bank_saved")),
        onError: (err) => toast.error(err instanceof ApiError ? err.message : t("aff_error")),
      },
    );
  };

  const onRequest = (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = amount.trim() === "" ? undefined : Number(amount);
    if (parsed !== undefined && (!Number.isFinite(parsed) || parsed <= 0)) return;
    requestPayout.mutate(parsed === undefined ? {} : { amount: parsed }, {
      onSuccess: () => {
        setAmount("");
        toast.success(t("aff_payout_submitted"));
      },
      onError: (err) => {
        const key = err instanceof ApiError ? PAYOUT_ERRORS[err.code] : undefined;
        toast.error(key ? t(key) : err instanceof ApiError ? err.message : t("aff_error"));
      },
    });
  };

  const columns: DataTableColumn<AffiliatePayout>[] = [
    { key: "requested", header: t("aff_col_requested"), cell: (r) => formatAffDate(r.requested_at) },
    { key: "amount", header: t("aff_col_amount"), align: "right", cell: (r) => formatRupiah(r.amount) },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "processed", header: t("aff_col_processed"), cell: (r) => formatAffDate(r.processed_at) },
    { key: "reason", header: t("aff_col_reason"), cell: (r) => r.reject_reason ?? "—" },
    {
      key: "proof",
      header: t("aff_col_proof"),
      cell: (r) =>
        r.proof_url ? (
          <a href={fileUrl(r.proof_url)} target="_blank" rel="noreferrer" className="text-success underline">
            {t("aff_proof_open")}
          </a>
        ) : (
          "—"
        ),
    },
  ];

  const summary = payouts.data;

  return (
    <div className="space-y-8">
      <h1 className="font-serif text-2xl font-bold text-ink-900">{t("aff_payout_title")}</h1>

      <section className="md-card-outlined space-y-3 p-5">
        <h2 className="font-semibold text-ink-900">{t("aff_bank_title")}</h2>
        <form onSubmit={onSaveBank} className="grid gap-3 sm:grid-cols-3">
          <div>
            <Label htmlFor="bank_name">{t("aff_bank_name")}</Label>
            <Input id="bank_name" value={bankName} onChange={(e) => setBankName(e.target.value)} />
          </div>
          <div>
            <Label htmlFor="bank_number">{t("aff_bank_number")}</Label>
            <Input id="bank_number" value={bankNumber} onChange={(e) => setBankNumber(e.target.value)} inputMode="numeric" />
          </div>
          <div>
            <Label htmlFor="bank_holder">{t("aff_bank_holder")}</Label>
            <Input id="bank_holder" value={bankHolder} onChange={(e) => setBankHolder(e.target.value)} />
          </div>
          <div className="sm:col-span-3">
            <Button type="submit" disabled={saveBank.isPending}>
              {t("aff_bank_save")}
            </Button>
          </div>
        </form>
      </section>

      <section className="md-card-outlined space-y-3 p-5">
        <h2 className="font-semibold text-ink-900">{t("aff_payout_request_title")}</h2>
        {summary && (
          <div className="space-y-1 text-sm text-ink-600">
            <p>
              {t("aff_payout_info")
                .replace("{saldo}", formatRupiah(summary.saldo))
                .replace("{min}", formatRupiah(summary.min_payout))}
            </p>
            {summary.payout_day_of_month !== undefined && (
              <p>{t("aff_payout_day").replace("{day}", String(summary.payout_day_of_month))}</p>
            )}
          </div>
        )}
        <form onSubmit={onRequest} className="flex flex-wrap items-end gap-3">
          <div className="min-w-[220px] flex-1">
            <Label htmlFor="payout_amount">{t("aff_payout_amount")}</Label>
            <Input
              id="payout_amount"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              inputMode="numeric"
            />
          </div>
          <Button type="submit" disabled={requestPayout.isPending}>
            {t("aff_payout_submit")}
          </Button>
        </form>
      </section>

      <section className="space-y-3">
        <h2 className="font-semibold text-ink-900">{t("aff_payout_history")}</h2>
        <AffiliateList
          query={payouts}
          columns={columns}
          rowKey={(r) => r.id}
          page={page}
          onPageChange={setPage}
          empty={t("aff_payout_empty")}
        />
      </section>
    </div>
  );
}
