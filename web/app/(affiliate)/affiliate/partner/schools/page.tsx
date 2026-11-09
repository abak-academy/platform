"use client";

import { Suspense } from "react";
import { Download, School } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliateList } from "@/components/affiliate/AffiliateList";
import { ListToolbar } from "@/components/affiliate/ListToolbar";
import { Button } from "@/components/ui/button";
import type { DataTableColumn } from "@/components/ui/data-table";
import { downloadCsv } from "@/lib/csv";
import { fetchAllPartnerPages, usePartnerSchools } from "@/lib/hooks/partner";
import { useListParams } from "@/lib/hooks/use-list-params";
import { useTranslation } from "@/lib/i18n";
import type { PartnerSchool } from "@/lib/types";

export default function PartnerSchoolsPage() {
  return (
    <Suspense fallback={null}>
      <SchoolsContent />
    </Suspense>
  );
}

function SchoolsContent() {
  const { t } = useTranslation();
  const { params, update, reset, hasFilters } = useListParams();
  const query = usePartnerSchools({ q: params.q, status: params.status, page: params.page, per_page: params.per_page });

  const exportCsv = async () => {
    try {
      const all = await fetchAllPartnerPages<PartnerSchool>("/partners/me/schools", { q: params.q, status: params.status });
      downloadCsv("sekolah-mitra.csv", ["name", "npsn", "code", "status"], all.map((s) => [s.name, s.npsn, s.code, s.status]));
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
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="outline" onClick={exportCsv}>
          <Download className="mr-2 size-4" />
          {t("partner_csv_download")}
        </Button>
      </div>
      <ListToolbar
        params={params}
        onChange={update}
        onReset={reset}
        searchPlaceholder={t("partner_schools_search")}
        filters={[
          {
            key: "status",
            label: t("partner_schools_col_status"),
            options: [
              { value: "active", label: "active" },
              { value: "inactive", label: "inactive" },
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
        empty={t("partner_schools_empty")}
        filtered={hasFilters}
      />
    </div>
  );
}
