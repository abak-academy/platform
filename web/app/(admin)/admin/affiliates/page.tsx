"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { HandCoins } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { ReasonDialog } from "@/components/affiliate/ReasonDialog";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { useAdminAffiliates, useDecideAffiliateApproval } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdminRow } from "@/lib/types";

const STATUSES = ["pending_approval", "active", "suspended", "deactivated", "rejected"] as const;
const LEVELS = ["bronze", "silver", "gold"] as const;

export default function AdminAffiliatesPage() {
  return (
    <Suspense fallback={null}>
      <AffiliatesContent />
    </Suspense>
  );
}

function AffiliatesContent() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const { params, update, reset, hasFilters } = useListParams();
  const [rejecting, setRejecting] = useState<AffiliateAdminRow | null>(null);
  const query = useAdminAffiliates({ status: params.status, level: params.type, q: params.q, page: params.page, per_page: params.per_page });
  const decide = useDecideAffiliateApproval();

  if (!canManage) return <NoAccess />;

  const fail = (err: unknown) => toast.error(err instanceof ApiError ? err.message : t("aff_action_failed"));

  const approve = (row: AffiliateAdminRow) =>
    decide.mutate(
      { id: row.id, decision: "approve" },
      { onSuccess: () => toast.success(t("admin_aff_approved")), onError: fail },
    );

  const reject = (reason: string) => {
    if (!rejecting) return;
    decide.mutate(
      { id: rejecting.id, decision: "reject", reason },
      {
        onSuccess: () => {
          toast.success(t("admin_aff_rejected"));
          setRejecting(null);
        },
        onError: fail,
      },
    );
  };

  const columns: DataTableColumn<AffiliateAdminRow>[] = [
    {
      key: "who",
      header: t("admin_aff_col_affiliator"),
      cell: (r) => (
        <div>
          <div className="font-medium text-ink-900">{r.name}</div>
          <div className="text-xs text-ink-500">{r.email ?? "—"}</div>
        </div>
      ),
    },
    { key: "code", header: t("admin_aff_col_code"), cell: (r) => r.code || "—" },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "level", header: t("admin_aff_col_level"), cell: (r) => affLabel(t, r.level) },
    { key: "joined", header: t("admin_aff_col_joined"), cell: (r) => formatAffDate(r.created_at) },
    {
      key: "actions",
      header: t("admin_aff_col_actions"),
      cell: (r) => (
        <div className="flex flex-wrap gap-2">
          {r.status === "pending_approval" && (
            <>
              <Button size="sm" onClick={() => approve(r)} disabled={decide.isPending}>
                {t("admin_aff_approve")}
              </Button>
              <Button size="sm" variant="outline" onClick={() => setRejecting(r)} disabled={decide.isPending}>
                {t("admin_aff_reject")}
              </Button>
            </>
          )}
          <Button size="sm" variant="ghost" asChild>
            <Link href={`/admin/affiliates/${r.id}`}>{t("admin_aff_view")}</Link>
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={HandCoins} title={t("admin_aff_title")} description={t("admin_aff_desc")} />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("admin_aff_search_placeholder")}
        filters={[
          { key: "status", label: t("aff_col_status"), options: STATUSES.map((v) => ({ value: v, label: affLabel(t, v) })) },
          { key: "type", label: t("admin_aff_col_level"), options: LEVELS.map((v) => ({ value: v, label: affLabel(t, v) })) },
        ]}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={t("admin_aff_empty")}
        filtered={hasFilters}
      />
      <ReasonDialog
        open={rejecting !== null}
        onOpenChange={(open) => !open && setRejecting(null)}
        title={t("admin_aff_reject_title")}
        description={rejecting?.name}
        isPending={decide.isPending}
        onConfirm={reject}
      />
    </div>
  );
}
