"use client";

import { Suspense, use } from "react";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { NoAccess } from "@/components/admin/NoAccess";
import { AdminCommissionsPanel } from "@/components/affiliate/AdminCommissionsPanel";
import { AffiliateAdminActions } from "@/components/affiliate/AffiliateAdminActions";
import { LevelBadge } from "@/components/affiliate/LevelBadge";
import { StatusBadge } from "@/components/affiliate/StatusBadge";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { ApiError } from "@/lib/api";
import { affLabel, formatAffDate } from "@/lib/affiliate-labels";
import { useAdminAffiliate } from "@/lib/hooks/admin-affiliates";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { AffiliateBinding } from "@/lib/types";

export default function AdminAffiliateDetailPage({ params }: { params: Promise<{ id: string }> }) {
  return (
    <Suspense fallback={null}>
      <DetailRoute params={params} />
    </Suspense>
  );
}

function Card({ label, value }: { label: string; value: string }) {
  return (
    <div className="md-card-outlined p-4">
      <div className="text-xs text-ink-500">{label}</div>
      <div className="mt-1 font-serif text-xl font-bold text-ink-900">{value}</div>
    </div>
  );
}

function DetailRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const query = useAdminAffiliate(id);

  if (!canManage) return <NoAccess />;
  const back = (
    <Button variant="outline" size="sm" asChild className="w-fit">
      <Link href="/admin/affiliates">
        <ArrowLeft className="size-4" aria-hidden />
        {t("admin_aff_back")}
      </Link>
    </Button>
  );

  if (query.isLoading) return <div className="text-sm text-ink-500">{t("aff_loading")}</div>;
  if (query.isError || !query.data) {
    if (query.error instanceof ApiError && query.error.status === 404) {
      return (
        <div className="space-y-3">
          {back}
          <p className="text-sm text-ink-600">{t("admin_aff_not_found")}</p>
        </div>
      );
    }
    return (
      <div role="alert" className="md-card-outlined space-y-3 p-6 text-sm text-ink-600">
        <p>{t("aff_error")}</p>
        <Button variant="outline" size="sm" onClick={() => query.refetch()}>
          {t("aff_retry")}
        </Button>
      </div>
    );
  }

  const d = query.data;
  const aff = d.affiliate;

  const bindingColumns: DataTableColumn<AffiliateBinding>[] = [
    { key: "name", header: t("aff_col_name"), cell: (r) => r.name },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "started", header: t("aff_col_started"), cell: (r) => formatAffDate(r.started_at) },
    { key: "ended", header: t("aff_col_ended"), cell: (r) => formatAffDate(r.expires_at) },
    { key: "spend", header: t("aff_col_spend"), align: "right", cell: (r) => formatRupiah(r.total_spend) },
  ];

  return (
    <div className="space-y-6">
      {back}
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="font-serif text-2xl font-bold text-ink-900">{d.name}</h1>
        <StatusBadge status={aff.status} />
        <LevelBadge level={aff.level} />
      </div>
      <p className="text-sm text-ink-600">
        {d.email ?? "—"} · {aff.code || "—"}
      </p>

      <div className="grid gap-4 sm:grid-cols-3">
        <Card label={t("admin_aff_detail_balance")} value={formatRupiah(d.saldo)} />
        <Card label={t("admin_aff_detail_bindings")} value={`${d.bindings.active} / ${d.bindings.ended}`} />
        <Card label={t("admin_aff_detail_commissions")} value={formatRupiah(d.commissions.total_amount)} />
      </div>

      <section className="space-y-2">
        <h2 className="font-semibold text-ink-900">{t("admin_aff_actions")}</h2>
        <AffiliateAdminActions id={aff.id} currentStatus={aff.status} currentLevel={aff.level} />
      </section>

      <section className="space-y-2">
        <h2 className="font-semibold text-ink-900">{t("admin_aff_sec_bindings")}</h2>
        <DataTable columns={bindingColumns} rows={d.bindings.data} rowKey={(r) => r.binding_id} empty={t("aff_students_empty")} />
      </section>
      <section>
        <AdminCommissionsPanel affiliatorId={aff.id} />
      </section>
    </div>
  );
}
