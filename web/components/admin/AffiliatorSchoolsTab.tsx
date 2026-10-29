"use client";

import { useState } from "react";
import { Download, Search, SlidersHorizontal, X } from "lucide-react";
import { toast } from "sonner";
import { AffiliatorFilterPicker } from "@/components/admin/AffiliatorFilterPicker";
import { TablePager } from "@/components/admin/TablePager";
import { SchoolFilterPicker } from "@/components/SchoolFilterPicker";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { downloadAffiliatorSchoolsCsv, useAdminAffiliator, useAffiliatorSchools } from "@/lib/hooks/admin-affiliators";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useOffsetPaging } from "@/lib/hooks/use-offset-paging";
import { useSchoolById } from "@/lib/hooks/students";
import { useTranslation } from "@/lib/i18n";
import type { AffiliatorSchoolRow } from "@/lib/types";

// Which schools each affiliator has, one row per assignment. Search covers both
// sides; the filters narrow to one affiliator and/or one school; the export is
// the same list, whole.
export function AffiliatorSchoolsTab() {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [affiliatorId, setAffiliatorId] = useState("");
  const [schoolId, setSchoolId] = useState("");
  const [limit, setLimit] = useState(20);
  const [exporting, setExporting] = useState(false);
  const q = useDebouncedValue(search).trim();
  const paging = useOffsetPaging([q, affiliatorId, schoolId, limit].join("|"), limit);
  const params = { q: q || undefined, affiliatorId: affiliatorId || undefined, schoolId: schoolId || undefined };
  const list = useAffiliatorSchools({ ...params, limit, offset: paging.offset });
  const rows = list.data?.data ?? [];
  const total = list.data?.total ?? 0;
  const affiliatorName = useAdminAffiliator(affiliatorId || null).data?.affiliator.name;
  const schoolName = useSchoolById(schoolId).data?.name;
  const active = [affiliatorId, schoolId].filter(Boolean).length;

  const chips = [
    ...(affiliatorId ? [{ key: "affiliator", label: `${t("aff_schools_filter_affiliator")}: ${affiliatorName ?? affiliatorId}`, clear: () => setAffiliatorId("") }] : []),
    ...(schoolId ? [{ key: "school", label: `${t("aff_schools_filter_school")}: ${schoolName ?? schoolId}`, clear: () => setSchoolId("") }] : []),
  ];

  async function exportCsv() {
    setExporting(true);
    try {
      await downloadAffiliatorSchoolsCsv(params);
    } catch {
      toast.error(t("aff_report_export_failed"));
    } finally {
      setExporting(false);
    }
  }

  const columns: DataTableColumn<AffiliatorSchoolRow>[] = [
    {
      key: "school",
      header: t("aff_schools_th_school"),
      cell: (r) => (
        <div>
          <div className="font-medium">{r.school_name}</div>
          <div className="text-xs text-ink-600">{[r.school_npsn, r.school_code].filter(Boolean).join(" · ")}</div>
        </div>
      ),
    },
    {
      key: "affiliator",
      header: t("aff_schools_th_affiliator"),
      cell: (r) => (
        <div>
          <div className="font-medium">{r.affiliator_name}</div>
          <div className="text-xs text-ink-600">{r.affiliator_email}</div>
        </div>
      ),
    },
    {
      key: "status",
      header: t("th_status"),
      cell: (r) => (
        <Badge variant="outline" className={r.affiliator_status === "active" ? "border-transparent bg-success-bg text-success" : undefined}>
          {t(r.affiliator_status === "active" ? "status_label_active" : "status_label_inactive")}
        </Badge>
      ),
    },
    { key: "students", header: t("aff_th_students"), align: "right", cell: (r) => r.student_count },
  ];

  return (
    <div className="space-y-4">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:max-w-md sm:flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
            <Input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("aff_schools_search")} aria-label={t("aff_schools_search")} className="pr-10 pl-9" />
            {search && (
              <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => setSearch("")}>
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </div>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${active ? ` (${active})` : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filters")}
                {active > 0 && <span className="tabular-nums">{active}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="admin-shell w-[min(24rem,calc(100vw-2rem))]" aria-label={t("aff_schools_filter_title")}>
              <div className="space-y-3">
                <h2 className="font-semibold">{t("aff_schools_filter_title")}</h2>
                <span className="block text-sm font-medium">{t("aff_schools_filter_affiliator")}</span>
                <AffiliatorFilterPicker value={affiliatorId} onChange={setAffiliatorId} label={t("aff_schools_filter_affiliator")} allLabel={t("aff_schools_all_affiliators")} />
                <span className="block text-sm font-medium">{t("aff_schools_filter_school")}</span>
                <SchoolFilterPicker compact value={schoolId} onChange={setSchoolId} label={t("aff_schools_filter_school")} allLabel={t("aff_schools_all_schools")} />
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setAffiliatorId("");
                      setSchoolId("");
                    }}
                  >
                    {t("aff_reset_filters")}
                  </Button>
                  <PopoverClose asChild>
                    <Button>{t("aff_report_show")}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
          <div className="ml-auto">
            <Button variant="outline" disabled={exporting} onClick={exportCsv}>
              <Download className="size-4" aria-hidden />
              {t("aff_report_export")}
            </Button>
          </div>
        </div>
        {chips.length > 0 && (
          <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_active_filters")}>
            {chips.map((chip) => (
              <span key={chip.key} className="md-chip md-chip-primary gap-1">
                {chip.label}
                <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${chip.label}`} onClick={chip.clear}>
                  <X className="size-4" aria-hidden />
                </Button>
              </span>
            ))}
          </div>
        )}
      </div>

      {list.isError ? (
        <div className="md-card-outlined py-12 text-center text-ink-500">
          <p className="mb-4">{t("dash_load_failed")}</p>
          <button type="button" className="md-btn-tonal" onClick={() => list.refetch()}>
            {t("admin_home_reload")}
          </button>
        </div>
      ) : (
        <DataTable
          columns={columns}
          rows={list.isLoading ? [] : rows}
          rowKey={(r) => `${r.affiliator_id}:${r.school_id}`}
          empty={list.isLoading ? t("aff_loading") : t("aff_schools_empty")}
          footer={
            <TablePager
              total={total}
              offset={paging.offset}
              count={rows.length}
              limit={limit}
              onLimitChange={setLimit}
              canPrev={paging.canPrev}
              canNext={paging.offset + rows.length < total}
              onPrev={paging.prev}
              onNext={paging.next}
            />
          }
        />
      )}
    </div>
  );
}
