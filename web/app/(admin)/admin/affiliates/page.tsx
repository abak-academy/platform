"use client";

import { useState } from "react";
import Link from "next/link";
import { HandCoins } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ReasonDialog } from "@/components/affiliate/ReasonDialog";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { useAdminAffiliates, useDecideAffiliateApproval } from "@/lib/hooks/admin-affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateAdminRow } from "@/lib/types";

const STATUSES = ["pending_approval", "active", "suspended", "deactivated", "rejected"] as const;
const LEVELS = ["bronze", "silver", "gold"] as const;

export default function AdminAffiliatesPage() {
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const [status, setStatus] = useState("");
  const [level, setLevel] = useState("");
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [rejecting, setRejecting] = useState<AffiliateAdminRow | null>(null);
  const q = useDebouncedValue(search.trim());
  const query = useAdminAffiliates({ status, level, q, page });
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
      <div className="flex flex-wrap gap-3">
        <Input
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(1);
          }}
          placeholder={t("admin_aff_search_placeholder")}
          aria-label={t("admin_aff_search_placeholder")}
          className="max-w-xs"
        />
        <select
          aria-label={t("aff_filter_status")}
          value={status}
          onChange={(e) => {
            setStatus(e.target.value);
            setPage(1);
          }}
          className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
        >
          <option value="">{t("aff_filter_all")}</option>
          {STATUSES.map((s) => (
            <option key={s} value={s}>
              {affLabel(t, s)}
            </option>
          ))}
        </select>
        <select
          aria-label={t("admin_aff_filter_level")}
          value={level}
          onChange={(e) => {
            setLevel(e.target.value);
            setPage(1);
          }}
          className="h-9 rounded-md border border-line bg-surface px-3 text-sm"
        >
          <option value="">{t("admin_aff_all_levels")}</option>
          {LEVELS.map((l) => (
            <option key={l} value={l}>
              {affLabel(t, l)}
            </option>
          ))}
        </select>
      </div>
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(r) => r.id}
        page={page}
        onPageChange={setPage}
        empty={t("admin_aff_empty")}
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
