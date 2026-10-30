"use client";

import { Suspense, useState } from "react";
import { Download, Wallet } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { PayoutProofDialog } from "@/components/affiliate/PayoutProofDialog";
import { ReasonDialog } from "@/components/affiliate/ReasonDialog";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import {
  downloadAffiliateCsv,
  useAdminAffiliatePayouts,
  useDecideAffiliatePayout,
  useProcessAffiliatePayout,
} from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdminPayout } from "@/lib/types";

const STATUSES = ["requested", "processing", "paid", "rejected"] as const;

export default function AdminAffiliatePayoutsPage() {
  return (
    <Suspense fallback={null}>
      <PayoutsContent />
    </Suspense>
  );
}

function PayoutsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const canManage = useHasCapability("affiliates:*");
  const [paying, setPaying] = useState<AffiliateAdminPayout | null>(null);
  const [rejecting, setRejecting] = useState<AffiliateAdminPayout | null>(null);
  const queue = { q: params.q, status: params.status, from: params.from, to: params.to };
  const query = useAdminAffiliatePayouts({ ...queue, page: params.page, per_page: params.per_page });
  const process = useProcessAffiliatePayout();
  const decide = useDecideAffiliatePayout();

  if (!canManage) return <NoAccess />;

  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));
  const done = () => {
    toast.success(t("admin_aff_pay_done"));
    setPaying(null);
    setRejecting(null);
  };

  const exportCsv = async () => {
    try {
      await downloadAffiliateCsv("payouts", queue);
    } catch {
      toast.error(t("admin_aff_export_failed"));
    }
  };

  const columns: DataTableColumn<AffiliateAdminPayout>[] = [
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
    { key: "requested", header: t("aff_col_requested"), cell: (r) => formatAffDate(r.requested_at) },
    { key: "amount", header: t("aff_col_amount"), align: "right", cell: (r) => formatRupiah(r.amount) },
    {
      key: "bank",
      header: t("admin_aff_pay_col_bank"),
      cell: (r) => (
        <div className="text-xs">
          <div>
            {r.bank_name} · {r.bank_account_number}
          </div>
          <div className="text-ink-500">{r.bank_account_holder}</div>
        </div>
      ),
    },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    {
      key: "actions",
      header: t("admin_aff_col_actions"),
      cell: (r) => (
        <div className="flex flex-wrap gap-2">
          {r.status === "requested" && (
            <Button
              size="sm"
              variant="outline"
              disabled={process.isPending}
              onClick={() => process.mutate(r.id, { onSuccess: done, onError: fail })}
            >
              {t("admin_aff_pay_process")}
            </Button>
          )}
          {(r.status === "requested" || r.status === "processing") && (
            <>
              <Button size="sm" onClick={() => setPaying(r)}>
                {t("admin_aff_pay_paid")}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setRejecting(r)}>
                {t("admin_aff_pay_reject")}
              </Button>
            </>
          )}
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader
        icon={Wallet}
        title={t("admin_aff_pay_title")}
        description={t("admin_aff_pay_desc")}
      />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("admin_aff_search_queue")}
        filters={[{ key: "status", label: t("aff_col_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) }]}
        dateLabel={t("admin_aff_date_requested")}
      />
      <div className="flex justify-end">
        <Button variant="outline" onClick={exportCsv}>
          <Download className="size-4" aria-hidden />
          {t("admin_aff_export_csv")}
        </Button>
      </div>
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_pay_empty")}
        filtered={hasFilters}
      />

      <PayoutProofDialog
        open={paying !== null}
        onOpenChange={(open) => !open && setPaying(null)}
        description={paying ? `${paying.affiliator_name} · ${formatRupiah(paying.amount)}` : undefined}
        isPending={decide.isPending}
        onConfirm={(proof) =>
          paying && decide.mutate({ id: paying.id, decision: "paid", proof_url: proof }, { onSuccess: done, onError: fail })
        }
      />
      <ReasonDialog
        open={rejecting !== null}
        onOpenChange={(open) => !open && setRejecting(null)}
        title={t("admin_aff_pay_reject_title")}
        description={rejecting ? `${rejecting.affiliator_name} · ${formatRupiah(rejecting.amount)}` : undefined}
        isPending={decide.isPending}
        onConfirm={(reason) =>
          rejecting && decide.mutate({ id: rejecting.id, decision: "reject", reason }, { onSuccess: done, onError: fail })
        }
      />
    </div>
  );
}
