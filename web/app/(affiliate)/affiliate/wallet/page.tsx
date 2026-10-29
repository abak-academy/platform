"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Landmark, Wallet } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ApiError } from "@/lib/api";
import { useAffiliateLedger, useAffiliateMe, useAffiliatePayouts, useRequestAffiliatePayout } from "@/lib/hooks/affiliate";
import { useListParams } from "@/lib/hooks/use-list-params";
import { affLabel, formatAffDate, ledgerDescription } from "@/lib/affiliate-labels";
import { formatRupiah } from "@/lib/format";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import type { AffiliateLedgerEntry, AffiliatePayout } from "@/lib/types";

const LEDGER_KINDS = ["commission", "refund_deduction", "adjustment", "payout", "payout_reversal"] as const;
const PAYOUT_STATUSES = ["requested", "processing", "paid", "rejected"] as const;

const PAYOUT_ERRORS: Record<string, I18nKey> = {
  affiliate_bank_missing: "aff_payout_err_bank",
  affiliate_payout_below_min: "aff_payout_err_min",
  affiliate_payout_in_flight: "aff_payout_err_in_flight",
};

export default function AffiliateWalletPage() {
  return (
    <Suspense fallback={null}>
      <WalletContent />
    </Suspense>
  );
}

function maskAccount(number: string): string {
  return number.length > 4 ? `•••• ${number.slice(-4)}` : number;
}

function WalletContent() {
  const { t } = useTranslation();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams();
  const tab = search.get("tab") === "payouts" ? "payouts" : "history";

  const me = useAffiliateMe();
  const summary = useAffiliatePayouts({ per_page: 1 });
  const profile = me.data;
  const saldo = summary.data?.saldo ?? 0;
  const minPayout = summary.data?.min_payout ?? 0;
  const payoutDay = summary.data?.payout_day_of_month;

  const bankReady = Boolean(profile?.bank_name && profile.bank_account_number && profile.bank_account_holder);

  let blocked: React.ReactNode = null;
  if (profile && profile.status !== "active") blocked = t("aff_wallet_block_inactive");
  else if (profile && !bankReady) {
    blocked = (
      <>
        {t("aff_wallet_block_bank")}{" "}
        <Link href="/affiliate/account" className="font-semibold text-success underline">
          {t("aff_wallet_block_bank_link")}
        </Link>
      </>
    );
  } else if (summary.data && saldo < minPayout) blocked = t("aff_wallet_block_min").replace("{min}", formatRupiah(minPayout));

  const changeTab = (value: string) => router.replace(value === "payouts" ? `${pathname}?tab=payouts` : pathname, { scroll: false });

  return (
    <div className="space-y-6">
      <AdminPageHeader icon={Wallet} title={t("aff_wallet_title")} description={t("aff_wallet_desc")} />

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard
          label={t("aff_wallet_balance")}
          value={summary.data ? formatRupiah(saldo) : "—"}
          icon={Wallet}
          trend={t("aff_wallet_balance_hint")}
          accent="primary"
        />
        <StatCard
          label={t("aff_wallet_min")}
          value={summary.data ? formatRupiah(minPayout) : "—"}
          trend={payoutDay ? t("aff_wallet_schedule").replace("{day}", String(payoutDay)) : undefined}
          accent="secondary"
        />
        <StatCard
          label={t("aff_wallet_bank")}
          value={
            bankReady && profile
              ? `${profile.bank_name} ${maskAccount(profile.bank_account_number ?? "")}`
              : profile
                ? t("aff_wallet_bank_missing")
                : "—"
          }
          icon={Landmark}
          trend={bankReady ? (profile?.bank_account_holder ?? undefined) : undefined}
          accent={bankReady || !profile ? "tertiary" : "error"}
        />
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <RequestPayout disabled={blocked !== null || !summary.data} saldo={saldo} bank={profile?.bank_name ?? ""} />
        {blocked && (
          <p role="note" className="text-sm text-ink-600">
            <span className="sr-only">{t("aff_wallet_why_disabled")} </span>
            {blocked}
          </p>
        )}
      </div>

      <Tabs value={tab} onValueChange={changeTab}>
        <TabsList>
          <TabsTrigger value="history">{t("aff_wallet_tab_history")}</TabsTrigger>
          <TabsTrigger value="payouts">{t("aff_wallet_tab_payouts")}</TabsTrigger>
        </TabsList>
      </Tabs>

      {tab === "history" ? <HistoryTab /> : <PayoutsTab />}
    </div>
  );
}

