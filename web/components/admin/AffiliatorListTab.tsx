"use client";

import { useState } from "react";
import { Search, SlidersHorizontal, UserPlus, X } from "lucide-react";
import { toast } from "sonner";
import { AffiliatorCreateDialog } from "@/components/admin/AffiliatorCreateDialog";
import { TablePager } from "@/components/admin/TablePager";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Popover, PopoverClose, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useAdminAffiliators, useSetAffiliatorStatus } from "@/lib/hooks/admin-affiliators";
import { useCursorPaging } from "@/lib/hooks/use-cursor-paging";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { formatRupiah } from "@/lib/format";
import { useTranslation } from "@/lib/i18n";
import type { Affiliator } from "@/lib/types";

const ALL = "__all__";

// The affiliator list: search, status filter, pagination, create (single or
// CSV) and the one row action that matters here, activate or deactivate.
export function AffiliatorListTab() {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [limit, setLimit] = useState(20);
  const [createOpen, setCreateOpen] = useState(false);
  const q = useDebouncedValue(search).trim();
  const paging = useCursorPaging([q, status, limit].join("|"));
  const list = useAdminAffiliators({ status: status || undefined, q: q || undefined, cursor: paging.cursor, limit });
  const setStatusMutation = useSetAffiliatorStatus();
  const rows = list.data?.data ?? [];
  const statusLabel = (s: string) => t(s === "active" ? "status_label_active" : "status_label_inactive");

  async function toggle(a: Affiliator) {
    const next = a.status === "active" ? "deactivated" : "active";
    if (next === "deactivated" && !confirm(t("aff_admin_confirm_deactivate"))) return;
    try {
      await setStatusMutation.mutateAsync({ id: a.id, status: next });
      toast.success(t("changes_saved"));
    } catch (err) {
      toast.error(err instanceof Error && err.message ? err.message : t("error_generic"));
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
        <Button size="sm" className="w-28" variant={a.status === "active" ? "destructive" : "outline"} onClick={() => toggle(a)} disabled={setStatusMutation.isPending}>
          {a.status === "active" ? t("aff_admin_deactivate") : t("aff_admin_activate")}
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
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
          <Button onClick={() => setCreateOpen(true)}>
            <UserPlus className="size-4" aria-hidden />
            {t("aff_admin_create")}
          </Button>
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
        <DataTable
          columns={columns}
          rows={list.isLoading ? [] : rows}
          rowKey={(a) => a.id}
          empty={list.isLoading ? t("aff_loading") : t("aff_admin_empty")}
          footer={
            <TablePager
              total={list.data?.total ?? 0}
              offset={paging.page * limit}
              count={rows.length}
              limit={limit}
              onLimitChange={setLimit}
              canPrev={paging.canPrev}
              canNext={Boolean(list.data?.next_cursor)}
              onPrev={paging.prev}
              onNext={() => list.data?.next_cursor && paging.next(list.data.next_cursor)}
            />
          }
        />
      )}

      <AffiliatorCreateDialog open={createOpen} onClose={() => setCreateOpen(false)} />
    </div>
  );
}
