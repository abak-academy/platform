"use client";

import { useState, useEffect, useMemo, useRef } from "react";
import {
  Plus,
  MoreHorizontal,
  Edit,
  Lock,
  School as SchoolIcon,
  Upload,
} from "lucide-react";
import { toast } from "sonner";
import { useQueryClient } from "@tanstack/react-query";
import { useTranslation } from "@/lib/i18n";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { DataTable, TableItem, type DataTableColumn } from "@/components/ui/data-table";
import { PageHead } from "@/components/shell/PageHead";
import {
  ActiveChip,
  FILTER_PANEL_CLASS,
  FilterGrid,
  FilterPanelFooter,
  FilterSelect,
  ToolbarCount,
  ToolbarFilterButton,
  ToolbarSearch,
  ToolbarSort,
} from "@/components/admin/ListToolbar";
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
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
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
  provinsi_id: string;
  kota_id: string;
}

const EMPTY_FORM: SchoolForm = {
  name: "",
  code: "",
  npsn: "",
  school_types: "",
  alamat: "",
  provinsi_id: "",
  kota_id: "",
};


// Search is sent to the server (q param), so it must be debounced the same
// way OrdersToolbar debounces order search — otherwise every keystroke fires
// a new paginated request and resets the accumulated list.

export default function SystemSchoolsPage() {
  const { t, lang } = useTranslation();
  const numberLocale = lang === "en" ? "en-US" : "id-ID";
  // searchInput is the raw input value; debouncedSearch is what actually goes
  // to the server (q param) and into filterKey below, so pagination doesn't
  // reset and refetch on every keystroke.
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput).trim();
  const [statusFilter, setStatusFilter] = useState<SchoolStatus | "all">("all");
  const [provinceFilter, setProvinceFilter] = useState("");
  const [sort, setSort] = useState("default");
  const [createOpen, setCreateOpen] = useState(false);
  const [bulkOpen, setBulkOpen] = useState(false);
  const [editTarget, setEditTarget] = useState<School | null>(null);
  const queryClient = useQueryClient();
  const [createForm, setCreateForm] = useState<SchoolForm>({ ...EMPTY_FORM });
  const [editForm, setEditForm] = useState<SchoolForm>({ ...EMPTY_FORM });
  const { data: provinces } = useProvinces();
  const { data: createCities } = useCitiesByProvince(createForm.provinsi_id);
  const { data: editCities } = useCitiesByProvince(editForm.provinsi_id);

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

  // Province filter and sorts only see loaded rows; sort is locked until every page is in.
  const partial = !!nextCursor;
  const clientFiltered = !!provinceFilter;
  const loadedOnly = partial && (clientFiltered || sort !== "default");
  const rows = useMemo(() => {
    const filtered = provinceFilter ? schools.filter((s) => s.provinsi_id === provinceFilter) : schools;
    if (sort === "students") return [...filtered].sort((a, b) => (b.student_count ?? 0) - (a.student_count ?? 0));
    if (sort === "name") return [...filtered].sort((a, b) => a.name.localeCompare(b.name));
    return filtered;
  }, [schools, provinceFilter, sort]);

  const statusOptions = [
    { value: "active", label: t("status_label_active") },
    { value: "deactivated", label: t("status_label_inactive") },
  ];
  const provinceOptions = (provinces ?? []).map((p) => ({ value: String(p.id), label: p.name }));
  const activeChips = [
    ...(statusFilter !== "all"
      ? [{ key: "status", label: `${t("accounts_th_status")}: ${statusOptions.find((o) => o.value === statusFilter)?.label}`, clear: () => setStatusFilter("all") }]
      : []),
    ...(provinceFilter
      ? [{ key: "prov", label: `${t("schools_filter_province")}: ${provinceOptions.find((o) => o.value === provinceFilter)?.label ?? provinceFilter}`, clear: () => setProvinceFilter("") }]
      : []),
  ];
  const resetFilters = () => {
    setStatusFilter("all");
    setProvinceFilter("");
  };
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
        : partial && clientFiltered
          ? (
            <div className="flex flex-col items-center gap-2 py-6">
              <p>{t("filter_empty_loaded")}</p>
              <Button variant="outline" size="sm" onClick={handleLoadMore}>{t("adm_load_more")}</Button>
            </div>
          )
          : t("sys_no_schools_found");

  const columns: DataTableColumn<School>[] = [
    {
      key: "school",
      header: t("schools_field_name"),
      cell: (school) => (
        <TableItem
          thumb={
            <span className="grid size-[38px] place-items-center rounded-xl bg-sky-50 text-sky-700">
              <SchoolIcon className="size-5" aria-hidden />
            </span>
          }
          title={school.name}
          sub={school.code || "—"}
        />
      ),
    },
    {
      key: "npsn",
      header: t("schools_field_npsn"),
      className: "font-mono text-xs",
      cell: (school) => school.npsn || "—",
    },
    {
      key: "address",
      header: t("schools_field_alamat"),
      className: "max-w-xs truncate",
      cell: (school) => <span title={school.alamat || undefined}>{school.alamat || "—"}</span>,
    },
    {
      key: "type",
      header: t("schools_field_school_types"),
      cell: (school) =>
        school.school_types?.length ? (
          <span>{school.school_types.join(" / ")}</span>
        ) : (
          "—"
        ),
    },
    {
      key: "students",
      header: t("schools_field_student_count"),
      align: "right",
      cell: (school) => (school.student_count ?? 0).toLocaleString(numberLocale),
    },
    {
      key: "status",
      header: t("accounts_th_status"),
      cell: (school) => (
        <Badge variant={school.status === "active" ? "teal" : "danger"}>
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
    <div className="flex flex-col gap-4 fade-in">
      <PageHead
        title={t("schools_title")}
        sub={`${t("adm_schools_count").replace("{n}", stats.total.toLocaleString(numberLocale))} · ${stats.active.toLocaleString(numberLocale)} ${t("status_label_active")} · ${stats.students.toLocaleString(numberLocale)} ${t("schools_stat_students")}`}
        actions={
          <>
            <Button variant="outline" onClick={() => setBulkOpen(true)}>
              <Upload className="size-4" />
              {t("bulk_school_import_button")}
            </Button>
            <Button variant="dark" onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              {t("create")}
            </Button>
          </>
        }
      />

      <section aria-label={t("adm_school_list")} className="md-card-flush">
        <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
          <ToolbarSearch
            value={searchInput}
            onChange={setSearchInput}
            placeholder={t("schools_search_placeholder")}
            clearLabel={t("list_toolbar_clear_search")}
          />
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarFilterButton label={t("list_toolbar_filter")} count={activeChips.length} />
            </PopoverTrigger>
            <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("list_toolbar_filter")}>
              <div className="flex flex-col gap-3.5">
                <FilterGrid>
                  <FilterSelect
                    label={t("accounts_th_status")}
                    value={statusFilter === "all" ? "" : statusFilter}
                    options={statusOptions}
                    onChange={(v) => setStatusFilter((v || "all") as SchoolStatus | "all")}
                  />
                  <FilterSelect label={t("schools_filter_province")} value={provinceFilter} options={provinceOptions} onChange={setProvinceFilter} />
                </FilterGrid>
                <FilterPanelFooter
                  onReset={resetFilters}
                  showLabel={t("list_toolbar_show_n").replace("{n}", `${rows.length}${loadedOnly ? "+" : ""}`)}
                />
              </div>
            </PopoverContent>
          </Popover>
          <ToolbarSort
            value={sort}
            onChange={setSort}
            label={t("list_toolbar_sort")}
            options={[
              { value: "default", label: t("filter_sort_default") },
              { value: "students", label: t("filter_sort_students"), disabled: partial },
              { value: "name", label: t("filter_sort_name"), disabled: partial },
            ]}
          />
          <ToolbarCount>
            {loadedOnly
              ? t("adm_schools_count").replace("{n}", `${rows.length}+`)
              : t("adm_schools_count").replace("{n}", stats.total.toLocaleString(numberLocale))}
          </ToolbarCount>
        </div>
        {partial && (
          <p className="border-b border-line px-4 py-2 text-xs text-ink-600">{t("filter_sort_locked")}</p>
        )}
        {activeChips.length > 0 && (
          <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5" role="group" aria-label={t("list_toolbar_active")}>
            {activeChips.map((c) => (
              <ActiveChip key={c.key} label={c.label} removeLabel={`${t("list_toolbar_remove")}: ${c.label}`} onRemove={c.clear} />
            ))}
            <Button variant="ghost" size="sm" onClick={resetFilters}>{t("list_toolbar_clear_all")}</Button>
          </div>
        )}

        <DataTable
          columns={columns}
          rows={rows}
          rowKey={(school) => school.id}
          empty={tableEmpty}
          surface="plain"
          onRowClick={handleEditOpen}
          data-testid="schools-table"
          footer={
            nextCursor ? (
              <div className="border-t border-line px-4 py-3 text-center">
                <Button variant="outline" size="sm" onClick={handleLoadMore} disabled={isLoading}>
                  {isLoading ? t("sys_loading") : t("adm_load_more")}
                </Button>
              </div>
            ) : undefined
          }
        />
      </section>

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
        <DialogContent className="max-w-[480px]">
          <DialogHeader>
            <DialogTitle>
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
                  className="mt-2 h-[46px] w-full rounded-[14px] border-[1.5px] border-line bg-surface px-3.5 text-sm text-ink-900 outline-none focus:border-brand-600 focus:ring-4 focus:ring-brand-50"
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
                  className="mt-2 h-[46px] w-full rounded-[14px] border-[1.5px] border-line bg-surface px-3.5 text-sm text-ink-900 outline-none focus:border-brand-600 focus:ring-4 focus:ring-brand-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="">{t("school_picker_city")}</option>
                  {(createCities ?? []).map((city) => (
                    <option key={city.id} value={city.id}>{city.name}</option>
                  ))}
                </select>
              </div>
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
        <DialogContent className="max-w-[480px]">
          <DialogHeader>
            <DialogTitle>
              {t("schools_action_edit")}
            </DialogTitle>
            <DialogDescription>
              {editTarget
                ? t("sys_edit_school_desc").replace("{name}", editTarget.name)
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
                  className="mt-2 h-[46px] w-full rounded-[14px] border-[1.5px] border-line bg-surface px-3.5 text-sm text-ink-900 outline-none focus:border-brand-600 focus:ring-4 focus:ring-brand-50"
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
                  className="mt-2 h-[46px] w-full rounded-[14px] border-[1.5px] border-line bg-surface px-3.5 text-sm text-ink-900 outline-none focus:border-brand-600 focus:ring-4 focus:ring-brand-50 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  <option value="">{t("school_picker_city")}</option>
                  {(editCities ?? []).map((city) => (
                    <option key={city.id} value={city.id}>{city.name}</option>
                  ))}
                </select>
              </div>
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
              <textarea
                rows={3}
                className="w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
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
