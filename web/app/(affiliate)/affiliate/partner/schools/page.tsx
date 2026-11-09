"use client";

import { Download, School } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { downloadCsv } from "@/lib/csv";
import { usePartnerSchools } from "@/lib/hooks/partner";
import { useTranslation } from "@/lib/i18n";
import type { PartnerSchool } from "@/lib/types";

export default function PartnerSchoolsPage() {
  const { t } = useTranslation();
  const query = usePartnerSchools();
  const rows = query.data?.data ?? [];

  const exportCsv = () => {
    try {
      downloadCsv("sekolah-mitra.csv", ["name", "npsn", "code", "status"], rows.map((s) => [s.name, s.npsn, s.code, s.status]));
    } catch {
      toast.error(t("partner_csv_failed"));
    }
  };

  const columns: DataTableColumn<PartnerSchool>[] = [
    { key: "name", header: t("partner_schools_col_name"), cell: (s) => <span className="font-medium text-ink-900">{s.name}</span> },
    { key: "npsn", header: t("partner_schools_col_npsn"), cell: (s) => <span className="font-mono">{s.npsn ?? "-"}</span> },
    { key: "code", header: t("partner_schools_col_code"), cell: (s) => <span className="font-mono">{s.code}</span> },
    {
      key: "status",
      header: t("partner_schools_col_status"),
      cell: (s) => <span className={s.status === "active" ? "text-success" : "text-warn"}>{s.status}</span>,
    },
  ];

  return (
    <div className="space-y-4">
      <AdminPageHeader icon={School} title={t("partner_schools_title")} description={t("partner_schools_desc")} />
      <div className="flex justify-end">
        <Button variant="outline" onClick={exportCsv} disabled={rows.length === 0}>
          <Download className="mr-2 size-4" />
          {t("partner_csv_download")}
        </Button>
      </div>
      {query.isLoading ? (
        <Skeleton className="h-32 w-full" />
      ) : query.isError ? (
        <div role="alert" className="space-y-2 text-sm text-danger">
          <p>{t("partner_load_failed")}</p>
          <Button size="sm" variant="outline" onClick={() => query.refetch()}>
            {t("partner_retry")}
          </Button>
        </div>
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(s) => s.id} empty={t("partner_schools_empty")} />
      )}
    </div>
  );
}
