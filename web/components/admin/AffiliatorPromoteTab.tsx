"use client";

import { useState } from "react";
import { Info, Search, X } from "lucide-react";
import { AffiliatorPromoteDialog, AffiliatorSchoolsDialog } from "@/components/admin/AffiliatorDialogs";
import { TablePager } from "@/components/admin/TablePager";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { useAdminAffiliators, usePromotableAdmins } from "@/lib/hooks/admin-affiliators";
import { useCursorPaging } from "@/lib/hooks/use-cursor-paging";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { useTranslation } from "@/lib/i18n";
import type { Affiliator, PromotableAdmin } from "@/lib/types";

type View = "admins" | "affiliators";

function SearchBox({ value, onChange, label }: { value: string; onChange: (v: string) => void; label: string }) {
  const { t } = useTranslation();
  return (
    <div className="relative w-full min-w-0 sm:max-w-md">
      <Search className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-ink-600" aria-hidden />
      <Input type="search" value={value} onChange={(e) => onChange(e.target.value)} placeholder={label} aria-label={label} className="pr-10 pl-9" />
      {value && (
        <Button type="button" variant="ghost" size="icon" className="absolute top-0 right-0 size-9" aria-label={t("aff_clear_search")} onClick={() => onChange("")}>
          <X className="size-4" aria-hidden />
        </Button>
      )}
    </div>
  );
}

function LoadFailed({ onRetry }: { onRetry: () => void }) {
  const { t } = useTranslation();
  return (
    <div className="md-card-outlined py-12 text-center text-ink-500">
      <p className="mb-4">{t("dash_load_failed")}</p>
      <button type="button" className="md-btn-tonal" onClick={onRetry}>
        {t("admin_home_reload")}
      </button>
    </div>
  );
}

function AdminsTable() {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [limit, setLimit] = useState(20);
  const [target, setTarget] = useState<PromotableAdmin | null>(null);
  const q = useDebouncedValue(search).trim();
  const paging = useCursorPaging([q, limit].join("|"));
  const list = usePromotableAdmins({ q: q || undefined, cursor: paging.cursor, limit });
  const rows = list.data?.data ?? [];

  const columns: DataTableColumn<PromotableAdmin>[] = [
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
    { key: "school", header: t("aff_th_school"), cell: (a) => a.school_name ?? "-" },
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (a) => (
        <Button size="sm" variant="outline" onClick={() => setTarget(a)}>
          {t("aff_admin_promote")}
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SearchBox value={search} onChange={setSearch} label={t("aff_promote_search")} />
      {list.isError ? (
        <LoadFailed onRetry={() => list.refetch()} />
      ) : (
        <DataTable
          columns={columns}
          rows={list.isLoading ? [] : rows}
          rowKey={(a) => a.id}
          empty={list.isLoading ? t("aff_loading") : t("aff_promote_empty")}
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
      <AffiliatorPromoteDialog admin={target} onClose={() => setTarget(null)} />
    </div>
  );
}

function AffiliatorsTable() {
  const { t } = useTranslation();
  const [search, setSearch] = useState("");
  const [limit, setLimit] = useState(20);
  const [target, setTarget] = useState<string | null>(null);
  const q = useDebouncedValue(search).trim();
  const paging = useCursorPaging([q, limit].join("|"));
  const list = useAdminAffiliators({ q: q || undefined, cursor: paging.cursor, limit });
  const rows = list.data?.data ?? [];

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
    {
      key: "actions",
      header: t("th_actions"),
      align: "right",
      cell: (a) => (
        <Button size="sm" variant="outline" onClick={() => setTarget(a.id)}>
          {t("aff_admin_edit_schools")}
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SearchBox value={search} onChange={setSearch} label={t("aff_admin_search")} />
      {list.isError ? (
        <LoadFailed onRetry={() => list.refetch()} />
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
      <AffiliatorSchoolsDialog id={target} onClose={() => setTarget(null)} />
    </div>
  );
}

// Promote school admins to affiliators, and change the schools of the ones that
// already are. Promotion cannot be undone, which the tab says up front.
export function AffiliatorPromoteTab() {
  const { t } = useTranslation();
  const [view, setView] = useState<View>("admins");

  return (
    <div className="space-y-4">
      <div role="note" className="flex items-start gap-3 rounded-[12px] border border-line bg-surface-2 px-4 py-3 text-sm">
        <Info size={18} className="mt-0.5 shrink-0" aria-hidden />
        <p>{t("aff_promote_tab_note")}</p>
      </div>

      <div role="radiogroup" aria-label={t("aff_promote_view")} className="flex w-fit overflow-hidden rounded-[10px] border border-line">
        {(["admins", "affiliators"] as const).map((v) => (
          <button
            key={v}
            type="button"
            role="radio"
            aria-checked={view === v}
            onClick={() => setView(v)}
            className="border-r border-line px-4 py-1.5 text-sm last:border-r-0"
            style={view === v ? { backgroundColor: "var(--md-sys-color-primary)", color: "var(--md-sys-color-on-primary)", fontWeight: 600 } : undefined}
          >
            {t(v === "admins" ? "aff_promote_view_admins" : "aff_promote_view_affiliators")}
          </button>
        ))}
      </div>

      {view === "admins" ? <AdminsTable /> : <AffiliatorsTable />}
    </div>
  );
}
