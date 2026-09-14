"use client";

import { useState, useEffect, useRef } from "react";
import {
  Plus,
  Users,
  MoreHorizontal,
  Edit,
  Lock,
  Search,
  Upload,
} from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";
import { SchoolBulkImportModal } from "@/components/admin/SchoolBulkImportModal";
import {
  useAdminSchools,
  useCreateSchool,
  useUpdateSchool,
  useChangeSchoolStatus,
  adminSchoolsKeys,
} from "@/lib/hooks/admin-schools";
import type { School } from "@/lib/types";

type SchoolStatus = "active" | "deactivated";

interface SchoolForm {
  name: string;
  code: string;
  npsn: string;
  school_types: string;
  alamat: string;
}

const EMPTY_FORM: SchoolForm = {
  name: "",
  code: "",
  npsn: "",
  school_types: "",
  alamat: "",
};

// Search is sent to the server (q param), so it must be debounced the same
// way OrdersToolbar debounces order search — otherwise every keystroke fires
// a new paginated request and resets the accumulated list.
const SEARCH_DEBOUNCE_MS = 300;

export default function SystemSchoolsPage() {
  const { t, lang } = useTranslation();
  const numberLocale = lang === "en" ? "en-US" : "id-ID";
  // searchInput is the raw input value; debouncedSearch is what actually goes
  // to the server (q param) and into filterKey below, so pagination doesn't
  // reset and refetch on every keystroke.
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<SchoolStatus | "all">("all");
  const [createOpen, setCreateOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<School | null>(null);
  const [inspectedSchoolId, setInspectedSchoolId] = useState<string>("");
  const queryClient = useQueryClient();
  const [createForm, setCreateForm] = useState<SchoolForm>({ ...EMPTY_FORM });
  const [editForm, setEditForm] = useState<SchoolForm>({ ...EMPTY_FORM });

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(searchInput.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [searchInput]);

  // Cursor-based pagination
  const [schools, setSchools] = useState<School[]>([]);
  const [fetchCursor, setFetchCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  // Stats mirror the server's filter-aware counts (Service.AdminListSchools),
  // not schools.length — that would only ever count the rows loaded so far,
  // which was the "Total ≈ 20" bug reported in
  // docs/backlog/school-bulk-list-pagination.md. Held in state (rather than
  // read straight off the query) so it doesn't flash to 0 between pages.
  const [stats, setStats] = useState({ total: 0, active: 0, students: 0 });

  // q/status are now server-side filters, so a filter change must restart
  // pagination at page 1 — otherwise "load more" would keep appending pages
  // fetched under the *previous* filter. Same guard as the students page.
  const filterKey = `${statusFilter}:${debouncedSearch}`;
  const pageFilterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== pageFilterKeyRef.current) {
      setSchools([]);
      setFetchCursor(undefined);
      setNextCursor(undefined);
      pageFilterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const { data, isLoading, error } = useAdminSchools({
    q: debouncedSearch || undefined,
    status: statusFilter === "all" ? undefined : statusFilter,
    cursor: fetchCursor,
    limit: 20,
  });
  const createSchool = useCreateSchool();
  const updateSchool = useUpdateSchool();
  const changeStatus = useChangeSchoolStatus();

  useEffect(() => {
    if (!data) return;
    if (filterKey !== pageFilterKeyRef.current) return;

    if (fetchCursor === undefined) {
      setSchools(data.data);
    } else {
      setSchools((prev) => {
        const ids = new Set(prev.map((s) => s.id));
        const fresh = data.data.filter((s) => !ids.has(s.id));
        return [...prev, ...fresh];
      });
    }
    setNextCursor(data.next_cursor);
    setStats({ total: data.total, active: data.active, students: data.students });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data]);

  const rows = schools;
  const inspectedSchool =
    schools.find((school) => school.id === inspectedSchoolId) ?? schools[0] ?? null;
  const inspectorLabel = lang === "en" ? "School details" : "Detail sekolah";

  function resetPagination() {
    setSchools([]);
    setFetchCursor(undefined);
    setNextCursor(undefined);
  }

  const handleCreate = async () => {
    if (!createForm.name || !createForm.code) {
      toast.error(t("accounts_toast_required"));
      return;
    }
    try {
      await createSchool.mutateAsync({
        name: createForm.name,
        code: createForm.code,
        npsn: createForm.npsn || undefined,
        school_types: createForm.school_types
          ? createForm.school_types
              .split(",")
              .map((s) => s.trim())
              .filter(Boolean)
          : undefined,
        alamat: createForm.alamat || undefined,
      });
      toast.success(t("changes_saved"));
      setCreateOpen(false);
      setCreateForm({ ...EMPTY_FORM });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  const handleEditOpen = (school: School) => {
    setEditTarget(school);
    setEditForm({
      name: school.name,
      code: school.code ?? "",
      npsn: school.npsn ?? "",
      school_types: (school.school_types ?? []).join(", "),
      alamat: school.alamat ?? "",
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    try {
      const payload: Record<string, unknown> = {};

      if (editForm.name !== editTarget.name) payload.name = editForm.name;
      if (editForm.code !== (editTarget.code ?? "")) payload.code = editForm.code;
      if (editForm.npsn !== (editTarget.npsn ?? "")) payload.npsn = editForm.npsn;

      const types = editForm.school_types
        ? editForm.school_types
            .split(",")
            .map((s) => s.trim())
            .filter(Boolean)
        : [];
      if (JSON.stringify(types) !== JSON.stringify(editTarget.school_types ?? []))
        payload.school_types = types;

      if (editForm.alamat !== (editTarget.alamat ?? ""))
        payload.alamat = editForm.alamat || undefined;

      if (Object.keys(payload).length === 0) {
        setEditTarget(null);
        return;
      }

      await updateSchool.mutateAsync({ id: editTarget.id, ...payload });
      toast.success(t("changes_saved"));
      setEditTarget(null);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  const handleStatusToggle = async (school: School) => {
    const newStatus: SchoolStatus =
      school.status === "active" ? "deactivated" : "active";
    try {
      await changeStatus.mutateAsync({ id: school.id, status: newStatus });
      toast.success(
        newStatus === "active"
          ? t("accounts_toast_activated")
          : t("accounts_toast_deactivated"),
      );
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("sys_save_failed");
      toast.error(msg);
    }
  };

  const handleLoadMore = () => {
    if (nextCursor) setFetchCursor(nextCursor);
  };

  // Loading/error states render inside the table instead of replacing the
  // whole page — an early return would unmount the search input mid-search,
  // dropping its focus every time results refresh (same shape as
  // ParticipantPicker, which keeps its toolbar mounted).
  const tableEmpty =
    error && schools.length === 0
      ? t("sys_error_load")
      : isLoading
        ? t("sys_loading_data")
        : lang === "en"
          ? "No schools found."
          : "Tidak ada sekolah ditemukan.";

  return (
    <div className="mx-auto max-w-[1400px] px-4 py-7 md:px-6 md:py-9 fade-in">
      <header className="mb-7 flex flex-col gap-6 border-b border-line pb-7 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-4xl font-bold tracking-[-0.045em] text-ink-900 md:text-5xl">
            {t("schools_title")}
          </h1>
          <p className="mt-3 max-w-xl text-sm leading-6 text-ink-600">
            {lang === "en"
              ? "Find a registry record, check its identity, then manage the school without leaving the index."
              : "Cari data sekolah, periksa identitasnya, lalu kelola sekolah tanpa meninggalkan daftar."}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button className="rounded-md" variant="outline" onClick={() => setBulkOpen(true)}>
            <Upload className="mr-1 size-4" />
            {t("bulk_school_import_button")}
          </Button>
          <Button className="rounded-md" onClick={() => setCreateOpen(true)}>
            <Plus className="mr-1 size-4" />
            {t("create")}
          </Button>
        </div>
      </header>

      <div className="border border-line bg-surface">
        <div className="flex flex-col gap-3 border-b border-line bg-surface-2 p-4 lg:flex-row lg:items-center">
          <div className="relative min-w-0 flex-1">
            <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
            <Input
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder={t("schools_search_placeholder")}
              className="h-10 rounded-sm bg-surface pl-9 text-xs"
            />
          </div>
          <div className="flex flex-wrap gap-2">
          <FilterChip
            active={statusFilter === "all"}
            onClick={() => setStatusFilter("all")}
          >
            {t("tab_all")}
          </FilterChip>
          <FilterChip
            active={statusFilter === "active"}
            onClick={() => setStatusFilter("active")}
          >
            {t("status_label_active")}
          </FilterChip>
          <FilterChip
            active={statusFilter === "deactivated"}
            onClick={() => setStatusFilter("deactivated")}
          >
            {t("status_label_inactive")}
          </FilterChip>
          </div>
        </div>

        <div className="grid xl:grid-cols-[minmax(0,1fr)_20rem]">
          <section className="min-w-0 px-5 py-6 md:px-7">
            <div className="mb-4 flex flex-col gap-3 border-t-4 border-t-ink-900 pt-4 sm:flex-row sm:items-end sm:justify-between">
              <div>
                <h2 className="text-2xl font-bold tracking-[-0.035em] text-ink-900">
                  {stats.total.toLocaleString(numberLocale)} {lang === "en" ? "matching schools" : "sekolah ditemukan"}
                </h2>
                <p className="mt-1 text-xs text-ink-500">
                  {lang === "en" ? "Sorted by school name" : "Diurutkan berdasarkan nama sekolah"}
                </p>
              </div>
              <div className="flex gap-5 text-xs text-ink-500">
                <span><strong className="text-base text-ink-900">{stats.active}</strong> {t("status_label_active")}</span>
                <span><strong className="text-base text-ink-900">{stats.students.toLocaleString(numberLocale)}</strong> {t("schools_stat_students")}</span>
              </div>
            </div>

            <div className="border-b border-line">
              {rows.length === 0 && (
                <div className="border-t border-line px-4 py-10 text-center text-sm text-ink-500">
                  {tableEmpty}
                </div>
              )}
              {rows.map((s) => {
                const selected = inspectedSchool?.id === s.id;
                return (
                  <article
                    key={s.id}
                    className={cn(
                      "grid min-h-20 grid-cols-[5.5rem_minmax(0,1fr)_auto_2rem] items-center gap-3 border-t border-line px-2 py-3 md:grid-cols-[6rem_minmax(11rem,1.4fr)_minmax(8rem,1fr)_5rem_5rem_2rem]",
                      selected && "-ml-2 border-l-4 border-l-brand-600 bg-surface-2 pl-3",
                    )}
                  >
                    <span className="text-xs font-bold tracking-[0.04em] text-brand-700">
                      {s.npsn ?? "—"}
                    </span>
                    <button
                      type="button"
                      aria-label={lang === "en" ? `View details for ${s.name}` : `Lihat detail ${s.name}`}
                      className="min-w-0 text-left"
                      onClick={() => setInspectedSchoolId(s.id)}
                    >
                      <span className="block truncate text-sm font-semibold text-ink-900">{s.name}</span>
                      <span className="mt-1 block truncate text-[11px] text-ink-500">{s.code ?? "—"}</span>
                    </button>
                    <span className="hidden truncate text-xs leading-5 text-ink-500 md:block">{s.alamat ?? "—"}</span>
                    <span className="hidden text-center md:block">
                      {(s.school_types ?? []).slice(0, 1).map((type) => (
                        <Badge key={type} variant="outline" className="rounded-sm bg-surface text-[10px]">{type}</Badge>
                      ))}
                    </span>
                    <span className="hidden items-center gap-1 text-xs text-ink-500 md:inline-flex">
                      <Users className="size-3" />
                      {(s.student_count ?? 0).toLocaleString(numberLocale)}
                    </span>
                    <div className="flex items-center justify-end gap-2">
                      <span className={cn("size-2 rounded-full", s.status === "active" ? "bg-success" : "bg-ink-400")} aria-label={s.status === "active" ? t("status_label_active") : t("status_label_inactive")} />
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon-xs">
                          <MoreHorizontal className="size-4 text-ink-500" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          onClick={() => handleEditOpen(s)}
                        >
                          <Edit className="mr-2 size-4" />
                          {t("schools_action_edit")}
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          onClick={() => handleStatusToggle(s)}
                        >
                          <Lock className="mr-2 size-4" />
                          {s.status === "active"
                            ? t("accounts_action_deactivate")
                            : t("accounts_action_activate")}
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                    </div>
                  </article>
                );
              })}
            </div>

            {nextCursor && (
              <div className="mt-4 text-center">
                <Button variant="outline" size="sm" className="rounded-sm" onClick={handleLoadMore} disabled={isLoading}>
                  {isLoading ? t("sys_loading") : lang === "en" ? "Load more" : "Muat lebih banyak"}
                </Button>
              </div>
            )}
          </section>

          <aside
            aria-label={inspectorLabel}
            className="border-t border-line bg-surface-2 p-6 xl:border-l xl:border-t-0"
          >
            {inspectedSchool ? (
              <>
                <div className="border-t-4 border-brand-600 bg-surface p-5">
                  <span className="text-xs font-bold tracking-[0.04em] text-brand-700">
                    {inspectedSchool.npsn ? `NPSN ${inspectedSchool.npsn}` : "NPSN —"}
                  </span>
                  <h2
                    aria-label={inspectedSchool.name}
                    className="mt-3 text-2xl font-bold leading-tight tracking-[-0.035em] text-ink-900"
                  >
                    {inspectedSchool.name.split(" ").map((part, index) => (
                      <span key={`${part}-${index}`}>{part} </span>
                    ))}
                  </h2>
                  <p className="mt-3 text-xs leading-5 text-ink-500">
                    {inspectedSchool.alamat ?? (lang === "en" ? "Address not provided" : "Alamat belum tersedia")}
                  </p>
                  <Button className="mt-5 rounded-md" onClick={() => handleEditOpen(inspectedSchool)}>
                    <Edit className="mr-2 size-4" />
                    {lang === "en" ? "Edit school record" : "Edit data sekolah"}
                  </Button>
                </div>
                <div className="mt-6 border-t border-line pt-5">
                  <h3 className="text-sm font-semibold text-ink-900">
                    {lang === "en" ? "School identity" : "Identitas sekolah"}
                  </h3>
                  {[
                    [t("accounts_th_status"), inspectedSchool.status === "active" ? t("status_label_active") : t("status_label_inactive")],
                    [t("schools_field_code"), inspectedSchool.code ? `#${inspectedSchool.code}` : "—"],
                    [t("schools_field_school_types"), inspectedSchool.school_types?.join(", ") || "—"],
                    [t("schools_field_student_count"), (inspectedSchool.student_count ?? 0).toLocaleString(numberLocale)],
                  ].map(([label, value]) => (
                    <div key={String(label)} className="mt-3 flex items-start justify-between gap-4 text-xs">
                      <span className="text-ink-500">{label}</span>
                      <strong className="max-w-[11rem] text-right text-ink-900">{value}</strong>
                    </div>
                  ))}
                </div>
              </>
            ) : (
              <p className="text-sm text-ink-500">
                {lang === "en" ? "Select a school to inspect its record." : "Pilih sekolah untuk melihat detailnya."}
              </p>
            )}
          </aside>
        </div>
      </div>

      <SchoolBulkImportModal
        open={bulkOpen}
        onOpenChange={setBulkOpen}
        onImportSuccess={() => {
          queryClient.invalidateQueries({ queryKey: adminSchoolsKeys.all });
          // A bulk job can land anywhere in name order, and the page may be
          // sitting on a "load more"'d cursor or a stale filtered view — reset
          // to page 1 under the current filters so the new rows are reachable
          // (docs/backlog/school-bulk-list-pagination.md, root cause #3).
          resetPagination();
        }}
      />

      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="font-serif">
              {t("schools_dialog_create_title")}
            </DialogTitle>
            <DialogDescription>
              {t("schools_dialog_create_desc")}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t("schools_field_name")}</Label>
              <Input
                value={createForm.name}
                onChange={(e) =>
                  setCreateForm((f) => ({ ...f, name: e.target.value }))}
                placeholder={t("schools_placeholder_name")}
              />
            </div>
            <div>
              <Label>{t("schools_field_code")}</Label>
              <Input
                value={createForm.code}
                onChange={(e) =>
                  setCreateForm((f) => ({ ...f, code: e.target.value }))}
                placeholder={t("schools_field_code")}
              />
            </div>
            <div>
              <Label>{t("schools_field_npsn")}</Label>
              <Input
                value={createForm.npsn}
                onChange={(e) =>
                  setCreateForm((f) => ({ ...f, npsn: e.target.value }))}
                placeholder={t("schools_placeholder_npsn")}
                maxLength={8}
              />
            </div>
            <div>
              <Label>{t("schools_field_school_types")}</Label>
              <Input
                value={createForm.school_types}
                onChange={(e) =>
                  setCreateForm((f) => ({ ...f, school_types: e.target.value }))}
                placeholder={t("schools_field_school_types")}
              />
            </div>
            <div>
              <Label>{t("schools_field_alamat")}</Label>
              <Input
                value={createForm.alamat}
                onChange={(e) =>
                  setCreateForm((f) => ({ ...f, alamat: e.target.value }))}
                placeholder={t("schools_field_alamat")}
              />
            </div>
          </div>
          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={createSchool.isPending}>
              {createSchool.isPending ? t("saving") : t("create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog
        open={editTarget !== null}
        onOpenChange={(open) => {
          if (!open) setEditTarget(null);
        }}
      >
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle className="font-serif">
              {t("schools_action_edit")}
            </DialogTitle>
            <DialogDescription>
              {editTarget
                ? (lang === "en"
                    ? `Edit school: ${editTarget.name}`
                    : `Edit sekolah: ${editTarget.name}`)
                : ""}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t("schools_field_name")}</Label>
              <Input
                value={editForm.name}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, name: e.target.value }))}
                placeholder={t("schools_placeholder_name")}
              />
            </div>
            <div>
              <Label>{t("schools_field_code")}</Label>
              <Input
                value={editForm.code}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, code: e.target.value }))}
                placeholder={t("schools_field_code")}
              />
            </div>
            <div>
              <Label>{t("schools_field_npsn")}</Label>
              <Input
                value={editForm.npsn}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, npsn: e.target.value }))}
                placeholder={t("schools_placeholder_npsn")}
                maxLength={8}
              />
            </div>
            <div>
              <Label>{t("schools_field_school_types")}</Label>
              <Input
                value={editForm.school_types}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, school_types: e.target.value }))}
                placeholder={t("schools_field_school_types")}
              />
            </div>
            <div>
              <Label>{t("schools_field_alamat")}</Label>
              <Input
                value={editForm.alamat}
                onChange={(e) =>
                  setEditForm((f) => ({ ...f, alamat: e.target.value }))}
                placeholder={t("schools_field_alamat")}
              />
            </div>
          </div>
          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setEditTarget(null)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleEdit} disabled={updateSchool.isPending}>
              {updateSchool.isPending ? t("saving") : t("save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function FilterChip({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        "rounded-lg border px-3 py-[7px] text-xs font-semibold transition-colors",
        active
          ? "border-brand-600 bg-brand-600 text-white"
          : "border-line bg-surface text-ink-600 hover:text-ink-900",
      )}
    >
      {children}
    </button>
  );
}
