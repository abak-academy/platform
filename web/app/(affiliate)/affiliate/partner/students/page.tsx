"use client";

import { useState } from "react";
import { Download, GraduationCap, Upload, UserPlus } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { BulkImportModal } from "@/components/admin/BulkImportModal";
import { RegisterStudentDialog } from "@/components/partner/RegisterStudentDialog";
import { ReissueCredentialsDialog } from "@/components/partner/ReissueCredentialsDialog";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { downloadCsv } from "@/lib/csv";
import { fetchAllPartnerPages, usePartnerSchools, usePartnerStudents } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import type { PartnerStudent } from "@/lib/types";

export default function PartnerStudentsPage() {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [mine, setMine] = useState(false);
  const [schoolId, setSchoolId] = useState("");
  const [status, setStatus] = useState("");
  const schools = usePartnerSchools();
  const [registering, setRegistering] = useState(false);
  const [importing, setImporting] = useState(false);
  const [reissuing, setReissuing] = useState<PartnerStudent | null>(null);
  const debounced = useDebouncedValue(q, 300);
  const query = usePartnerStudents({ q: debounced, registeredByMe: mine, schoolId, status });

  const rows = query.data?.pages.flatMap((p) => p.data) ?? [];
  const noSchools = rows.length === 0 && query.data?.pages[0]?.hint === "no_schools";

  const exportCsv = async () => {
    try {
      const params: Record<string, string> = {};
      if (debounced) params.q = debounced;
      if (mine) params.registered_by_me = "true";
      if (schoolId) params.school_id = schoolId;
      if (status) params.status = status;
      const all = await fetchAllPartnerPages<PartnerStudent>("/partners/me/students", params);
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
      <AdminPageHeader icon={GraduationCap} title={t("partner_students_title")} description={t("partner_students_desc")} />
      <div className="flex flex-wrap items-center gap-3">
        <Input className="max-w-xs" value={q} onChange={(e) => setQ(e.target.value)} placeholder={t("partner_students_search")} aria-label={t("partner_students_search")} />
        <div className="flex gap-1" role="group" aria-label={t("partner_students_col_source")}>
          <Button size="sm" variant={mine ? "outline" : "default"} aria-pressed={!mine} onClick={() => setMine(false)}>
            {t("partner_students_filter_all")}
          </Button>
          <Button size="sm" variant={mine ? "default" : "outline"} aria-pressed={mine} onClick={() => setMine(true)}>
            {t("partner_students_filter_mine")}
          </Button>
        </div>
        <select
          aria-label={t("partner_col_school")}
          value={schoolId}
          onChange={(e) => setSchoolId(e.target.value)}
          className="h-9 max-w-[14rem] rounded-md border border-input bg-transparent px-2 text-sm"
        >
          <option value="">{t("partner_students_filter_school")}</option>
          {(schools.data?.data ?? []).map((sc) => (
            <option key={sc.id} value={sc.id}>
              {sc.name}
            </option>
          ))}
        </select>
        <select
          aria-label={t("partner_students_status_label")}
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-md border border-input bg-transparent px-2 text-sm"
        >
          <option value="">{t("partner_students_filter_status_all")}</option>
          <option value="active">{t("partner_students_status_active")}</option>
          <option value="deactivated">{t("partner_students_status_deactivated")}</option>
        </select>
        <div className="ml-auto flex gap-2">
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
        </div>
      </div>

      {query.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : query.isError ? (
        <div role="alert" className="space-y-2 text-sm text-danger">
          <p>{t("partner_load_failed")}</p>
          <Button size="sm" variant="outline" onClick={() => query.refetch()}>
            {t("partner_retry")}
          </Button>
        </div>
      ) : (
        <>
          <DataTable columns={columns} rows={rows} rowKey={(s) => s.id} empty={noSchools ? t("partner_students_no_schools") : t("partner_students_empty")} />
          {query.hasNextPage && (
            <div className="flex justify-center">
              <Button variant="outline" onClick={() => query.fetchNextPage()} disabled={query.isFetchingNextPage}>
                {t("partner_load_more")}
              </Button>
            </div>
          )}
        </>
      )}

      <RegisterStudentDialog open={registering} onOpenChange={setRegistering} />
      <BulkImportModal open={importing} onOpenChange={setImporting} allowExplicitPassword={false} partner />
      <ReissueCredentialsDialog student={reissuing} onClose={() => setReissuing(null)} />
    </div>
  );
}
