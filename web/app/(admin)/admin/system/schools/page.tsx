"use client";

import { useState, useEffect, useRef } from "react";
import {
  Plus,
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
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
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
import { useCitiesByProvince, useProvinces } from "@/lib/hooks/regions";
import type { School } from "@/lib/types";

type SchoolStatus = "active" | "deactivated";

interface SchoolForm {
  name: string;
  code: string;
  npsn: string;
  school_types: string;
  alamat: string;
  category: string;
  provinsi_id: string;
  kota_id: string;
}

const EMPTY_FORM: SchoolForm = {
  name: "",
  code: "",
  npsn: "",
  school_types: "",
  alamat: "",
  category: "",
  provinsi_id: "",
  kota_id: "",
};

const SCHOOL_CATEGORIES = ["SD", "MI", "SMP", "MTS", "SMA", "MA", "SMK"];

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
  const [inspectedSchoolId, setInspectedSchoolId] = useState("");
  const queryClient = useQueryClient();
  const [createForm, setCreateForm] = useState<SchoolForm>({ ...EMPTY_FORM });
  const [editForm, setEditForm] = useState<SchoolForm>({ ...EMPTY_FORM });
  const { data: provinces } = useProvinces();
  const { data: createCities } = useCitiesByProvince(createForm.provinsi_id);
  const { data: editCities } = useCitiesByProvince(editForm.provinsi_id);

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
    if (!createForm.name || !createForm.code || Boolean(createForm.provinsi_id) !== Boolean(createForm.kota_id)) {
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
        category: createForm.category || undefined,
        provinsi_id: createForm.provinsi_id || undefined,
        kota_id: createForm.kota_id || undefined,
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
      category: school.category ?? "",
      provinsi_id: school.provinsi_id ?? "",
      kota_id: school.kota_id ?? "",
    });
  };

  const handleEdit = async () => {
    if (!editTarget) return;
    if (Boolean(editForm.provinsi_id) !== Boolean(editForm.kota_id)) {
      toast.error(t("accounts_toast_required"));
      return;
    }
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
      if (editForm.category !== (editTarget.category ?? ""))
        payload.category = editForm.category;
      if (editForm.provinsi_id !== (editTarget.provinsi_id ?? ""))
        payload.provinsi_id = editForm.provinsi_id;
      if (editForm.kota_id !== (editTarget.kota_id ?? ""))
        payload.kota_id = editForm.kota_id;

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

  const columns: DataTableColumn<School>[] = [
    {
      key: "school",
      header: t("schools_field_name"),
      cell: (school) => (
        <button
          type="button"
          aria-label={lang === "en" ? `View details for ${school.name}` : `Lihat detail ${school.name}`}
          className="min-w-0 text-left"
          onClick={() => setInspectedSchoolId(school.id)}
        >
          <span className={cn(
            "block font-medium",
            inspectedSchool?.id === school.id ? "text-brand-700" : "text-ink-900",
          )}>
            {school.name}
          </span>
          <span className="mt-1 block font-mono text-xs text-brand-700">{school.code || "—"}</span>
        </button>
      ),
    },
    {
      key: "npsn",
      header: t("schools_field_npsn"),
      className: "font-mono text-xs text-ink-600",
      cell: (school) => school.npsn || "—",
    },
    {
      key: "address",
      header: t("schools_field_alamat"),
      className: "max-w-xs text-xs text-ink-600",
      cell: (school) => school.alamat || "—",
    },
    {
      key: "type",
      header: t("schools_field_school_types"),
      cell: (school) =>
        school.school_types?.length ? (
          <span className="text-xs text-ink-600">{school.school_types.join(" / ")}</span>
        ) : (
          "—"
        ),
    },
    {
      key: "students",
      header: t("schools_field_student_count"),
      className: "text-xs text-ink-600",
      cell: (school) => (school.student_count ?? 0).toLocaleString(numberLocale),
    },
    {
      key: "status",
      header: t("accounts_th_status"),
      cell: (school) => (
        <Badge
          variant="outline"
          className={cn(
            "text-[11px] font-semibold",
            school.status === "active"
              ? "border-success bg-success-bg text-success"
              : "border-danger bg-danger-bg text-danger",
          )}
        >
          {school.status === "active" ? t("status_label_active") : t("status_label_inactive")}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (school) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-xs" className="rounded-full">
              <MoreHorizontal className="size-4 text-ink-500" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => handleEditOpen(school)}>
              <Edit className="mr-2 size-4" />
              {t("schools_action_edit")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleStatusToggle(school)}>
              <Lock className="mr-2 size-4" />
              {school.status === "active"
                ? t("accounts_action_deactivate")
                : t("accounts_action_activate")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  return (
    <div className="space-y-6 fade-in">
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

      <div className="school-management-workspace overflow-hidden rounded-[20px] border border-line bg-surface shadow-[var(--md-sys-elevation-1)]">
        <div className="flex flex-col gap-3 border-b border-line bg-surface p-4 lg:flex-row lg:items-center">
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
          <section
            aria-label={lang === "en" ? "School list" : "Daftar sekolah"}
            className="min-w-0 px-5 py-6 md:px-7"
          >
            <div className="mb-4 flex flex-col gap-3 border-b border-line pb-4 sm:flex-row sm:items-end sm:justify-between">
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

            <div className="border-y border-line">
              <DataTable
                columns={columns}
                rows={rows}
                rowKey={(school) => school.id}
                empty={tableEmpty}
                surface="plain"
                data-testid="schools-table"
              />
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
            className="border-t border-line bg-surface p-6 xl:border-l xl:border-t-0"
          >
            {inspectedSchool ? (
              <>
                <div className="rounded-[16px] border border-line bg-surface p-5">
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
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="create-school-province">{t("school_picker_province")}</Label>
                <select
                  id="create-school-province"
                  data-testid="create-school-province"
                  value={createForm.provinsi_id}
                  onChange={(e) =>
                    setCreateForm((f) => ({ ...f, provinsi_id: e.target.value, kota_id: "" }))}
                  className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50"
                >
                  <option value="">{t("school_picker_province")}</option>
                  {(provinces ?? []).map((province) => (
                    <option key={province.id} value={province.id}>{province.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <Label htmlFor="create-school-city">{t("school_picker_city")}</Label>
                <select
                  id="create-school-city"
                  data-testid="create-school-city"
                  value={createForm.kota_id}
                  onChange={(e) => setCreateForm((f) => ({ ...f, kota_id: e.target.value }))}
                  disabled={!createForm.provinsi_id}
                  className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="">{t("school_picker_city")}</option>
                  {(createCities ?? []).map((city) => (
                    <option key={city.id} value={city.id}>{city.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <div>
              <Label htmlFor="create-school-category">{t("school_picker_category")}</Label>
              <select
                id="create-school-category"
                data-testid="create-school-category"
                value={createForm.category}
                onChange={(e) => setCreateForm((f) => ({ ...f, category: e.target.value }))}
                className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50"
              >
                <option value="">{t("school_picker_category")}</option>
                {SCHOOL_CATEGORIES.map((category) => (
                  <option key={category} value={category}>{category}</option>
                ))}
              </select>
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
            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="edit-school-province">{t("school_picker_province")}</Label>
                <select
                  id="edit-school-province"
                  data-testid="edit-school-province"
                  value={editForm.provinsi_id}
                  onChange={(e) =>
                    setEditForm((f) => ({ ...f, provinsi_id: e.target.value, kota_id: "" }))}
                  className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50"
                >
                  <option value="">{t("school_picker_province")}</option>
                  {(provinces ?? []).map((province) => (
                    <option key={province.id} value={province.id}>{province.name}</option>
                  ))}
                </select>
              </div>
              <div>
                <Label htmlFor="edit-school-city">{t("school_picker_city")}</Label>
                <select
                  id="edit-school-city"
                  data-testid="edit-school-city"
                  value={editForm.kota_id}
                  onChange={(e) => setEditForm((f) => ({ ...f, kota_id: e.target.value }))}
                  disabled={!editForm.provinsi_id}
                  className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="">{t("school_picker_city")}</option>
                  {(editCities ?? []).map((city) => (
                    <option key={city.id} value={city.id}>{city.name}</option>
                  ))}
                </select>
              </div>
            </div>
            <div>
              <Label htmlFor="edit-school-category">{t("school_picker_category")}</Label>
              <select
                id="edit-school-category"
                data-testid="edit-school-category"
                value={editForm.category}
                onChange={(e) => setEditForm((f) => ({ ...f, category: e.target.value }))}
                className="mt-2 h-9 w-full rounded-md border border-input bg-transparent px-3 text-sm outline-none focus:border-ring focus:ring-2 focus:ring-brand-300/50"
              >
                <option value="">{t("school_picker_category")}</option>
                {SCHOOL_CATEGORIES.map((category) => (
                  <option key={category} value={category}>{category}</option>
                ))}
              </select>
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
