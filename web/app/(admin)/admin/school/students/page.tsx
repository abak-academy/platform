"use client";

import { useEffect, useRef, useState } from "react";
import {
  Plus,
  MoreHorizontal,
  Lock,
  Search,
  Copy,
  Check,
  FileUp,
  UserRound,
  GraduationCap,
  MapPin,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { JENJANG_OPTIONS } from "@/lib/jenjang";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";
import { BulkImportModal } from "@/components/admin/BulkImportModal";
import { StatCard } from "@/components/admin/StatCard";
import {
  useAdminStudents,
  useRegisterStudent,
  useChangeStudentStatus,
  useReissueStudentCredentials,
} from "@/lib/hooks/admin-students";
import { useSchoolOptions } from "@/lib/hooks/admin-schools";
import { useSchools } from "@/lib/hooks/students";
import { useProvinces, useCitiesByProvince, useDistrictsByCity } from "@/lib/hooks/regions";
import { useAuthStore } from "@/stores/auth";
import type {
  AdminStudent,
  StudentRegistrationInput,
  StudentRegistrationResult,
  StudentCredentials,
} from "@/lib/types";

// Radix Select forbids an empty-string item value, so "every school" needs its
// own sentinel; it maps back to "" (no school_id param) for the query.
const ALL_SCHOOLS_VALUE = "_all_";

// Radix Select forbids an empty-string item value, so "no school" needs its own
// sentinel too; it maps back to "" (registered without a school).
const NO_SCHOOL_VALUE = "_none_";

// Search is sent to the server (q param), so it must be debounced the same
// way the schools page debounces school search — otherwise every keystroke
// fires a new paginated request and resets the accumulated list.
const SEARCH_DEBOUNCE_MS = 300;

function compactStudentRegistration(
  form: StudentRegistrationInput,
): StudentRegistrationInput {
  const payload: StudentRegistrationInput = {
    name: form.name,
    jenjang: form.jenjang,
  };
  const email = form.email?.trim();
  if (email) payload.email = email;
  const dob = form.dob?.trim();
  if (dob) payload.dob = dob;
  const gender = form.gender?.trim();
  if (gender) payload.gender = gender;
  if (form.grade != null) payload.grade = form.grade;
  const alamat = form.alamat_domisili?.trim();
  if (alamat) payload.alamat_domisili = alamat;
  const target = form.target_exam?.trim();
  if (target) payload.target_exam = target;
  if (form.provinsi_id) payload.provinsi_id = form.provinsi_id;
  if (form.kota_id) payload.kota_id = form.kota_id;
  if (form.kecamatan_id) payload.kecamatan_id = form.kecamatan_id;
  const kodePos = form.kode_pos?.trim();
  if (kodePos) payload.kode_pos = kodePos;
  return payload;
}

const STATUS_TONE: Record<string, string> = {
  active: "bg-success-bg text-success border-success",
  deactivated: "bg-danger-bg text-danger border-danger",
};

function initials(name: string) {
  return name
    .split(" ")
    .map((n) => n[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

export default function SchoolStudentsPage() {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  // Role-gated school picker (super_admin only)
  const currentRole = useAuthStore((s) => s.user?.role);
  const isSuperAdmin = currentRole === "super_admin";
  const { data: schoolsData, isLoading: schoolsLoading } = useSchoolOptions();
  const [selectedSchoolId, setSelectedSchoolId] = useState<string>("");

  // Filters
  // searchInput is the raw input value; debouncedSearch is what actually goes
  // to the server (q param) and into filterKey below, so pagination doesn't
  // reset and refetch on every keystroke.
  const [searchInput, setSearchInput] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState<string>("all");

  useEffect(() => {
    const id = setTimeout(() => setDebouncedSearch(searchInput.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [searchInput]);

  // Cursor pagination
  const [accumulated, setAccumulated] = useState<AdminStudent[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  // Stats mirror the server's filter-aware counts (Service.ListStudents →
  // CountStudentsAdmin), not accumulated.length — that would only ever count
  // the rows loaded so far, which was the "Total ≈ 20" bug previously fixed
  // on the schools page. Held in state (rather than read straight off the
  // query) so the numbers don't flash to 0 between pages.
  const [stats, setStats] = useState({ total: 0, active: 0, deactivated: 0 });

  // Guard: reset pagination on filter change
  const filterKey = `${statusFilter}:${debouncedSearch}:${selectedSchoolId}`;
  const pageFilterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== pageFilterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      pageFilterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useAdminStudents({
    status: statusFilter === "all" ? undefined : statusFilter,
    q: debouncedSearch || undefined,
    cursor: activeCursor,
    limit: 20,
    ...(isSuperAdmin && selectedSchoolId ? { schoolId: selectedSchoolId } : {}),
  });

  // Accumulate pages as they arrive
  useEffect(() => {
    if (!query.data) return;
    if (filterKey !== pageFilterKeyRef.current) return;

    setAccumulated((prev) => {
      if (activeCursor === undefined) return query.data!.data;
      const ids = new Set(prev.map((s) => s.id));
      const fresh = query.data!.data.filter((s) => !ids.has(s.id));
      return [...prev, ...fresh];
    });
    setNextCursor(query.data.next_cursor);
    setStats({
      total: query.data.total ?? 0,
      active: query.data.active ?? 0,
      deactivated: query.data.deactivated ?? 0,
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data]);

  // Bulk import dialog
  const [bulkImportOpen, setBulkImportOpen] = useState(false);

  // Register dialog
  const [registerOpen, setRegisterOpen] = useState(false);
  const [registerForm, setRegisterForm] = useState<StudentRegistrationInput>({
    name: "",
    jenjang: "",
    email: "",
    dob: "",
    gender: "",
    grade: undefined,
    alamat_domisili: "",
    target_exam: "",
    provinsi_id: undefined,
    kota_id: undefined,
    kecamatan_id: undefined,
    kode_pos: undefined,
  });
  const [registerResult, setRegisterResult] =
    useState<StudentRegistrationResult | null>(null);
  // Which school a super_admin is registering into — separate from the
  // page-level school filter (selectedSchoolId) above the student list, so
  // picking a school for this registration doesn't silently change which
  // students you're browsing.
  const [registerSchoolId, setRegisterSchoolId] = useState<string>("");

  // Reissue dialog
  const [reissueTarget, setReissueTarget] = useState<AdminStudent | null>(null);
  const [reissueResult, setReissueResult] =
    useState<StudentCredentials | null>(null);

  // Copy-to-clipboard state
  const [copied, setCopied] = useState<"username" | "password" | null>(null);

  const registerStudent = useRegisterStudent();
  const changeStatus = useChangeStudentStatus();
  const reissueCreds = useReissueStudentCredentials();

  // Region hooks (Task 17)
  const { data: provinces, isLoading: provincesLoading } = useProvinces();
  const { data: cities, isLoading: citiesLoading } = useCitiesByProvince(registerForm.provinsi_id);
  const { data: districts, isLoading: districtsLoading } = useDistrictsByCity(registerForm.kota_id);

  // Jenjang options from target school's school_types
  const currentUser = useAuthStore((s) => s.user);
  const { data: publicSchools } = useSchools();
  const adminOwnSchool = publicSchools?.find((s) => s.id === currentUser?.school_id);
  const adminOwnSchoolTypes = adminOwnSchool?.school_types ?? [];
  const registerSchoolObj = schoolsData?.data?.find((s) => s.id === registerSchoolId);
  const superAdminSchoolTypes = registerSchoolObj?.school_types ?? [];
  const schoolJenjangTypes = isSuperAdmin ? superAdminSchoolTypes : adminOwnSchoolTypes;
  // With no school chosen there are no school_types to constrain jenjang, but
  // jenjang is still required — fall back rather than leaving the field unusable.
  const jenjangOptions = schoolJenjangTypes.length ? schoolJenjangTypes : JENJANG_OPTIONS;

  const handleRegister = async () => {
    // School is deliberately not required: not every registrant is a school
    // pupil, and an operator can confirm the school after registration.
    if (!registerForm.name || !registerForm.jenjang) {
      toast.error(t("accounts_toast_required"));
      return;
    }
    try {
      const result = await registerStudent.mutateAsync({
        input: compactStudentRegistration(registerForm),
        schoolId: isSuperAdmin ? registerSchoolId : undefined,
      });
      setRegisterResult(result);
      toast.success(t("students_register_success"));
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : t("students_register_failed");
      toast.error(msg);
    }
  };

  const handleStatusToggle = async (student: AdminStudent) => {
    const newStatus =
      student.status === "active" ? "deactivated" : "active";
    const success =
      newStatus === "active"
        ? t("students_toast_activated")
        : t("students_toast_deactivated");
    try {
      await changeStatus.mutateAsync({
        id: student.id,
        status: newStatus,
        schoolId: isSuperAdmin ? selectedSchoolId : undefined,
      });
      toast.success(success);
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : t("students_toast_status_failed");
      toast.error(msg);
    }
  };

  const handleReissue = async () => {
    if (!reissueTarget) return;
    try {
      const result = await reissueCreds.mutateAsync({
        id: reissueTarget.id,
        schoolId: isSuperAdmin ? selectedSchoolId : undefined,
      });
      setReissueResult(result);
      toast.success(t("students_credential_reissued"));
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : t("students_credential_reissue_failed");
      toast.error(msg);
    }
  };

  const handleCopy = async (text: string, field: "username" | "password") => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(field);
      setTimeout(() => setCopied(null), 2000);
    } catch {
      // Clipboard unavailable — silently ignore
    }
  };

  const handleCloseRegister = () => {
    setRegisterOpen(false);
    setRegisterSchoolId("");
    // Discard plaintext credentials
    setRegisterResult(null);
    setRegisterForm({
      name: "",
      jenjang: "",
      email: "",
      dob: "",
      gender: "",
      grade: undefined,
      alamat_domisili: "",
      target_exam: "",
      provinsi_id: undefined,
      kota_id: undefined,
      kecamatan_id: undefined,
      kode_pos: undefined,
    });
  };

  const handleCloseReissue = () => {
    setReissueTarget(null);
    setReissueResult(null);
  };

  const handleLoadMore = () => {
    if (nextCursor) {
      setActiveCursor(nextCursor);
    }
  };

  const columns: DataTableColumn<AdminStudent>[] = [
    {
      key: "name",
      header: t("students_field_name"),
      cell: (s) => (
        <div className="flex items-center gap-3">
          <Avatar size="sm">
            <AvatarFallback className="bg-brand-50 text-brand-700 text-xs">
              {initials(s.name)}
            </AvatarFallback>
          </Avatar>
          <div className="font-medium text-ink-900">{s.name}</div>
        </div>
      ),
    },
    {
      key: "username",
      header: t("students_credential_username"),
      className: "font-mono text-xs text-brand-700",
      cell: (s) => (s.username ? `@${s.username}` : "—"),
    },
    {
      key: "email",
      header: t("email"),
      className: "text-xs text-ink-600",
      cell: (s) => s.email || "—",
    },
    {
      key: "school",
      header: t("students_field_school"),
      className: "text-xs",
      cell: (s) =>
        s.school_name ? (
          <span className="text-ink-600">{s.school_name}</span>
        ) : s.unlisted_school_name ? (
          <span className="text-warn" title={t("students_school_unconfirmed")}>
            {s.unlisted_school_name}
          </span>
        ) : (
          <span className="text-ink-400">{t("students_school_none")}</span>
        ),
    },
    {
      key: "status",
      header: t("th_status"),
      cell: (s) => (
        <Badge
          variant="outline"
          className={cn(
            "text-[11px] font-semibold capitalize",
            STATUS_TONE[s.status] ?? "bg-surface-2 text-ink-500 border-line"
          )}
        >
          {s.status === "active" ? t("status_label_active") : t("status_label_inactive")}
        </Badge>
      ),
    },
    {
      key: "grade",
      header: t("students_field_grade"),
      className: "text-xs text-ink-600",
      cell: (s) => s.grade || "—",
    },
    {
      key: "created",
      header: t("accounts_th_created"),
      className: "text-xs text-ink-600",
      cell: (s) =>
        s.created_at
          ? new Date(s.created_at).toLocaleString(dateLocale, {
              day: "2-digit",
              month: "short",
              year: "numeric",
            })
          : "—",
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (s) => (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon-xs" className="rounded-full">
              <MoreHorizontal className="size-4 text-ink-500" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => setReissueTarget(s)}>
              <Lock className="mr-2 size-4" />
              {t("students_credential_reissue")}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => handleStatusToggle(s)}>
              <Lock className="mr-2 size-4" />
              {s.status === "active"
                ? t("students_status_toggle_deactivated")
                : t("students_status_toggle_active")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      ),
    },
  ];

  // Loading state (first page only)
  if (query.isLoading && accumulated.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader icon={UserRound} title={t("school_students_title")} description={t("sys_loading")} />
        <div className="py-12 text-center text-ink-500">
          {t("sys_loading_data")}
        </div>
      </div>
    );
  }

  if (query.error && accumulated.length === 0) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
        <AdminPageHeader icon={UserRound} title={t("school_students_title")} description={t("sys_error_title")} />
        <div className="py-12 text-center text-ink-500">
          {t("sys_error_load")}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 md:px-6 md:py-10 fade-in">
      <AdminPageHeader
        icon={UserRound}
        title={t("school_students_title")}
        description={t("students_subtitle")}
        actions={
          <>
            <Button
              size="sm"
              variant="outline"
              className="rounded-full"
              onClick={() => setBulkImportOpen(true)}
            >
              <FileUp className="mr-1 size-4" />
              {t("bulk_register_title")}
            </Button>
            <Button
              size="sm"
              className="rounded-full"
              onClick={() => {
                setRegisterSchoolId(selectedSchoolId);
                setRegisterOpen(true);
              }}
            >
              <Plus className="mr-1 size-4" />
              {t("students_register_title")}
            </Button>
          </>
        }
      />

      {/* School picker (super_admin only) */}
      {isSuperAdmin && (
        <div className="mb-6">
          <p className="text-xs text-ink-500">{t("select_school")}</p>
          {schoolsLoading ? (
            <div className="mt-1 h-9 w-[240px] animate-pulse rounded-md bg-surface-2" />
          ) : (
            <Select
              value={selectedSchoolId || ALL_SCHOOLS_VALUE}
              onValueChange={(v) => setSelectedSchoolId(v === ALL_SCHOOLS_VALUE ? "" : v)}
            >
              <SelectTrigger className="mt-1 h-9 w-[240px] text-xs" aria-label={t("select_school")}>
                <SelectValue placeholder={t("students_all_schools")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_SCHOOLS_VALUE}>{t("students_all_schools")}</SelectItem>
                {(schoolsData?.data ?? []).map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      )}

      {/* Stats */}
      <div className="mb-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          label={t("accounts_stat_total")}
          value={String(stats.total)}
        />
        <StatCard
          label={t("status_label_active")}
          value={String(stats.active)}
        />
        <StatCard
          label={t("status_label_inactive")}
          value={String(stats.deactivated)}
        />
      </div>

      {/* Filters */}
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
          <Select
            value={statusFilter}
            onValueChange={(v) => setStatusFilter(v)}
          >
            <SelectTrigger className="h-9 w-[140px] text-xs">
              <SelectValue placeholder={t("accounts_status_placeholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("accounts_status_all")}</SelectItem>
              <SelectItem value="active">
                {t("status_label_active")}
              </SelectItem>
              <SelectItem value="deactivated">
                {t("status_label_inactive")}
              </SelectItem>
            </SelectContent>
          </Select>
          <Search className="size-4 text-ink-400" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("students_search_placeholder")}
            className="h-9 w-[200px] text-xs"
          />
        </div>
      </div>

      {/* Table */}
      <DataTable
        columns={columns}
        rows={accumulated}
        rowKey={(s) => s.id}
        empty={t("students_empty")}
        data-testid="school-students-table"
        footer={
          nextCursor ? (
            <div className="border-t border-line px-4 py-3 text-center">
              <Button
                variant="outline"
                size="sm"
                className="rounded-full"
                onClick={handleLoadMore}
                disabled={query.isFetching}
              >
                {query.isFetching ? t("sys_loading") : t("load_more")}
              </Button>
            </div>
          ) : undefined
        }
      />

      <BulkImportModal open={bulkImportOpen} onOpenChange={setBulkImportOpen} />

      {/* Register dialog */}
      <Dialog open={registerOpen} onOpenChange={(open) => {
        if (!open) handleCloseRegister();
      }}>
        <DialogContent className="sm:max-w-2xl">
          {registerResult ? (
            /* Credential panel — one-time display */
            <>
              <DialogHeader>
                <DialogTitle className="font-serif">
                  {t("students_credential_display")}
                </DialogTitle>
                <DialogDescription>
                  {t("students_credential_warning")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div>
                  <Label>{t("students_credential_username")}</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <code className="flex-1 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm font-mono">
                      {registerResult.username}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      className="rounded-full"
                      onClick={() =>
                        handleCopy(registerResult.username, "username")
                      }
                    >
                      {copied === "username" ? (
                        <Check className="size-4 text-success" />
                      ) : (
                        <Copy className="size-4" />
                      )}
                      <span className="ml-1">
                        {copied === "username"
                          ? t("students_credential_copied")
                          : t("students_credential_copy")}
                      </span>
                    </Button>
                  </div>
                </div>
                <div>
                  <Label>{t("students_credential_password")}</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <code className="flex-1 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm font-mono">
                      {registerResult.temp_password}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      className="rounded-full"
                      onClick={() =>
                        handleCopy(registerResult.temp_password, "password")
                      }
                    >
                      {copied === "password" ? (
                        <Check className="size-4 text-success" />
                      ) : (
                        <Copy className="size-4" />
                      )}
                      <span className="ml-1">
                        {copied === "password"
                          ? t("students_credential_copied")
                          : t("students_credential_copy")}
                      </span>
                    </Button>
                  </div>
                </div>
              </div>
              <DialogFooter className="mt-4">
                <Button className="rounded-full" onClick={handleCloseRegister}>
                  {t("cancel")}
                </Button>
              </DialogFooter>
            </>
          ) : (
            /* Registration form */
            <>
              <DialogHeader>
                <DialogTitle className="font-serif text-xl">
                  {t("students_register_title")}
                </DialogTitle>
                <DialogDescription>
                  {t("students_register_desc")}
                </DialogDescription>
              </DialogHeader>

              <RegisterSection
                icon={UserRound}
                label={t("students_register_section_identity")}
                delay={0}
                isFirst
              >
                <FormField label={t("students_field_name")} required>
                  <Input
                    value={registerForm.name}
                    onChange={(e) =>
                      setRegisterForm((f) => ({
                        ...f,
                        name: e.target.value,
                      }))
                    }
                    placeholder={t("students_field_name")}
                  />
                </FormField>
                <div className="grid grid-cols-2 gap-4">
                  <FormField label={t("students_field_email")}>
                    <Input
                      type="email"
                      value={registerForm.email ?? ""}
                      onChange={(e) =>
                        setRegisterForm((f) => ({
                          ...f,
                          email: e.target.value || undefined,
                        }))
                      }
                      placeholder={t("accounts_placeholder_email")}
                    />
                  </FormField>
                  <FormField label={t("students_field_dob")}>
                    <Input
                      type="date"
                      value={registerForm.dob ?? ""}
                      onChange={(e) =>
                        setRegisterForm((f) => ({
                          ...f,
                          dob: e.target.value || undefined,
                        }))
                      }
                    />
                  </FormField>
                </div>
                <FormField label={t("students_field_gender")}>
                  <Select
                    value={registerForm.gender ?? ""}
                    onValueChange={(v) =>
                      setRegisterForm((f) => ({
                        ...f,
                        gender: v || undefined,
                      }))
                    }
                  >
                    <SelectTrigger>
                      <SelectValue
                        placeholder={t("accounts_placeholder_pick_role")}
                      />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="male">
                        {lang === "id" ? "Laki-laki" : "Male"}
                      </SelectItem>
                      <SelectItem value="female">
                        {lang === "id" ? "Perempuan" : "Female"}
                      </SelectItem>
                    </SelectContent>
                  </Select>
                </FormField>
              </RegisterSection>

              <RegisterSection
                icon={GraduationCap}
                label={t("students_register_section_academic")}
                delay={60}
              >
                {isSuperAdmin && (
                  <FormField label={t("school")} hint={t("students_school_optional_hint")}>
                    <Select
                      value={registerSchoolId || NO_SCHOOL_VALUE}
                      onValueChange={(v) => {
                        setRegisterSchoolId(v === NO_SCHOOL_VALUE ? "" : v);
                        // Jenjang options depend on the chosen school — a
                        // previously picked jenjang may no longer be valid.
                        setRegisterForm((f) => ({ ...f, jenjang: "" }));
                      }}
                    >
                      <SelectTrigger aria-label={t("school")}>
                        <SelectValue placeholder={t("students_school_none_option")} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={NO_SCHOOL_VALUE}>
                          {t("students_school_none_option")}
                        </SelectItem>
                        {(schoolsData?.data ?? []).map((s) => (
                          <SelectItem key={s.id} value={s.id}>
                            {s.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormField>
                )}
                <div className="grid grid-cols-2 gap-4">
                  <FormField label={t("students_field_jenjang")} required>
                    <Select
                      value={registerForm.jenjang}
                      onValueChange={(v) =>
                        setRegisterForm((f) => ({
                          ...f,
                          jenjang: v,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("students_field_jenjang")} />
                      </SelectTrigger>
                      <SelectContent>
                        {jenjangOptions.length === 0 ? (
                          <div className="px-2 py-3 text-center text-xs text-ink-500">
                            {t("students_field_jenjang_empty")}
                          </div>
                        ) : (
                          jenjangOptions.map((opt) => (
                            <SelectItem key={opt} value={opt}>
                              {opt}
                            </SelectItem>
                          ))
                        )}
                      </SelectContent>
                    </Select>
                  </FormField>
                  <FormField label={t("students_field_grade")}>
                    <Input
                      value={registerForm.grade ?? ""}
                      onChange={(e) =>
                        setRegisterForm((f) => ({
                          ...f,
                          grade: e.target.value ? Number(e.target.value) : undefined,
                        }))
                      }
                      placeholder={t("students_field_grade")}
                    />
                  </FormField>
                </div>
                <FormField label={t("students_field_target_exam")}>
                  <Input
                    value={registerForm.target_exam ?? ""}
                    onChange={(e) =>
                      setRegisterForm((f) => ({
                        ...f,
                        target_exam: e.target.value || undefined,
                      }))
                    }
                    placeholder={t("target_exam_examples")}
                  />
                </FormField>
              </RegisterSection>

              <RegisterSection
                icon={MapPin}
                label={t("students_register_section_address")}
                delay={120}
              >
                <FormField label={t("students_field_alamat_domisili")}>
                  <Input
                    value={registerForm.alamat_domisili ?? ""}
                    onChange={(e) =>
                      setRegisterForm((f) => ({
                        ...f,
                        alamat_domisili: e.target.value || undefined,
                      }))
                    }
                    placeholder={t("students_field_alamat_domisili")}
                  />
                </FormField>
                <div className="grid grid-cols-2 gap-4">
                  <FormField label={t("students_field_provinsi")}>
                    <Select
                      value={registerForm.provinsi_id ?? ""}
                      onValueChange={(v) =>
                        setRegisterForm((f) => ({
                          ...f,
                          provinsi_id: v || undefined,
                          kota_id: undefined,
                          kecamatan_id: undefined,
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("students_field_provinsi")} />
                      </SelectTrigger>
                      <SelectContent>
                        {(provinces ?? []).map((p) => (
                          <SelectItem key={p.id} value={p.id}>
                            {p.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormField>
                  <FormField label={t("students_field_kota")}>
                    <Select
                      value={registerForm.kota_id ?? ""}
                      onValueChange={(v) =>
                        setRegisterForm((f) => ({
                          ...f,
                          kota_id: v || undefined,
                          kecamatan_id: undefined,
                        }))
                      }
                      disabled={!registerForm.provinsi_id}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("students_field_kota")} />
                      </SelectTrigger>
                      <SelectContent>
                        {(cities ?? []).map((c) => (
                          <SelectItem key={c.id} value={c.id}>
                            {c.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormField>
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <FormField label={t("students_field_kecamatan")}>
                    <Select
                      value={registerForm.kecamatan_id ?? ""}
                      onValueChange={(v) =>
                        setRegisterForm((f) => ({
                          ...f,
                          kecamatan_id: v || undefined,
                        }))
                      }
                      disabled={!registerForm.kota_id}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("students_field_kecamatan")} />
                      </SelectTrigger>
                      <SelectContent>
                        {(districts ?? []).map((d) => (
                          <SelectItem key={d.id} value={d.id}>
                            {d.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormField>
                  <FormField label={t("students_field_kode_pos")}>
                    <Input
                      value={registerForm.kode_pos ?? ""}
                      onChange={(e) =>
                        setRegisterForm((f) => ({
                          ...f,
                          kode_pos: e.target.value || undefined,
                        }))
                      }
                      placeholder={t("students_field_kode_pos")}
                    />
                  </FormField>
                </div>
              </RegisterSection>

              <DialogFooter className="mt-6 border-t border-line pt-4">
                <Button variant="outline" className="rounded-full" onClick={handleCloseRegister}>
                  {t("cancel")}
                </Button>
                <Button
                  className="rounded-full"
                  onClick={handleRegister}
                  disabled={registerStudent.isPending}
                >
                  {registerStudent.isPending
                    ? t("saving")
                    : t("students_register_title")}
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Reissue credentials dialog */}
      <Dialog
        open={reissueTarget !== null}
        onOpenChange={(open) => {
          if (!open) handleCloseReissue();
        }}
      >
        <DialogContent className="sm:max-w-lg">
          {reissueResult ? (
            /* Credential panel — one-time display */
            <>
              <DialogHeader>
                <DialogTitle className="font-serif">
                  {t("students_credential_display")}
                </DialogTitle>
                <DialogDescription>
                  {t("students_credential_warning")}
                </DialogDescription>
              </DialogHeader>
              <div className="space-y-4">
                <div>
                  <Label>{t("students_credential_username")}</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <code className="flex-1 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm font-mono">
                      {reissueResult.username}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      className="rounded-full"
                      onClick={() =>
                        handleCopy(reissueResult.username, "username")
                      }
                    >
                      {copied === "username" ? (
                        <Check className="size-4 text-success" />
                      ) : (
                        <Copy className="size-4" />
                      )}
                      <span className="ml-1">
                        {copied === "username"
                          ? t("students_credential_copied")
                          : t("students_credential_copy")}
                      </span>
                    </Button>
                  </div>
                </div>
                <div>
                  <Label>{t("students_credential_password")}</Label>
                  <div className="mt-1 flex items-center gap-2">
                    <code className="flex-1 rounded-md border border-line bg-surface-2 px-3 py-2 text-sm font-mono">
                      {reissueResult.temp_password}
                    </code>
                    <Button
                      variant="outline"
                      size="sm"
                      className="rounded-full"
                      onClick={() =>
                        handleCopy(reissueResult.temp_password, "password")
                      }
                    >
                      {copied === "password" ? (
                        <Check className="size-4 text-success" />
                      ) : (
                        <Copy className="size-4" />
                      )}
                      <span className="ml-1">
                        {copied === "password"
                          ? t("students_credential_copied")
                          : t("students_credential_copy")}
                      </span>
                    </Button>
                  </div>
                </div>
              </div>
              <DialogFooter className="mt-4">
                <Button className="rounded-full" onClick={handleCloseReissue}>
                  {t("cancel")}
                </Button>
              </DialogFooter>
            </>
          ) : (
            /* Confirmation step */
            <>
              <DialogHeader>
                <DialogTitle className="font-serif">
                  {t("students_credential_reissue")}
                </DialogTitle>
                <DialogDescription>
                  {t("students_credential_reissue_warning")}
                </DialogDescription>
              </DialogHeader>
              <DialogFooter className="mt-4">
                <Button variant="outline" className="rounded-full" onClick={handleCloseReissue}>
                  {t("cancel")}
                </Button>
                <Button
                  className="rounded-full"
                  onClick={handleReissue}
                  disabled={reissueCreds.isPending}
                >
                  {reissueCreds.isPending
                    ? t("saving")
                    : t("students_credential_reissue")}
                </Button>
              </DialogFooter>
            </>
          )}
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
          : "border-line bg-surface text-ink-600 hover:text-ink-900"
      )}
    >
      {children}
    </button>
  );
}

// Groups a set of fields in the Register Student dialog under a small
// icon + eyebrow label, with a staggered fade-in on open (delay in ms).
function RegisterSection({
  icon: Icon,
  label,
  delay,
  isFirst,
  children,
}: {
  icon: LucideIcon;
  label: string;
  delay: number;
  isFirst?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div
      className={cn(
        "fade-in space-y-4",
        isFirst ? "mt-5" : "mt-6 border-t border-line pt-6",
      )}
      style={{ animationDelay: `${delay}ms`, animationFillMode: "backwards" }}
    >
      <div className="flex items-center gap-2">
        <Icon className="size-4 text-brand-600" />
        <h4 className="text-[11px] font-semibold uppercase tracking-wide text-ink-500">
          {label}
        </h4>
      </div>
      <div className="space-y-4">{children}</div>
    </div>
  );
}

function FormField({
  label,
  required,
  hint,
  children,
}: {
  label: string;
  required?: boolean;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="space-y-1.5">
      <Label>
        {label} {required && <span className="text-danger">*</span>}
      </Label>
      {children}
      {hint && <p className="text-xs text-ink-500">{hint}</p>}
    </div>
  );
}