function RequestPayout({ disabled, saldo, bank }: { disabled: boolean; saldo: number; bank: string }) {
  const { t } = useTranslation();
  const request = useRequestAffiliatePayout();
  const [open, setOpen] = useState(false);
  const [amount, setAmount] = useState("");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const parsed = amount.trim() === "" ? undefined : Number(amount);
    if (parsed !== undefined && (!Number.isFinite(parsed) || parsed <= 0)) {
      toast.error(t("aff_wallet_amount_invalid"));
      return;
    }
    request.mutate(parsed === undefined ? {} : { amount: parsed }, {
      onSuccess: () => {
        setOpen(false);
        setAmount("");
        toast.success(t("aff_payout_submitted"));
      },
      onError: (err) => {
        const key = err instanceof ApiError ? PAYOUT_ERRORS[err.code] : undefined;
        toast.error(key ? t(key) : err instanceof ApiError ? err.message : t("aff_error"));
      },
    });
  };

  return (
    <>
      <Button onClick={() => setOpen(true)} disabled={disabled}>
        {t("aff_wallet_request")}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("aff_wallet_dialog_title")}</DialogTitle>
            <DialogDescription>{t("aff_wallet_dialog_desc").replace("{bank}", bank)}</DialogDescription>
          </DialogHeader>
          <form onSubmit={submit} className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="payout_amount">{t("aff_col_amount")}</Label>
              <Input id="payout_amount" inputMode="numeric" value={amount} onChange={(e) => setAmount(e.target.value)} />
              <p className="text-xs text-ink-500">{t("aff_wallet_amount_hint").replace("{saldo}", formatRupiah(saldo))}</p>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setOpen(false)} disabled={request.isPending}>
                {t("aff_cancel_btn")}
              </Button>
              <Button type="submit" disabled={request.isPending}>
                {t("aff_wallet_request")}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}

function HistoryTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliateLedger({
    page: params.page,
    per_page: params.per_page,
    q: params.q,
    type: params.type,
    from: params.from,
    to: params.to,
    order: "desc",
  });

  const columns: DataTableColumn<AffiliateLedgerEntry>[] = [
    { key: "date", header: t("aff_col_date"), cell: (r) => formatAffDate(r.created_at) },
    { key: "desc", header: t("aff_col_description"), cell: (r) => ledgerDescription(t, r) },
    { key: "kind", header: t("aff_col_kind"), cell: (r) => affLabel(t, r.entry_type) },
    {
      key: "amount",
      header: t("aff_col_amount"),
      align: "right",
      cell: (r) => <span className={r.amount < 0 ? "font-semibold text-danger" : "font-semibold text-ink-900"}>{formatRupiah(r.amount)}</span>,
    },
  ];

  return (
    <div className="space-y-4">
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("aff_wallet_search")}
        filters={[{ key: "type", label: t("aff_wallet_filter_kind"), options: LEDGER_KINDS.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("aff_wallet_filter_date")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_wallet_ledger_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}

function PayoutsTab() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = useAffiliatePayouts({
    page: params.page,
    per_page: params.per_page,
    status: params.status,
    from: params.from,
    to: params.to,
  });

  const columns: DataTableColumn<AffiliatePayout>[] = [
    { key: "requested", header: t("aff_col_requested"), cell: (r) => formatAffDate(r.requested_at) },
    { key: "amount", header: t("aff_col_amount"), align: "right", cell: (r) => <span className="font-semibold">{formatRupiah(r.amount)}</span> },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "processed", header: t("aff_col_processed"), cell: (r) => formatAffDate(r.processed_at) },
    { key: "reason", header: t("aff_col_reason"), cell: (r) => r.reject_reason ?? "—" },
  ];

  return (
    <div className="space-y-4">
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        filters={[{ key: "status", label: t("aff_col_status"), options: PAYOUT_STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("aff_wallet_filter_requested")}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("aff_payout_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
