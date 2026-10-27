"use client";

import { useRef, useState } from "react";
import { Handshake, Search, SlidersHorizontal, Upload, UserPlus, Users, X } from "lucide-react";
import { toast } from "sonner";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { NoAccess } from "@/components/admin/NoAccess";
import { SchoolMultiPicker, type PickedSchool } from "@/components/admin/SchoolMultiPicker";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { PasswordInput } from "@/components/ui/password-input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAdminAccounts } from "@/lib/hooks/admin-accounts";
import {
  useAdminAffiliator,
  useAdminAffiliators,
  useBulkCreateAffiliators,
  useCreateAffiliator,
  usePromoteAffiliator,
  useSetAffiliatorSchools,
  useSetAffiliatorStatus,
} from "@/lib/hooks/admin-affiliators";
import { useHasCapability } from "@/lib/hooks/use-capability";
import { affiliateOrderStatusLabel } from "@/lib/affiliate-order-status";
import { useTranslation } from "@/lib/i18n";
import { AFFILIATOR_CSV_HEADER, parseAffiliatorCsv } from "@/lib/affiliator-csv";
import { formatRupiah } from "@/lib/format";
import type { Affiliator, AffiliatorBulkResult } from "@/lib/types";

const ALL = "__all__";

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error && err.message ? err.message : fallback;
}

type Dialogs =
  | { kind: "create" }
  | { kind: "bulk" }
  | { kind: "promote" }
  | { kind: "schools"; id: string }
  | { kind: "detail"; id: string }
  | null;

