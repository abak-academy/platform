"use client";

import { useMemo, useState } from "react";
import {
  Plus,
  MoreHorizontal,
  Lock,
  Mail,
} from "lucide-react";
import { toast } from "sonner";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Badge, badgeVariants, statusTone } from "@/components/ui/badge";
import type { VariantProps } from "class-variance-authority";
import { Input } from "@/components/ui/input";
import { PasswordInput } from "@/components/ui/password-input";
import { Label } from "@/components/ui/label";
import { Avatar, AvatarFallback } from "@/components/ui/avatar";
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
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ToolbarCount, ToolbarSearch } from "@/components/admin/ListToolbar";
import { DataTable, TableItem } from "@/components/ui/data-table";
import { SchoolFilterPicker } from "@/components/SchoolFilterPicker";
import {
  useAdminAccounts,
  useCreateAdminAccount,
  useChangeAccountRole,
  useChangeAccountStatus,
  useResetAccountPassword,
} from "@/lib/hooks/admin-accounts";
import type { AdminAccount, AdminAccountRole, AdminAccountStatus } from "@/lib/types";

type BadgeVariant = NonNullable<VariantProps<typeof badgeVariants>["variant"]>;

const ROLE_TONE: Record<AdminAccountRole, BadgeVariant> = {
  super_admin: "danger",
  admin_exam: "sky",
  admin_school: "berry",
  admin_store: "teal",
};

const ROLES: AdminAccountRole[] = ["super_admin", "admin_exam", "admin_school", "admin_store"];

