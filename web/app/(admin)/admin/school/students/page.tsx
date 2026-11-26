"use client";

import { useEffect, useRef, useState } from "react";
import {
  Plus,
  MoreHorizontal,
  Lock,
  Copy,
  Check,
  Eye,
  EyeOff,
  FileUp,
  UserRound,
  GraduationCap,
  MapPin,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { JENJANG_OPTIONS } from "@/lib/jenjang";
import { formatGrade, GRADE_GROUPS, gradeOptionsForJenjang } from "@/lib/grade";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { PageHead } from "@/components/shell/PageHead";
import { StatCard } from "@/components/admin/StatCard";
import {
  ActiveChip,
  FILTER_PANEL_CLASS,
  FilterGrid,
  FilterPanelFooter,
  FilterSelect,
  ToolbarCount,
  ToolbarFilterButton,
  ToolbarSearch,
} from "@/components/admin/ListToolbar";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
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
import { SchoolPicker } from "@/components/SchoolPicker";
import {
  useAdminStudents,
  useRegisterStudent,
  useChangeStudentStatus,
  useReissueStudentCredentials,
  useSetStudentPassword,
} from "@/lib/hooks/admin-students";
import { hasCapability } from "@/lib/hooks/use-capability";
import { useSchoolById } from "@/lib/hooks/students";
import { useProvinces, useCitiesByProvince, useDistrictsByCity } from "@/lib/hooks/regions";
import { useAuthStore } from "@/stores/auth";
import type {
  AdminStudent,
  StudentRegistrationInput,
  StudentRegistrationResult,
  StudentCredentials,
  SchoolOption,
} from "@/lib/types";

// Search is sent to the server (q param), so it must be debounced the same
// way the schools page debounces school search — otherwise every keystroke
// fires a new paginated request and resets the accumulated list.

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
  const password = form.password?.trim();
  if (password) payload.password = password;
  const unlistedSchoolName = form.unlisted_school_name?.trim();
  if (unlistedSchoolName) payload.unlisted_school_name = unlistedSchoolName;
  return payload;
}

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

  // School picker and global actions are for roles holding student-scope:all
  const currentRole = useAuthStore((s) => s.user?.role);
  const canManageAllStudents = hasCapability(currentRole, "student-scope:all");
  const [selectedSchoolId, setSelectedSchoolId] = useState<string>("");
  const [selectedSchoolFilter, setSelectedSchoolFilter] = useState<SchoolOption | null>(null);

  // Filters
  // searchInput is the raw input value; debouncedSearch is what actually goes
  // to the server (q param) and into filterKey below, so pagination doesn't
  // reset and refetch on every keystroke.
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(searchInput).trim();
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [jenjangFilter, setJenjangFilter] = useState("");

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
  const filterKey = `${statusFilter}:${debouncedSearch}:${selectedSchoolId}:${jenjangFilter}`;
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
    jenjang: jenjangFilter || undefined,
    cursor: activeCursor,
    limit: 20,
    ...(canManageAllStudents && selectedSchoolId ? { schoolId: selectedSchoolId } : {}),
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
  const [showRegisterPassword, setShowRegisterPassword] = useState(false);
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
    password: "",
  });
  const [registerResult, setRegisterResult] =
    useState<StudentRegistrationResult | null>(null);
  // Which school a super_admin is registering into — separate from the
  // page-level school filter (selectedSchoolId) above the student list, so
  // picking a school for this registration doesn't silently change which
  // students you're browsing.
  const [registerSchoolId, setRegisterSchoolId] = useState<string>("");
  const [registerSelectedSchool, setRegisterSelectedSchool] = useState<SchoolOption | null>(null);
  const [registerUnlistedSchoolName, setRegisterUnlistedSchoolName] = useState("");

  // Reissue dialog
  const [reissueTarget, setReissueTarget] = useState<AdminStudent | null>(null);
  const [reissueResult, setReissueResult] =
    useState<StudentCredentials | null>(null);

  // Set password dialog
  const [setPasswordTarget, setSetPasswordTarget] = useState<AdminStudent | null>(null);
  const [passwordForm, setPasswordForm] = useState({
    newPassword: "",
    confirmPassword: "",
  });
  const [showNewPassword, setShowNewPassword] = useState(false);
  const [showConfirmPassword, setShowConfirmPassword] = useState(false);

  // Copy-to-clipboard state
  const [copied, setCopied] = useState<"username" | "password" | null>(null);

  const registerStudent = useRegisterStudent();
  const changeStatus = useChangeStudentStatus();
  const reissueCreds = useReissueStudentCredentials();
  const setPassword = useSetStudentPassword();

  // Region hooks (Task 17)
  const { data: provinces, isLoading: provincesLoading } = useProvinces();
  const { data: cities, isLoading: citiesLoading } = useCitiesByProvince(registerForm.provinsi_id);
  const { data: districts, isLoading: districtsLoading } = useDistrictsByCity(registerForm.kota_id);

  // Jenjang options from target school's school_types
  const currentUser = useAuthStore((s) => s.user);
  const { data: adminOwnSchool } = useSchoolById(currentUser?.school_id ?? "");
  const { data: hydratedRegisterSchool } = useSchoolById(registerSchoolId);
  const adminOwnSchoolTypes = adminOwnSchool?.school_types ?? [];
  const registerSchoolObj = registerSelectedSchool?.id === registerSchoolId ? registerSelectedSchool : hydratedRegisterSchool;
  const superAdminSchoolTypes = registerSchoolObj?.school_types ?? [];
  const schoolJenjangTypes = canManageAllStudents ? superAdminSchoolTypes : adminOwnSchoolTypes;
  // With no school chosen there are no school_types to constrain jenjang, but
  // jenjang is still required — fall back rather than leaving the field unusable.
  const jenjangOptions = schoolJenjangTypes.length ? schoolJenjangTypes : JENJANG_OPTIONS;
  const registerGradeOptions = gradeOptionsForJenjang(registerForm.jenjang);

  const handleRegister = async () => {
    // School is deliberately not required: not every registrant is a school
    // pupil, and an operator can confirm the school after registration.
    if (!registerForm.name || !registerForm.jenjang) {
      toast.error(t("accounts_toast_required"));
      return;
    }
    try {
      const input = compactStudentRegistration({
        ...registerForm,
        unlisted_school_name: canManageAllStudents && !registerSchoolId ? registerUnlistedSchoolName : undefined,
      });
      const result = await registerStudent.mutateAsync({
        input,
        schoolId: canManageAllStudents && registerSchoolId ? registerSchoolId : undefined,
      });
      toast.success(t("students_register_success"));
      setRegisterResult(result);
    } catch (err: unknown) {
      const msg =
        err instanceof Error ? err.message : t("students_register_failed");
      toast.error(msg);
    }
  };

  const handleSetPassword = async () => {
    if (!setPasswordTarget) return;
    if (!passwordForm.newPassword) {
      toast.error(t("accounts_toast_required"));
      return;
    }
    if (passwordForm.newPassword !== passwordForm.confirmPassword) {
      toast.error(t("students_password_mismatch"));
      return;
    }
    try {
      await setPassword.mutateAsync({
        id: setPasswordTarget.id,
        newPassword: passwordForm.newPassword,
      });
      toast.success(t("students_password_updated"));
      handleCloseSetPassword();
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : t("students_password_update_failed");
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
        schoolId: canManageAllStudents ? selectedSchoolId : undefined,
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
        schoolId: canManageAllStudents ? selectedSchoolId : undefined,
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
    setRegisterSelectedSchool(null);
    setRegisterUnlistedSchoolName("");
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
      password: "",
    });
  };

  const handleCloseReissue = () => {
    setReissueTarget(null);
    setReissueResult(null);
  };

  const handleCloseSetPassword = () => {
    setSetPasswordTarget(null);
    setPasswordForm({ newPassword: "", confirmPassword: "" });
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
          <div className="min-w-0">
            <div className="font-medium text-ink-900">{s.name}</div>
            <div className="mt-1 font-mono text-xs text-brand-700">
              {s.username ? `@${s.username}` : "—"}
            </div>
          </div>
        </div>
      ),
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
      cell: (s) => (
        <div>
          {s.school_name ? (
            <span className="text-ink-600">{s.school_name}</span>
          ) : s.unlisted_school_name ? (
            <span className="text-warn" title={t("students_school_unconfirmed")}>
              {s.unlisted_school_name}
            </span>
          ) : (
            <span className="text-ink-400">{t("students_school_none")}</span>
          )}
          <div className="mt-1 text-[11px] text-ink-400">
            {t("students_field_grade")} {s.grade != null ? formatGrade(s.grade) : "—"}
          </div>
        </div>
      ),
    },
    {
      key: "status",
      header: t("th_status"),
      cell: (s) => (
        <Badge variant={s.status === "active" ? "teal" : "danger"}>
          {s.status === "active" ? t("status_label_active") : t("status_label_inactive")}
        </Badge>
      ),
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
            {canManageAllStudents && (
              <DropdownMenuItem onClick={() => setSetPasswordTarget(s)}>
                <Lock className="mr-2 size-4" />
                {t("students_set_password")}
              </DropdownMenuItem>
            )}
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

  // Loading/error states render inside the table (DataTable's empty cell)
  // instead of replacing the whole page — an early return would unmount the
  // search input mid-search, dropping its focus every time results refresh
  // (same shape as ParticipantPicker, which keeps its toolbar mounted).
  const tableEmpty =
    query.error && accumulated.length === 0
      ? t("sys_error_load")
      : query.isLoading
        ? t("sys_loading_data")
        : t("students_empty");

  const filterPanelLabel = t("students_filter_panel_label");
  const rosterLabel = t("students_roster_label");
  const filterCount = (statusFilter !== "all" ? 1 : 0) + (selectedSchoolId ? 1 : 0) + (jenjangFilter ? 1 : 0);
  const statusOptions = [
    { value: "active", label: t("status_label_active") },
    { value: "deactivated", label: t("status_label_inactive") },
  ];
  const resetFilters = () => {
    setStatusFilter("all");
    setJenjangFilter("");
    setSelectedSchoolFilter(null);
    setSelectedSchoolId("");
  };

  return (
    <div className="flex flex-col gap-5 fade-in">
      <PageHead
        title={t("school_students_title")}
        sub={[!canManageAllStudents && adminOwnSchool?.name, t("adm_accounts_count").replace("{n}", String(stats.total))].filter(Boolean).join(" · ")}
        actions={
          <>
            <Button variant="outline" onClick={() => setBulkImportOpen(true)}>
              <FileUp className="size-4" />
              {t("bulk_register_title")}
            </Button>
            <Button
              variant="dark"
              onClick={() => {
                setRegisterSelectedSchool(selectedSchoolFilter);
                setRegisterSchoolId(selectedSchoolId);
                setRegisterUnlistedSchoolName("");
                setRegisterOpen(true);
              }}
            >
              <Plus className="size-4" />
              {t("students_register_title")}
            </Button>
          </>
        }
      />

      <div className="grid grid-cols-3 gap-3 max-narrow:grid-cols-1">
        <StatCard tone="brand" label={t("accounts_stat_total")} value={String(stats.total)} />
        <StatCard tone="teal" label={t("status_label_active")} value={String(stats.active)} />
        <StatCard tone="ember" label={t("status_label_inactive")} value={String(stats.deactivated)} />
      </div>

      <section aria-label={rosterLabel} className="md-card-flush">
        <div className="flex flex-col gap-2.5 border-b border-line px-4 py-3.5">
          <div className="flex flex-wrap items-center gap-2.5">
            <ToolbarSearch
              value={searchInput}
              onChange={setSearchInput}
              placeholder={t("students_search_placeholder")}
              clearLabel={t("list_toolbar_clear_search")}
            />
            <Popover>
              <PopoverTrigger asChild>
                <ToolbarFilterButton label={filterPanelLabel} count={filterCount} />
              </PopoverTrigger>
              <PopoverContent align="end" role="region" aria-label={filterPanelLabel} className={FILTER_PANEL_CLASS}>
                <div className="flex flex-col gap-3.5">
                  <FilterGrid>
                    <FilterSelect
                      label={t("accounts_th_status")}
                      value={statusFilter === "all" ? "" : statusFilter}
                      options={statusOptions}
                      onChange={(v) => setStatusFilter(v || "all")}
                    />
                    <FilterSelect
                      label={t("students_filter_jenjang")}
                      value={jenjangFilter}
                      options={JENJANG_OPTIONS.map((j) => ({ value: j, label: j }))}
                      onChange={setJenjangFilter}
                    />
                    <div className="flex min-w-0 flex-col gap-1.5 sm:col-span-2">
                      <span className="font-mono text-[11px] font-semibold tracking-[.06em] text-ink-600 uppercase">{t("select_school")}</span>
                      {canManageAllStudents ? (
                        <SchoolPicker
                          id="student-school-filter"
                          value={selectedSchoolId}
                          selectedSchool={selectedSchoolFilter}
                          onChange={(school) => {
                            setSelectedSchoolFilter(school);
                            setSelectedSchoolId(school?.id ?? "");
                          }}
                        />
                      ) : (
                        <p className="text-xs leading-5 text-ink-600">{t("students_roster_bound_note")}</p>
                      )}
                    </div>
                  </FilterGrid>
                  <FilterPanelFooter
                    onReset={resetFilters}
                    showLabel={t("list_toolbar_show_n").replace("{n}", String(stats.total))}
                  />
                </div>
              </PopoverContent>
            </Popover>
            <ToolbarCount>{t("adm_accounts_count").replace("{n}", String(stats.total))}</ToolbarCount>
          </div>
          {filterCount > 0 && (
            <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={t("list_toolbar_active")}>
              {selectedSchoolId && (
                <ActiveChip
                  label={selectedSchoolFilter?.name ?? t("select_school")}
                  removeLabel={t("students_all_schools")}
                  onRemove={() => {
                    setSelectedSchoolFilter(null);
                    setSelectedSchoolId("");
                  }}
                />
              )}
              {jenjangFilter && (
                <ActiveChip
                  label={`${t("students_filter_jenjang")}: ${jenjangFilter}`}
                  removeLabel={`${t("list_toolbar_remove")}: ${t("students_filter_jenjang")}`}
                  onRemove={() => setJenjangFilter("")}
                />
              )}
              {statusFilter !== "all" && (
                <ActiveChip
                  label={`${t("accounts_th_status")}: ${statusOptions.find((o) => o.value === statusFilter)?.label}`}
                  removeLabel={`${t("list_toolbar_remove")}: ${t("accounts_th_status")}`}
                  onRemove={() => setStatusFilter("all")}
                />
              )}
              <Button variant="ghost" size="sm" onClick={resetFilters}>{t("list_toolbar_clear_all")}</Button>
            </div>
          )}
        </div>

        <DataTable
          columns={columns}
          rows={accumulated}
          rowKey={(s) => s.id}
          empty={tableEmpty}
          surface="plain"
          data-testid="school-students-table"
          footer={
            nextCursor ? (
              <div className="border-t border-line px-4 py-3 text-center">
                <Button variant="outline" size="sm" onClick={handleLoadMore} disabled={query.isFetching}>
                  {query.isFetching ? t("sys_loading") : t("load_more")}
                </Button>
              </div>
            ) : undefined
          }
        />
      </section>

      <BulkImportModal open={bulkImportOpen} onOpenChange={setBulkImportOpen} allowExplicitPassword={canManageAllStudents} />

      {/* Register dialog */}
      <Dialog open={registerOpen} onOpenChange={(open) => {
        if (!open) handleCloseRegister();
      }}>
        <DialogContent className="max-w-[740px]">
          {registerResult ? (
            /* Credential panel — one-time display */
            <>
              <DialogHeader>
                <DialogTitle>
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
                {registerResult.temp_password ? (
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
                        handleCopy(registerResult.temp_password ?? "", "password")
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
                ) : null}
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
                <DialogTitle>
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
                {canManageAllStudents && (
                  <FormField label={t("students_password_optional")} hint={t("students_password_optional_hint")}>
                    <div className="relative">
                      <Input
                        type={showRegisterPassword ? "text" : "password"}
                        value={registerForm.password ?? ""}
                        onChange={(e) =>
                          setRegisterForm((f) => ({
                            ...f,
                            password: e.target.value,
                          }))
                        }
                        placeholder={t("students_password_optional")}
                        className="pr-10"
                      />
                      <button
                        type="button"
                        onClick={() => setShowRegisterPassword((p) => !p)}
                        aria-label={showRegisterPassword ? t("auth_hide_password") : t("auth_show_password")}
                        className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
                      >
                        {showRegisterPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                      </button>
                    </div>
                  </FormField>
                )}
              </RegisterSection>

              <RegisterSection
                icon={GraduationCap}
                label={t("students_register_section_academic")}
                delay={60}
              >
                {canManageAllStudents && (
                  <FormField label={t("school")} hint={t("students_school_optional_hint")}>
                    <SchoolPicker
                      id="register-school"
                      value={registerSchoolId}
                      selectedSchool={registerSelectedSchool}
                      onChange={(school) => {
                        setRegisterSelectedSchool(school);
                        setRegisterSchoolId(school?.id ?? "");
                        setRegisterForm((f) => ({ ...f, jenjang: "", grade: undefined }));
                      }}
                      allowUnlisted
                      unlistedName={registerUnlistedSchoolName}
                      onUnlistedNameChange={setRegisterUnlistedSchoolName}
                    />
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
                          grade:
                            f.grade != null && !gradeOptionsForJenjang(v).includes(f.grade)
                              ? undefined
                              : f.grade,
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
                    <Select
                      value={registerForm.grade != null ? String(registerForm.grade) : "_empty_"}
                      onValueChange={(v) =>
                        setRegisterForm((f) => ({
                          ...f,
                          grade: v === "_empty_" ? undefined : Number(v),
                        }))
                      }
                    >
                      <SelectTrigger>
                        <SelectValue placeholder={t("students_field_grade")} />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="_empty_">{t("students_field_grade")}</SelectItem>
                        {registerGradeOptions.length > 0
                          ? registerGradeOptions.map((g) => (
                              <SelectItem key={g} value={String(g)}>
                                {formatGrade(g)}
                              </SelectItem>
                            ))
                          : GRADE_GROUPS.map((group) => (
                              <SelectGroup key={group.label}>
                                <SelectLabel>{group.label}</SelectLabel>
                                {group.values.map((g) => (
                                  <SelectItem key={g} value={String(g)}>
                                    {formatGrade(g)}
                                  </SelectItem>
                                ))}
                              </SelectGroup>
                            ))}
                      </SelectContent>
                    </Select>
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
        <DialogContent className="max-w-[480px]">
          {reissueResult ? (
            /* Credential panel — one-time display */
            <>
              <DialogHeader>
                <DialogTitle>
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
                <DialogTitle>
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

      {/* Set password dialog */}
      <Dialog
        open={setPasswordTarget !== null}
        onOpenChange={(open) => {
          if (!open) handleCloseSetPassword();
        }}
      >
        <DialogContent className="max-w-[480px]">
          <DialogHeader>
            <DialogTitle>
              {t("students_set_password")}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <FormField label={t("students_new_password")} required>
              <div className="relative">
                <Input
                  type={showNewPassword ? "text" : "password"}
                  value={passwordForm.newPassword}
                  onChange={(e) =>
                    setPasswordForm((f) => ({
                      ...f,
                      newPassword: e.target.value,
                    }))
                  }
                  placeholder={t("students_new_password")}
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowNewPassword((p) => !p)}
                  aria-label={showNewPassword ? t("auth_hide_password") : t("auth_show_password")}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
                >
                  {showNewPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </FormField>
            <FormField label={t("students_confirm_password")} required>
              <div className="relative">
                <Input
                  type={showConfirmPassword ? "text" : "password"}
                  value={passwordForm.confirmPassword}
                  onChange={(e) =>
                    setPasswordForm((f) => ({
                      ...f,
                      confirmPassword: e.target.value,
                    }))
                  }
                  placeholder={t("students_confirm_password")}
                  className="pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowConfirmPassword((p) => !p)}
                  aria-label={showConfirmPassword ? t("auth_hide_password") : t("auth_show_password")}
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-ink-400 transition-colors hover:text-ink-600"
                >
                  {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </FormField>
          </div>
          <DialogFooter className="mt-4">
            <Button variant="outline" className="rounded-full" onClick={handleCloseSetPassword}>
              {t("cancel")}
            </Button>
            <Button
              className="rounded-full"
              onClick={handleSetPassword}
              disabled={setPassword.isPending}
            >
              {setPassword.isPending
                ? t("saving")
                : t("students_set_password")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
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