export default function AdminAffiliatorsPage() {
  const { t } = useTranslation();
  const allowed = useHasCapability("system:admin");
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [dialog, setDialog] = useState<Dialogs>(null);
  const list = useAdminAffiliators(status || undefined);
  const setStatusMutation = useSetAffiliatorStatus();

  if (!allowed) return <NoAccess />;

  const needle = search.trim().toLowerCase();
  const rows = (list.data ?? []).filter((a) => !needle || a.name.toLowerCase().includes(needle) || (a.email ?? "").toLowerCase().includes(needle));
  const statusLabel = (s: string) => t(s === "active" ? "status_label_active" : "status_label_inactive");

  async function toggle(a: Affiliator) {
    const next = a.status === "active" ? "deactivated" : "active";
    if (next === "deactivated" && !confirm(t("aff_admin_confirm_deactivate"))) return;
    try {
      await setStatusMutation.mutateAsync({ id: a.id, status: next });
      toast.success(t("changes_saved"));
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  const columns: DataTableColumn<Affiliator>[] = [
    {
      key: "name",
      header: t("aff_register_name"),
      cell: (a) => (
        <div>
          <div className="font-medium">{a.name}</div>
          <div className="text-xs text-ink-600">{a.email}</div>
        </div>
      ),
    },
    { key: "schools", header: t("aff_th_school"), align: "right", cell: (a) => a.school_ids.length },
    { key: "students", header: t("aff_th_students"), align: "right", cell: (a) => a.student_count },
    { key: "orders", header: t("aff_th_orders"), align: "right", cell: (a) => a.order_count },
    { key: "paid", header: t("aff_th_amount"), align: "right", cell: (a) => formatRupiah(a.paid_amount) },
    {
      key: "status",
      header: t("th_status"),
      cell: (a) => (
        <Badge variant="outline" className={a.status === "active" ? "border-transparent bg-success-bg text-success" : undefined}>
          {statusLabel(a.status)}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (a) => (
        <div className="flex flex-wrap justify-end gap-2">
          <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "detail", id: a.id })}>
            {t("aff_admin_detail")}
          </Button>
          <Button size="sm" variant="outline" onClick={() => setDialog({ kind: "schools", id: a.id })}>
            {t("aff_admin_edit_schools")}
          </Button>
          <Button size="sm" variant={a.status === "active" ? "destructive" : "outline"} onClick={() => toggle(a)} disabled={setStatusMutation.isPending}>
            {a.status === "active" ? t("aff_admin_deactivate") : t("aff_admin_activate")}
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div className="fade-in space-y-6">
      <AdminPageHeader
        icon={Handshake}
        title={t("aff_admin_title")}
        description={t("aff_admin_subtitle")}
        actions={
          <>
            <Button variant="outline" onClick={() => setDialog({ kind: "promote" })}>
              <Users className="size-4" aria-hidden />
              {t("aff_admin_promote")}
            </Button>
            <Button variant="outline" onClick={() => setDialog({ kind: "bulk" })}>
              <Upload className="size-4" aria-hidden />
              {t("aff_bulk")}
            </Button>
            <Button onClick={() => setDialog({ kind: "create" })}>
              <UserPlus className="size-4" aria-hidden />
              {t("aff_admin_create")}
            </Button>
          </>
        }
      />

      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="relative w-full min-w-0 sm:w-auto sm:min-w-56 sm:flex-1">
            <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
            <Input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={t("aff_admin_search")} aria-label={t("aff_admin_search")} className="pr-10 pl-9" />
            {search && (
              <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => setSearch("")}>
                <X className="size-4" aria-hidden />
              </Button>
            )}
          </div>
          <Popover>
            <PopoverTrigger asChild>
              <Button variant="outline" className="h-9" aria-label={`${t("aff_filters")}${status ? " (1)" : ""}`}>
                <SlidersHorizontal className="size-4" aria-hidden />
                {t("aff_filters")}
                {status && <span className="tabular-nums">1</span>}
              </Button>
            </PopoverTrigger>
            <PopoverContent align="end" className="admin-shell w-[min(24rem,calc(100vw-2rem))]" aria-label={t("aff_admin_filter")}>
              <div className="space-y-3">
                <h2 className="font-semibold">{t("aff_admin_filter")}</h2>
                <span className="block text-sm font-medium">{t("th_status")}</span>
                <Select value={status || ALL} onValueChange={(v) => setStatus(v === ALL ? "" : v)}>
                  <SelectTrigger className="w-full" aria-label={t("th_status")}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={ALL}>{t("tab_all")}</SelectItem>
                    <SelectItem value="active">{statusLabel("active")}</SelectItem>
                    <SelectItem value="deactivated">{statusLabel("deactivated")}</SelectItem>
                  </SelectContent>
                </Select>
                <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
                  <Button variant="ghost" onClick={() => setStatus("")}>
                    {t("aff_reset_filters")}
                  </Button>
                  <PopoverClose asChild>
                    <Button>{t("aff_admin_show")}</Button>
                  </PopoverClose>
                </div>
              </div>
            </PopoverContent>
          </Popover>
        </div>
        {status && (
          <div className="flex flex-wrap items-center gap-2" role="group" aria-label={t("aff_active_filters")}>
            <span className="md-chip md-chip-primary gap-1">
              {`${t("th_status")}: ${statusLabel(status)}`}
              <Button variant="ghost" size="icon" className="size-8" aria-label={`${t("aff_remove_filter")}: ${statusLabel(status)}`} onClick={() => setStatus("")}>
                <X className="size-4" aria-hidden />
              </Button>
            </span>
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
        <DataTable columns={columns} rows={list.isLoading ? [] : rows} rowKey={(a) => a.id} empty={list.isLoading ? t("aff_loading") : t("aff_admin_empty")} />
      )}

      <CreateDialog open={dialog?.kind === "create"} onClose={() => setDialog(null)} />
      <BulkDialog open={dialog?.kind === "bulk"} onClose={() => setDialog(null)} />
      <PromoteDialog open={dialog?.kind === "promote"} onClose={() => setDialog(null)} />
      <SchoolsDialog id={dialog?.kind === "schools" ? dialog.id : null} onClose={() => setDialog(null)} />
      <DetailDialog id={dialog?.kind === "detail" ? dialog.id : null} onClose={() => setDialog(null)} />
    </div>
  );
}

function CreateDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateAffiliator();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [schools, setSchools] = useState<PickedSchool[]>([]);
  const ready = name.trim() !== "" && email.trim() !== "" && password.length >= 8 && schools.length > 0;

  function close() {
    setName("");
    setEmail("");
    setPassword("");
    setSchools([]);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!ready) return;
    try {
      await create.mutateAsync({ name: name.trim(), email: email.trim(), password, school_ids: schools.map((s) => s.id) });
      toast.success(t("aff_admin_created"));
      close();
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_create")}</DialogTitle>
            <DialogDescription>{t("aff_admin_create_hint")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="aff-new-name">{t("aff_register_name")}</Label>
              <Input id="aff-new-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="aff-new-email">{t("aff_admin_email")}</Label>
              <Input id="aff-new-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="aff-new-password">{t("aff_admin_password")}</Label>
              <PasswordInput id="aff-new-password" value={password} onChange={(e) => setPassword(e.target.value)} />
            </div>
            <div className="grid gap-2">
              <Label>{t("aff_admin_schools")}</Label>
              <SchoolMultiPicker value={schools} onChange={setSchools} />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("aff_cancel")}
            </Button>
            <Button type="submit" disabled={!ready || create.isPending}>
              {create.isPending ? t("aff_saving") : t("aff_admin_create")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function BulkDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const bulk = useBulkCreateAffiliators();
  const fileRef = useRef<HTMLInputElement | null>(null);
  const [schools, setSchools] = useState<PickedSchool[]>([]);
  const [results, setResults] = useState<AffiliatorBulkResult[] | null>(null);

  function close() {
    setSchools([]);
    setResults(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    const file = fileRef.current?.files?.[0];
    if (!file || schools.length === 0) return;
    const parsed = parseAffiliatorCsv(await file.text());
    if (parsed.error) {
      toast.error(t(parsed.error === "header" ? "aff_bulk_bad_header" : "aff_bulk_empty"));
      return;
    }
    const school_ids = schools.map((s) => s.id);
    try {
      const res = await bulk.mutateAsync(parsed.rows.map((r) => ({ ...r, school_ids })));
      setResults(res.results);
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  const okCount = results?.filter((r) => r.ok).length ?? 0;

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_bulk_title")}</DialogTitle>
            <DialogDescription>{t("aff_admin_bulk_hint").replace("{header}", AFFILIATOR_CSV_HEADER)}</DialogDescription>
          </DialogHeader>
          {results ? (
            <div className="space-y-3 py-4 text-sm">
              <p>{t("aff_admin_bulk_summary").replace("{ok}", String(okCount)).replace("{total}", String(results.length))}</p>
              <ul className="max-h-60 space-y-1 overflow-y-auto">
                {results.filter((r) => !r.ok).map((r) => (
                  <li key={r.row} className="text-destructive">
                    {`#${r.row} ${r.email}: ${r.error}`}
                  </li>
                ))}
              </ul>
            </div>
          ) : (
            <div className="grid gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="aff-bulk-file">{t("aff_admin_bulk_file")}</Label>
                <input id="aff-bulk-file" ref={fileRef} type="file" accept=".csv,text/csv" />
              </div>
              <div className="grid gap-2">
                <Label>{t("aff_admin_schools")}</Label>
                <SchoolMultiPicker value={schools} onChange={setSchools} />
              </div>
            </div>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {results ? t("aff_close") : t("aff_cancel")}
            </Button>
            {!results && (
              <Button type="submit" disabled={schools.length === 0 || bulk.isPending}>
                {bulk.isPending ? t("aff_saving") : t("aff_admin_bulk_run")}
              </Button>
            )}
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function PromoteDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { t } = useTranslation();
  const accounts = useAdminAccounts("admin_school", "active");
  const promote = usePromoteAffiliator();
  const [accountId, setAccountId] = useState("");
  const [schools, setSchools] = useState<PickedSchool[]>([]);

  function close() {
    setAccountId("");
    setSchools([]);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!accountId || schools.length === 0) return;
    try {
      await promote.mutateAsync({ id: accountId, school_ids: schools.map((s) => s.id) });
      toast.success(t("aff_admin_promoted"));
      close();
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_promote")}</DialogTitle>
            <DialogDescription>{t("aff_admin_promote_hint")}</DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="aff-promote-account">{t("aff_admin_school_admin")}</Label>
              <Select value={accountId} onValueChange={setAccountId}>
                <SelectTrigger id="aff-promote-account" className="w-full" aria-label={t("aff_admin_school_admin")}>
                  <SelectValue placeholder={t("aff_admin_choose_account")} />
                </SelectTrigger>
                <SelectContent>
                  {(accounts.data ?? []).map((a) => (
                    <SelectItem key={a.id} value={a.id}>
                      {`${a.name} (${a.email ?? "-"})`}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="grid gap-2">
              <Label>{t("aff_admin_schools")}</Label>
              <SchoolMultiPicker value={schools} onChange={setSchools} />
            </div>
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("aff_cancel")}
            </Button>
            <Button type="submit" disabled={!accountId || schools.length === 0 || promote.isPending}>
              {promote.isPending ? t("aff_saving") : t("aff_admin_promote")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function SchoolsDialog({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const detail = useAdminAffiliator(id);
  const save = useSetAffiliatorSchools();
  const [picked, setPicked] = useState<PickedSchool[] | null>(null);

  const current: PickedSchool[] = (detail.data?.affiliator.schools ?? []).map((s) => ({ id: s.id, name: s.name }));
  const schools = picked ?? current;

  function close() {
    setPicked(null);
    onClose();
  }

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!id || schools.length === 0) return;
    try {
      await save.mutateAsync({ id, school_ids: schools.map((s) => s.id) });
      toast.success(t("changes_saved"));
      close();
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  return (
    <Dialog open={id !== null} onOpenChange={(o) => !o && close()}>
      <DialogContent className="sm:max-w-lg">
        <form onSubmit={submit}>
          <DialogHeader>
            <DialogTitle>{t("aff_admin_edit_schools")}</DialogTitle>
            <DialogDescription>{detail.data?.affiliator.name}</DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <SchoolMultiPicker value={schools} onChange={setPicked} disabled={detail.isLoading} />
          </div>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={close}>
              {t("aff_cancel")}
            </Button>
            <Button type="submit" disabled={schools.length === 0 || save.isPending || detail.isLoading}>
              {save.isPending ? t("aff_saving") : t("aff_save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function DetailDialog({ id, onClose }: { id: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const detail = useAdminAffiliator(id);
  const d = detail.data;

  return (
    <Dialog open={id !== null} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{d?.affiliator.name ?? t("aff_admin_detail")}</DialogTitle>
          <DialogDescription>{d?.affiliator.email}</DialogDescription>
        </DialogHeader>
        {detail.isLoading || !d ? (
          <p className="py-6 text-sm text-ink-500">{t("aff_loading")}</p>
        ) : (
          <div className="space-y-4 py-2 text-sm">
            <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3">
              <div><dt className="text-ink-600">{t("aff_stat_paid_orders")}</dt><dd className="font-semibold">{d.totals.paid_orders}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_units")}</dt><dd className="font-semibold">{d.totals.paid_units}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_paid_amount")}</dt><dd className="font-semibold">{formatRupiah(d.totals.paid_amount)}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_discount")}</dt><dd className="font-semibold">{formatRupiah(d.totals.discount_total)}</dd></div>
              <div><dt className="text-ink-600">{t("aff_stat_pending")}</dt><dd className="font-semibold">{d.totals.pending_orders}</dd></div>
              <div><dt className="text-ink-600">{t("aff_th_students")}</dt><dd className="font-semibold">{d.affiliator.student_count}</dd></div>
            </dl>
            <div>
              <h3 className="font-semibold">{t("aff_section_schools")}</h3>
              <p>{(d.affiliator.schools ?? []).map((s) => s.name).join(", ") || "-"}</p>
            </div>
            <div>
              <h3 className="font-semibold">{t("aff_admin_recent_orders")}</h3>
              <ul className="space-y-1">
                {d.recent_orders.map((o) => (
                  <li key={o.id} className="flex justify-between gap-3">
                    <span>{`${o.product_name} · ${o.participant_count}`}</span>
                    <span>{`${formatRupiah(o.total)} · ${affiliateOrderStatusLabel(o.status, t)}`}</span>
                  </li>
                ))}
                {d.recent_orders.length === 0 && <li className="text-ink-500">{t("aff_orders_empty")}</li>}
              </ul>
            </div>
          </div>
        )}
        <DialogFooter>
          <Button type="button" onClick={onClose}>
            {t("aff_close")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