function initials(name: string) {
  return name
    .split(" ")
    .map((n) => n[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
}

export default function SystemAccountsPage() {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";
  const ROLE_LABEL: Record<AdminAccountRole, string> = {
    super_admin: t("role_super_admin"),
    admin_exam: t("role_admin_exam"),
    admin_school: t("role_admin_school"),
    admin_store: t("role_admin_store"),
  };
  const [search, setSearch] = useState("");
  const [roleFilter, setRoleFilter] = useState<AdminAccountRole | "all">("all");
  const [statusFilter, setStatusFilter] = useState<AdminAccountStatus | "all">("all");
  const [createOpen, setCreateOpen] = useState(false);
  const [createForm, setCreateForm] = useState({
    name: "",
    email: "",
    role: "admin_store" as AdminAccountRole,
    password: "",
    school_id: "",
  });
  const [roleChangeTarget, setRoleChangeTarget] = useState<AdminAccount | null>(null);
  const [roleChangeRole, setRoleChangeRole] = useState<AdminAccountRole>("admin_store");
  const [roleChangeSchoolId, setRoleChangeSchoolId] = useState("");

  const { data: accounts = [], isLoading, error } = useAdminAccounts(
    roleFilter === "all" ? undefined : roleFilter,
    statusFilter === "all" ? undefined : statusFilter
  );

  const createAccount = useCreateAdminAccount();
  const changeRole = useChangeAccountRole();
  const changeStatus = useChangeAccountStatus();
  const resetPwd = useResetAccountPassword();

  const rows = useMemo(() => {
    if (search.trim() === "") return accounts;
    const q = search.toLowerCase();
    return accounts.filter(
      (a) => a.name.toLowerCase().includes(q) || (a.email ?? "").toLowerCase().includes(q)
    );
  }, [search, accounts]);

  const handleCreate = async () => {
    if (!createForm.name || !createForm.email || !createForm.password) {
      toast.error(t("accounts_toast_required"));
      return;
    }
    if (createForm.role === "admin_school" && !createForm.school_id) {
      toast.error(t("accounts_toast_school_required"));
      return;
    }
    try {
      await createAccount.mutateAsync({
        name: createForm.name,
        email: createForm.email,
        role: createForm.role,
        password: createForm.password,
        school_id: createForm.school_id || undefined,
      });
      toast.success(t("accounts_toast_created"));
      setCreateOpen(false);
      setCreateForm({ name: "", email: "", role: "admin_store", password: "", school_id: "" });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("accounts_toast_create_failed");
      toast.error(msg);
    }
  };

  const handleRoleChange = async () => {
    if (!roleChangeTarget) return;
    if (roleChangeRole === "admin_school" && !roleChangeSchoolId) {
      toast.error(t("accounts_toast_school_required"));
      return;
    }
    try {
      await changeRole.mutateAsync({
        id: roleChangeTarget.id,
        role: roleChangeRole,
        school_id: roleChangeRole === "admin_school" ? roleChangeSchoolId || undefined : undefined,
      });
      toast.success(t("accounts_toast_role_changed"));
      setRoleChangeTarget(null);
      setRoleChangeSchoolId("");
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("accounts_toast_role_failed");
      toast.error(msg);
    }
  };

  const handleStatusToggle = async (account: AdminAccount) => {
    const newStatus: AdminAccountStatus = account.status === "active" ? "deactivated" : "active";
    const success = newStatus === "active" ? t("accounts_toast_activated") : t("accounts_toast_deactivated");
    const failure = newStatus === "active" ? t("accounts_toast_activate_failed") : t("accounts_toast_deactivate_failed");
    try {
      await changeStatus.mutateAsync({ id: account.id, status: newStatus });
      toast.success(success);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : failure;
      toast.error(msg);
    }
  };

  const handleResetPassword = async (account: AdminAccount) => {
    try {
      await resetPwd.mutateAsync(account.id);
      toast.success(t("accounts_toast_reset_sent"));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : t("accounts_toast_reset_failed");
      toast.error(msg);
    }
  };

  if (isLoading) {
    return (
      <div className="space-y-4 fade-in">
        <AdminPageHeader title={t("accounts_title")} description={t("sys_loading")} />
        <div className="py-12 text-center text-ink-500">{t("sys_loading_data")}</div>
      </div>
    );
  }

  if (error) {
    const msg =
      (error as { code?: string })?.code === "forbidden"
        ? t("sys_error_forbidden")
        : t("sys_error_load");
    return (
      <div className="space-y-4 fade-in">
        <AdminPageHeader title={t("accounts_title")} description={t("sys_error_title")} />
        <div className="py-12 text-center text-ink-500">{msg}</div>
      </div>
    );
  }

  return (
    <div className="space-y-4 fade-in">
      <AdminPageHeader
        title={t("accounts_title")}
        description={t("accounts_subtitle")}
        actions={
          <Button variant="dark" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            {t("create")}
          </Button>
        }
      />

      <div className="md-card-outlined md-card-flush">
        <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
          <ToolbarSearch
            value={search}
            onChange={setSearch}
            placeholder={t("accounts_search_placeholder")}
            clearLabel={t("list_toolbar_clear_search")}
          />
          <Select value={roleFilter} onValueChange={(v) => setRoleFilter(v as AdminAccountRole | "all")}>
            <SelectTrigger size="pill" aria-label={t("accounts_th_role")}>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("tab_all")}</SelectItem>
              {ROLES.map((r) => (
                <SelectItem key={r} value={r}>{ROLE_LABEL[r]}</SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select
            value={statusFilter}
            onValueChange={(v) => setStatusFilter(v as AdminAccountStatus | "all")}
          >
            <SelectTrigger size="pill" aria-label={t("accounts_th_status")}>
              <SelectValue placeholder={t("accounts_status_placeholder")} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("accounts_status_all")}</SelectItem>
              <SelectItem value="active">{t("status_label_active")}</SelectItem>
              <SelectItem value="deactivated">{t("status_label_inactive")}</SelectItem>
            </SelectContent>
          </Select>
          <ToolbarCount>{rows.length} {t("accounts_th_account").toLowerCase()}</ToolbarCount>
        </div>
        <DataTable
          surface="plain"
          rows={rows}
          rowKey={(a) => a.id}
          empty={t("accounts_empty")}
          columns={[
            {
              key: "account",
              header: t("accounts_th_account"),
              cell: (a) => (
                <TableItem
                  thumb={
                    <Avatar size="sm">
                      <AvatarFallback className="bg-brand-50 text-xs text-brand-700">{initials(a.name)}</AvatarFallback>
                    </Avatar>
                  }
                  title={a.name}
                  sub={a.email ?? "—"}
                />
              ),
            },
            {
              key: "role",
              header: t("accounts_th_role"),
              cell: (a) => (
                <Badge variant={ROLE_TONE[a.role]} className="font-mono">{ROLE_LABEL[a.role]}</Badge>
              ),
            },
            {
              key: "status",
              header: t("accounts_th_status"),
              cell: (a) => (
                <Badge variant={statusTone(a.status)}>
                  {a.status === "active" ? t("status_label_active") : t("status_label_inactive")}
                </Badge>
              ),
            },
            {
              key: "created",
              header: t("accounts_th_created"),
              cell: (a) =>
                a.created_at
                  ? new Date(a.created_at).toLocaleString(dateLocale, { day: "2-digit", month: "short", year: "numeric" })
                  : "—",
            },
          ]}
          rowActions={(a) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="icon-xs">
                  <MoreHorizontal className="size-4 text-ink-500" />
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem
                  onClick={() => {
                    setRoleChangeTarget(a);
                    setRoleChangeRole(a.role);
                    setRoleChangeSchoolId(a.school_id ?? "");
                  }}
                >
                  <Mail className="mr-2 size-4" />
                  {t("accounts_action_change_role")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => handleStatusToggle(a)}>
                  <Lock className="mr-2 size-4" />
                  {a.status === "active" ? t("accounts_action_deactivate") : t("accounts_action_activate")}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => handleResetPassword(a)}>
                  <Lock className="mr-2 size-4" />
                  {t("accounts_action_reset_password")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          )}
        />
      </div>

      {/* Create account dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle >{t("accounts_dialog_create_title")}</DialogTitle>
            <DialogDescription>{t("accounts_dialog_create_desc")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t("accounts_field_name")}</Label>
              <Input
                value={createForm.name}
                onChange={(e) => setCreateForm((f) => ({ ...f, name: e.target.value }))}
                placeholder={t("accounts_placeholder_full_name")}
              />
            </div>
            <div>
              <Label>{t("accounts_field_email")}</Label>
              <Input
                type="email"
                value={createForm.email}
                onChange={(e) => setCreateForm((f) => ({ ...f, email: e.target.value }))}
                placeholder={t("accounts_placeholder_email")}
              />
            </div>
            <div>
              <Label>{t("accounts_field_role")}</Label>
              <Select
                value={createForm.role}
                onValueChange={(v) =>
                  setCreateForm((f) => ({
                    ...f,
                    role: v as AdminAccountRole,
                    school_id: v === "admin_school" ? f.school_id : "",
                  }))
                }
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("accounts_placeholder_pick_role")} />
                </SelectTrigger>
                <SelectContent>
                  {ROLES.map((r) => (
                    <SelectItem key={r} value={r}>
                      {ROLE_LABEL[r]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {createForm.role === "admin_school" && (
              <SchoolFilterPicker
                value={createForm.school_id}
                onChange={(schoolId) => setCreateForm((f) => ({ ...f, school_id: schoolId }))}
                label={t("accounts_field_school")}
                allLabel={t("accounts_placeholder_pick_school")}
              />
            )}
            <div>
              <Label>{t("accounts_field_password")}</Label>
              <PasswordInput
                value={createForm.password}
                onChange={(e) => setCreateForm((f) => ({ ...f, password: e.target.value }))}
                placeholder={t("accounts_placeholder_min_8")}
              />
            </div>
          </div>
          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setCreateOpen(false)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleCreate} disabled={createAccount.isPending}>
              {createAccount.isPending ? t("saving") : t("create")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Role change dialog */}
      <Dialog
        open={roleChangeTarget !== null}
        onOpenChange={(open) => {
          if (!open) {
            setRoleChangeTarget(null);
            setRoleChangeSchoolId("");
          }
        }}
      >
        <DialogContent className="sm:max-w-[480px]">
          <DialogHeader>
            <DialogTitle >{t("accounts_dialog_role_title")}</DialogTitle>
            <DialogDescription>
              {roleChangeTarget ? `${t("accounts_dialog_role_desc_prefix")}${roleChangeTarget.name}` : ""}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div>
              <Label>{t("accounts_field_new_role")}</Label>
              <Select
                value={roleChangeRole}
                onValueChange={(v) => {
                  setRoleChangeRole(v as AdminAccountRole);
                  if (v !== "admin_school") setRoleChangeSchoolId("");
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("accounts_placeholder_pick_role")} />
                </SelectTrigger>
                <SelectContent>
                  {ROLES.map((r) => (
                    <SelectItem key={r} value={r}>
                      {ROLE_LABEL[r]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {roleChangeRole === "admin_school" && (
              <SchoolFilterPicker
                value={roleChangeSchoolId}
                onChange={setRoleChangeSchoolId}
                label={t("accounts_field_school")}
                allLabel={t("accounts_placeholder_pick_school")}
              />
            )}
          </div>
          <DialogFooter className="mt-4">
            <Button variant="outline" onClick={() => setRoleChangeTarget(null)}>
              {t("cancel")}
            </Button>
            <Button onClick={handleRoleChange} disabled={changeRole.isPending}>
              {changeRole.isPending ? t("saving") : t("accounts_save")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
