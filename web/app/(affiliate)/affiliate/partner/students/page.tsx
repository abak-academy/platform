"use client";

import { Suspense, useState } from "react";
import { Download, GraduationCap, Upload, UserPlus } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { BulkImportModal } from "@/components/admin/BulkImportModal";
import { RegisterStudentDialog } from "@/components/partner/RegisterStudentDialog";
import { ReissueCredentialsDialog } from "@/components/partner/ReissueCredentialsDialog";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { downloadCsv } from "@/lib/csv";
import { fetchAllPartnerPages, usePartnerSchools, usePartnerStudentsPage } from "@/lib/hooks/partner";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useTranslation } from "@/lib/i18n";
import type { PartnerStudent } from "@/lib/types";

export default function PartnerStudentsPage() {
  return (
    <Suspense fallback={null}>
      <StudentsContent />
    </Suspense>
  );
}

function StudentsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const [registering, setRegistering] = useState(false);
  const [importing, setImporting] = useState(false);
  const [reissuing, setReissuing] = useState<PartnerStudent | null>(null);
  const schools = usePartnerSchools();
  const filter = {
    q: params.q,
    status: params.status,
    school_id: params.school,
    registered_by_me: params.type === "mine",
  };
  const query = usePartnerStudentsPage({ ...filter, page: params.page, per_page: params.per_page });
  const noSchools = query.data?.hint === "no_schools";

  const exportCsv = async () => {
    try {
      const all = await fetchAllPartnerPages<PartnerStudent>("/partners/me/students", filter);
      downloadCsv(
        "siswa-mitra.csv",
        ["name", "username", "school", "grade", "jenjang", "status", "activated", "registered_by_me"],
        all.map((s) => [s.name, s.username, s.school_name, s.grade, s.jenjang, s.status, s.activated ? "yes" : "no", s.registered_by_me ? "yes" : "no"]),
      );
    } catch {
      toast.error(t("partner_csv_failed"));
    }
  };

  const columns: DataTableColumn<PartnerStudent>[] = [
    { key: "name", header: t("partner_col_name"), cell: (s) => <span className="font-medium text-ink-900">{s.name}</span> },
    { key: "username", header: t("partner_col_username"), cell: (s) => s.username ?? "-" },
    { key: "school", header: t("partner_col_school"), cell: (s) => s.school_name ?? "-" },
    {
      key: "source",
      header: t("partner_students_col_source"),
      cell: (s) => (
        <span className={s.registered_by_me ? "rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary" : "text-xs text-ink-500"}>
          {t(s.registered_by_me ? "partner_students_source_me" : "partner_students_source_other")}
        </span>
      ),
    },
    { key: "grade", header: t("partner_col_grade"), cell: (s) => (s.grade === null ? "-" : String(s.grade)) },
    {
      key: "activated",
      header: t("partner_col_activated"),
      cell: (s) => <span className={s.activated ? "text-success" : "text-warn"}>{t(s.activated ? "partner_student_activated" : "partner_student_pending")}</span>,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (s) =>
        s.registered_by_me && !s.activated ? (
          <Button size="sm" variant="outline" onClick={() => setReissuing(s)}>
            {t("partner_student_reissue")}
          </Button>
        ) : null,
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader
        icon={GraduationCap}
        title={t("partner_students_title")}
        description={t("partner_students_desc")}
        actions={
          <>
            <Button variant="outline" onClick={exportCsv}>
              <Download className="mr-2 size-4" />
              {t("partner_csv_download")}
            </Button>
            <Button variant="outline" onClick={() => setImporting(true)}>
              <Upload className="mr-2 size-4" />
              {t("partner_students_import")}
            </Button>
            <Button onClick={() => setRegistering(true)}>
              <UserPlus className="mr-2 size-4" />
              {t("partner_students_register")}
            </Button>
          </>
        }
      />
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("partner_students_search")}
        filters={[
          { key: "type", label: t("partner_students_filter_registered"), options: [{ value: "mine", label: t("partner_students_source_me") }] },
          {
            key: "school",
            label: t("partner_students_filter_school_label"),
            options: (schools.data?.data ?? []).map((sc) => ({ value: sc.id, label: sc.name })),
          },
          {
            key: "status",
            label: t("partner_students_filter_account"),
            options: [
              { value: "active", label: t("partner_students_status_active") },
              { value: "deactivated", label: t("partner_students_status_deactivated") },
            ],
          },
        ]}
      />
      <AffiliateList
        query={query}
        columns={columns}
        rowKey={(s) => s.id}
        page={params.page}
        onPageChange={(page) => update({ page })}
        empty={noSchools ? t("partner_students_no_schools") : t("partner_students_empty")}
        filtered={hasFilters}
      />

      <RegisterStudentDialog open={registering} onOpenChange={setRegistering} />
      <BulkImportModal open={importing} onOpenChange={setImporting} allowExplicitPassword={false} partner />
      <ReissueCredentialsDialog student={reissuing} onClose={() => setReissuing(null)} />
    </div>
  );
}
