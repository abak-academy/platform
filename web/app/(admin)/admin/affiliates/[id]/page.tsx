"use client";

import { Suspense, use } from "react";
import { Banknote, HandCoins, Link2 } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
import { NoAccess } from "@/components/admin/NoAccess";
import { AdminCommissionsPanel } from "@/components/affiliate/AdminCommissionsPanel";
import { AffiliateAdminActions } from "@/components/affiliate/AffiliateAdminActions";
import { LevelBadge } from "@/components/affiliate/LevelBadge";
import { inAffiliate, ProgramBadges } from "@/components/affiliate/ProgramBadges";
import { PartnerProgramPanel } from "@/components/affiliate/PartnerProgramPanel";
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

function DetailRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { t } = useTranslation();
  const canManage = useHasCapability("affiliates:*");
  const query = useAdminAffiliate(id);

  if (!canManage) return <NoAccess />;
  const back = { href: "/admin/affiliates", label: t("admin_aff_back") };

  if (query.isLoading) return <div className="text-sm text-ink-500">{t("aff_loading")}</div>;
  if (query.isError || !query.data) {
    if (query.error instanceof ApiError && query.error.status === 404) {
      return (
        <div className="space-y-3">
          <AdminPageHeader title={t("admin_aff_not_found")} back={back} />
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
  const inV1 = inAffiliate(aff.programs);

  const bindingColumns: DataTableColumn<AffiliateBinding>[] = [
    { key: "name", header: t("aff_col_name"), cell: (r) => r.name },
    { key: "status", header: t("aff_col_status"), cell: (r) => <StatusBadge status={r.status} /> },
    { key: "started", header: t("aff_col_started"), cell: (r) => formatAffDate(r.started_at) },
    { key: "ended", header: t("aff_col_ended"), cell: (r) => formatAffDate(r.expires_at) },
    { key: "spend", header: t("aff_col_spend"), align: "right", cell: (r) => formatRupiah(r.total_spend) },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader
        title={d.name}
        description={`${d.email ?? "—"} · ${aff.code || "—"}`}
        back={back}
        actions={
          <>
            <StatusBadge status={aff.status} />
            <ProgramBadges programs={aff.programs} />
            {inV1 && <LevelBadge level={aff.level} />}
          </>
        }
      />

      {inV1 && (
        <div className="grid gap-4 sm:grid-cols-3">
          <StatCard label={t("admin_aff_detail_balance")} value={formatRupiah(d.saldo)} icon={Banknote} tone="brand" />
          <StatCard label={t("admin_aff_detail_bindings")} value={`${d.bindings.active} / ${d.bindings.ended}`} icon={Link2} tone="sky" />
          <StatCard label={t("admin_aff_detail_commissions")} value={formatRupiah(d.commissions.total_amount)} icon={HandCoins} tone="teal" />
        </div>
      )}

      <PartnerProgramPanel id={aff.id} programs={aff.programs} />

      <section className="space-y-2">
        <h2 className="type-h3 text-ink-900">{t("admin_aff_actions")}</h2>
        <AffiliateAdminActions id={aff.id} currentStatus={aff.status} currentLevel={aff.level} affiliate={inV1} programs={aff.programs} />
      </section>

      {inV1 && (
        <>
          <section className="space-y-2">
            <h2 className="type-h3 text-ink-900">{t("admin_aff_sec_bindings")}</h2>
            <DataTable columns={bindingColumns} rows={d.bindings.data} rowKey={(r) => r.binding_id} empty={t("aff_students_empty")} />
          </section>
          <section>
            <AdminCommissionsPanel affiliatorId={aff.id} />
          </section>
        </>
      )}
    </div>
  );
}
