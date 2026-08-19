"use client";

import { useState, useEffect, useRef } from "react";
import {
  Plus,
  Building,
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
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { StatCard } from "@/components/admin/StatCard";
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

const STATUS_TONE: Record<SchoolStatus, string> = {
  active: "bg-success-bg text-success border-success",
  deactivated: "bg-danger-bg text-danger border-danger",
};

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
      if (editForm.npsn !== (editTarget.npsn ?? "")) payload.npsn = editForm.npsn || undefined;

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

  if (isLoading && schools.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={Building}
          title={t("schools_title")}
          description={t("sys_loading")}
        />
        <div className="py-12 text-center text-ink-500">
          {t("sys_loading_data")}
        </div>
      </div>
    );
  }

  if (error && schools.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader
          icon={Building}
          title={t("schools_title")}
          description={t("sys_error_title")}
        />
        <div className="py-12 text-center text-ink-500">
          {t("sys_error_load")}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
      <AdminPageHeader
        icon={Building}
        title={t("schools_title")}
        description={t("schools_subtitle")}
        actions={
          <div className="flex gap-2">
            <Button size="sm" variant="outline" onClick={() => setBulkOpen(true)}>
              <Upload className="mr-1 size-4" />
              {t("bulk_school_import_button")}
            </Button>
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="mr-1 size-4" />
              {t("create")}
            </Button>
          </div>
        }
      />

      <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <StatCard label={t("schools_stat_total")} value={String(stats.total)} />
        <StatCard
          label={t("status_label_active")}
          value={String(stats.active)}
        />
        <StatCard
          label={t("schools_stat_students")}
          value={stats.students.toLocaleString(numberLocale)}
        />
      </div>

      <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center">
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
        <div className="flex items-center gap-2 lg:ml-auto">
          <Search className="size-4 text-ink-400" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("schools_search_placeholder")}
            className="h-9 w-[200px] text-xs"
          />
        </div>
      </div>

      <div className="md-card-outlined">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead className="bg-surface-2 text-left text-xs font-semibold text-ink-600">
              <tr>
                <th className="px-4 py-3">{t("schools_th_school")}</th>
                <th className="px-4 py-3">{t("schools_field_code")}</th>
                <th className="px-4 py-3">NPSN</th>
                <th className="px-4 py-3">{t("schools_field_school_types")}</th>
                <th className="px-4 py-3">{t("schools_field_alamat")}</th>
                <th className="px-4 py-3">{t("accounts_th_status")}</th>
                <th className="px-4 py-3">{t("schools_field_student_count")}</th>
                <th className="px-4 py-3 text-right" />
              </tr>
            </thead>
            <tbody className="divide-y divide-line">
              {rows.length === 0 && (
                <tr>
                  <td
                    colSpan={8}
                    className="px-4 py-8 text-center text-sm text-ink-500"
                  >
                    {lang === "en" ? "No schools found." : "Tidak ada sekolah ditemukan."}
                  </td>
                </tr>
              )}
              {rows.map((s) => (
                <tr key={s.id} className="group hover:bg-surface-2">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2 font-medium text-ink-900">
                      <Building className="size-4 shrink-0 text-brand-600" />
                      <span className="max-w-[160px] truncate">{s.name}</span>
                    </div>
                    <div className="font-mono text-[11px] text-ink-500">
                      {s.id}
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-xs text-ink-700">
                      {s.code ?? "—"}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink-600">
                    {s.npsn ?? "—"}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex flex-wrap gap-1">
                      {s.school_types && s.school_types.length > 0
                        ? s.school_types.map((st) => (
                            <Badge
                              key={st}
                              variant="outline"
                              className="bg-surface-2 text-[11px] text-ink-700"
                            >
                              {st}
                            </Badge>
                          ))
                        : (
                            <span className="text-xs text-ink-400">—</span>
                          )}
                    </div>
                  </td>
                  <td className="max-w-[160px] truncate px-4 py-3 text-xs text-ink-600">
                    {s.alamat ?? "—"}
                  </td>
                  <td className="px-4 py-3">
                    <Badge
                      variant="outline"
                      className={cn(
                        "text-[11px] font-semibold capitalize",
                        STATUS_TONE[s.status as SchoolStatus],
                      )}
                    >
                      {s.status === "active"
                        ? t("status_label_active")
                        : t("status_label_inactive")}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink-600">
                    <span className="inline-flex items-center gap-1">
                      <Users className="size-3" />
                      {(s.student_count ?? 0).toLocaleString(numberLocale)}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
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
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {nextCursor && (
        <div className="mt-4 text-center">
          <Button
            variant="outline"
            size="sm"
            onClick={handleLoadMore}
            disabled={isLoading}
          >
            {isLoading
              ? t("sys_loading")
              : lang === "en"
              ? "Load more"
              : "Muat lebih banyak"}
          </Button>
        </div>
      )}

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
                placeholder="NPSN"
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
                placeholder="NPSN"
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
