"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { Search, SlidersHorizontal, Upload, UserPlus, Users, X } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { AffiliatorNotice, useAffiliatorReadOnly } from "@/components/admin/AffiliatorNotice";
import { AffiliatorRegisterStudentModal } from "@/components/admin/AffiliatorRegisterStudentModal";
import { BulkImportModal } from "@/components/admin/BulkImportModal";
import { NoAccess } from "@/components/admin/NoAccess";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAdminStudents } from "@/lib/hooks/admin-students";
import { useAffiliatorMe } from "@/lib/hooks/affiliates";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { useTranslation } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import type { AdminStudent } from "@/lib/types";

const ALL = "__all__";
const PAGE_SIZE = 20;

export default function AffiliateStudentsPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("students:read");
  const readOnly = useAffiliatorReadOnly();
  const userId = useAuthStore((s) => s.user?.id);
  const params = useSearchParams();
  const me = useAffiliatorMe();
  const schools = me.data?.schools ?? [];

  const [search, setSearch] = useState("");
  const [schoolId, setSchoolId] = useState("");
  const [mine, setMine] = useState(false);
  const [notPurchased, setNotPurchased] = useState(params.get("not_purchased") === "true");
  // Cursor stack: the last entry is the current page's cursor.
  const [cursors, setCursors] = useState<string[]>([""]);
  const [registerOpen, setRegisterOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const q = useDebouncedValue(search).trim();

  const filterKey = [q, schoolId, mine, notPurchased].join("|");
  useEffect(() => setCursors([""]), [filterKey]);

  const { data, isLoading, isError, refetch } = useAdminStudents({
    q: q || undefined,
    schoolId: schoolId || undefined,
    registeredByMe: mine,
    notPurchased,
    cursor: cursors[cursors.length - 1] || undefined,
    limit: PAGE_SIZE,
  });
  const rows = data?.data ?? [];

  if (!allowed) return <NoAccess />;

  const schoolName = schools.find((s) => s.id === schoolId)?.name;
  const chips = [
    ...(schoolId ? [{ key: "school", label: `${t("aff_th_school")}: ${schoolName ?? schoolId}`, clear: () => setSchoolId("") }] : []),
    ...(mine ? [{ key: "mine", label: t("aff_filter_mine"), clear: () => setMine(false) }] : []),
    ...(notPurchased ? [{ key: "unpurchased", label: t("aff_filter_unpurchased"), clear: () => setNotPurchased(false) }] : []),
  ];

  const columns: DataTableColumn<AdminStudent>[] = [
    { key: "name", header: t("aff_register_name"), cell: (r) => <span className="font-medium">{r.name}</span> },
    { key: "jenjang", header: t("aff_th_jenjang"), cell: (r) => r.jenjang || "-" },
    { key: "school", header: t("aff_th_school"), cell: (r) => r.school_name ?? "-" },
    { key: "status", header: t("th_status"), cell: (r) => (r.status === "active" ? t("status_label_active") : t("status_label_inactive")) },
    { key: "by", header: t("aff_th_registered_by"), cell: (r) => (r.registered_by && r.registered_by === userId ? t("aff_registered_by_me") : "-") },
  ];

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader
        icon={Users}
        title={t("aff_students_title")}
        description={t("aff_students_subtitle")}
        actions={
          <>
            <Button variant="outline" onClick={() => setBulkOpen(true)} disabled={readOnly}>
              <Upload className="size-4" aria-hidden />
              {t("aff_bulk")}
            </Button>
            <Button onClick={() => setRegisterOpen(true)} disabled={readOnly}>
              <UserPlus className="size-4" aria-hidden />
              {t("aff_register")}
            </Button>
          </>
        }
      />
      <AffiliatorNotice />

      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
            <Input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={t("aff_students_search")}
              aria-label={t("aff_students_search")}
              className="pr-10 pl-9"
            />
            {search && (
              <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => setSearch("")}>
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </div>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${chips.length ? ` (${chips.length})` : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filters")}
                {chips.length > 0 && <span className="tabular-nums">{chips.length}</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="admin-shell max-h-[var(--radix-popover-content-available-height)] w-[min(24rem,calc(100vw-2rem))] overflow-y-auto" aria-label={t("aff_filter_students")}>
              <div className="space-y-3">
                <h2 className="font-semibold">{t("aff_filter_students")}</h2>
                <span className="block text-sm font-medium">{t("aff_th_school")}</span>
                <Select value={schoolId || ALL} onValueChange={(v) => setSchoolId(v === ALL ? "" : v)}>
                  <SelectTrigger className="w-full" aria-label={t("aff_th_school")}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={ALL}>{t("aff_filter_all_schools")}</SelectItem>
                    {schools.map((s) => (
                      <SelectItem key={s.id} value={s.id}>
                        {s.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={mine} onChange={(e) => setMine(e.target.checked)} />
                  {t("aff_filter_mine")}
                </label>
                <label className="flex items-center gap-2 text-sm">
                  <input type="checkbox" checked={notPurchased} onChange={(e) => setNotPurchased(e.target.checked)} />
                  {t("aff_filter_unpurchased")}
                </label>
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button
                    variant="ghost"
                    onClick={() => {
                      setSchoolId("");
                      setMine(false);
                      setNotPurchased(false);
                    }}
                  >
                    {t("aff_reset_filters")}
                  </Button>
                  <PopoverClose asChild>
                    <Button>{t("aff_show_students")}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
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

      {isError ? (
        <div className="md-card-outlined py-12 text-center text-ink-500">
          <p className="mb-4">{t("dash_load_failed")}</p>
          <button type="button" className="md-btn-tonal" onClick={() => refetch()}>
            {t("admin_home_reload")}
          </button>
        </div>
      ) : (
        <DataTable
          columns={columns}
          rows={isLoading ? [] : rows}
          rowKey={(r) => r.id}
          empty={isLoading ? t("aff_loading") : t("aff_students_empty")}
          footer={
            <div className="flex flex-wrap items-center justify-between gap-2 px-4 py-3 text-sm">
              <span>
                {t("aff_total")}: {data?.total ?? 0}
              </span>
              <div className="flex gap-2">
                <Button variant="outline" size="sm" disabled={cursors.length <= 1} onClick={() => setCursors((c) => c.slice(0, -1))}>
                  {t("aff_prev")}
                </Button>
                <Button variant="outline" size="sm" disabled={!data?.next_cursor} onClick={() => data?.next_cursor && setCursors((c) => [...c, data.next_cursor as string])}>
                  {t("aff_next")}
                </Button>
              </div>
            </div>
          }
        />
      )}

      <AffiliatorRegisterStudentModal open={registerOpen} onOpenChange={setRegisterOpen} schools={schools} defaultSchoolId={schoolId || undefined} />
      <BulkImportModal open={bulkOpen} onOpenChange={setBulkOpen} allowExplicitPassword={false} schoolPerRow />
    </div>
  );
}
