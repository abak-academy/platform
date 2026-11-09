"use client";

import { useState } from "react";
import { GraduationCap, Upload, UserPlus } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { BulkImportModal } from "@/components/admin/BulkImportModal";
import { RegisterStudentDialog } from "@/components/partner/RegisterStudentDialog";
import { ReissueCredentialsDialog } from "@/components/partner/ReissueCredentialsDialog";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { usePartnerStudents } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import type { PartnerStudent } from "@/lib/types";

export default function PartnerStudentsPage() {
  const { t } = useTranslation();
  const [q, setQ] = useState("");
  const [mine, setMine] = useState(false);
  const [registering, setRegistering] = useState(false);
  const [importing, setImporting] = useState(false);
  const [reissuing, setReissuing] = useState<PartnerStudent | null>(null);
  const debounced = useDebouncedValue(q, 300);
  const query = usePartnerStudents({ q: debounced, registeredByMe: mine });

  const rows = query.data?.pages.flatMap((p) => p.data) ?? [];
  const noSchools = rows.length === 0 && query.data?.pages[0]?.hint === "no_schools";

  const columns: DataTableColumn<PartnerStudent>[] = [
    { key: "name", header: t("partner_col_name"), cell: (s) => <span className="font-medium text-ink-900">{s.name}</span> },
    { key: "username", header: t("partner_col_username"), cell: (s) => s.username ?? "-" },
    { key: "school", header: t("partner_col_school"), cell: (s) => s.school_name ?? "-" },
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
        <label className="flex items-center gap-2 text-sm text-ink-700">
          <input type="checkbox" checked={mine} onChange={(e) => setMine(e.target.checked)} />
          {t("partner_students_filter_mine")}
        </label>
        <div className="ml-auto flex gap-2">
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
