"use client";

import { useMemo, useState } from "react";
import { ClipboardList, Pencil, Plus, ShieldCheck, Trash2, type LucideIcon } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import {
  ActiveChip,
  FILTER_PANEL_CLASS,
  FilterGrid,
  FilterPanelFooter,
  FilterSelect,
  ToolbarFilterButton,
  ToolbarSearch,
  ToolbarSort,
} from "@/components/admin/ListToolbar";
import { Button } from "@/components/ui/button";
import { Pagination } from "@/components/affiliate/Pagination";
import { TableFooter } from "@/components/ui/table-footer";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { useAdminAuditLog } from "@/lib/hooks/admin-audit";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import type { AuditLogEntry } from "@/lib/types";

const PAGE_SIZE = 20;

function actionLabel(action: string): string {
  const verb = action.split(".").pop() ?? action;
  return verb.replace(/_/g, " ");
}

function actionTone(action: string): string {
  if (action.includes("delete") || action.includes("remove")) return "bg-danger-600 text-on-danger";
  if (action.includes("change") || action.includes("update")) return "bg-brand-600 text-on-brand";
  return "bg-teal-600 text-white";
}

function actionIcon(action: string): LucideIcon {
  if (action.includes("create") || action.includes("add")) return Plus;
  if (action.includes("delete") || action.includes("remove")) return Trash2;
  if (action.includes("change") || action.includes("update")) return Pencil;
  return ShieldCheck;
}


export default function SystemAuditPage() {
  const { t, lang } = useTranslation();
  const [search, setSearch] = useState("");
  // Search is a server filter, so it must be debounced — otherwise every
  // keystroke fires a new request.
  const debouncedSearch = useDebouncedValue(search);
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const [actor, setActor] = useState("");
  const [page, setPage] = useState(1);

  const { data, isLoading, error } = useAdminAuditLog({
    q: debouncedSearch.trim() || undefined,
    page,
    limit: PAGE_SIZE,
  });
  const loaded = data?.data ?? [];
  const total = data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  // Actor is not a server filter yet, so it narrows the current page only.
  const entries = useMemo(
    () => (actor ? loaded.filter((a) => (actor === "sys") === !a.actor_name) : loaded),
    [loaded, actor]
  );
  const actorOptions = [
    { value: "sys", label: t("audit_actor_system") },
    { value: "adm", label: t("audit_actor_admin") },
  ];
  const actorLabel = actorOptions.find((o) => o.value === actor)?.label;

  if (isLoading) {
    return (
      <div className="flex flex-col gap-5 fade-in">
        <AdminPageHeader title={t("audit_title")} description={t("sys_loading")} />
        <div className="py-12 text-center text-ink-600">{t("sys_loading_data")}</div>
      </div>
    );
  }

  if (error) {
    const msg =
      (error as { code?: string })?.code === "forbidden"
        ? t("sys_error_forbidden")
        : t("sys_error_load");
    return (
      <div className="flex flex-col gap-5 fade-in">
        <AdminPageHeader
          title={t("audit_title")}
          description={t("sys_error_title")}
        />
        <div className="py-12 text-center text-ink-600">{msg}</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader title={t("audit_title")} description={t("audit_subtitle")} />

      <div className="md-card-outlined md-card-flush">
        <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
          <ToolbarSearch
            value={search}
            onChange={(v) => {
              setSearch(v);
              setPage(1);
            }}
            placeholder={t("audit_search_placeholder")}
            clearLabel={t("list_toolbar_clear_search")}
          />
          <Popover>
            <PopoverTrigger asChild>
              <ToolbarFilterButton label={t("list_toolbar_filter")} count={actor ? 1 : 0} />
            </PopoverTrigger>
            <PopoverContent align="end" className={FILTER_PANEL_CLASS} aria-label={t("list_toolbar_filter")}>
              <div className="flex flex-col gap-3.5">
                <FilterGrid>
                  <FilterSelect label={t("audit_filter_actor")} value={actor} options={actorOptions} onChange={setActor} />
                </FilterGrid>
                <FilterPanelFooter
                  onReset={() => setActor("")}
                  showLabel={t("list_toolbar_show_n").replace("{n}", String(entries.length))}
                />
              </div>
            </PopoverContent>
          </Popover>
          <ToolbarSort
            value="new"
            onChange={() => {}}
            label={t("list_toolbar_sort")}
            options={[
              { value: "new", label: t("audit_sort_newest") },
              { value: "old", label: t("audit_sort_oldest"), disabled: true },
            ]}
          />
        </div>
        <p className="border-b border-line px-4 py-2 text-xs text-ink-600">{t("audit_sort_oldest_locked")}</p>
        {actor && (
          <div className="flex flex-wrap items-center gap-1.5 border-b border-line px-4 py-2.5" role="group" aria-label={t("list_toolbar_active")}>
            <ActiveChip
              label={`${t("audit_filter_actor")}: ${actorLabel}`}
              removeLabel={`${t("list_toolbar_remove")}: ${actorLabel}`}
              onRemove={() => setActor("")}
            />
            <Button variant="ghost" size="sm" onClick={() => setActor("")}>{t("list_toolbar_clear_all")}</Button>
          </div>
        )}
        {entries.length === 0 ? (
          <EmptyState icon={ClipboardList} title={t("audit_empty")} />
        ) : (
          <ol className="p-6">
            {entries.map((a, i) => {
              const Icon = actionIcon(a.action);
              return (
                <li key={a.id} className="relative grid grid-cols-[34px_1fr_auto] gap-3 pb-[18px] last:pb-0">
                  {i < entries.length - 1 && (
                    <span aria-hidden className="absolute top-[34px] bottom-0 left-[16px] w-0.5 bg-teal-600" />
                  )}
                  <span className={`z-10 grid size-[34px] place-items-center rounded-[12px] ${actionTone(a.action)}`}>
                    <Icon className="size-4" aria-hidden />
                  </span>
                  <div className="min-w-0">
                    <p className="font-[family-name:var(--font-display)] text-sm font-bold text-ink-900">
                      {a.actor_name ?? "System"}{" "}
                      <span className="font-semibold text-ink-700 capitalize">{actionLabel(a.action)}</span>
                    </p>
                    <p className="mt-0.5 text-[12.5px] text-ink-600">
                      {a.target_type} · {a.target_id}
                      {a.actor_email && ` · ${a.actor_email}`}
                    </p>
                  </div>
                  <time dateTime={a.created_at} className="text-xs whitespace-nowrap text-ink-600">
                    {new Date(a.created_at).toLocaleString(dateLocale, {
                      day: "2-digit",
                      month: "short",
                      year: "numeric",
                      hour: "2-digit",
                      minute: "2-digit",
                    })}
                  </time>
                </li>
              );
            })}
          </ol>
        )}
        {total > 0 && (
          <TableFooter
            summary={
              actor || loaded.length === 0
                ? ""
                : t("aff_range_summary")
                    .replace("{from}", String((page - 1) * PAGE_SIZE + 1))
                    .replace("{to}", String((page - 1) * PAGE_SIZE + loaded.length))
                    .replace("{total}", String(total))
            }
          >
            <Pagination page={page} pages={pages} onPageChange={setPage} />
          </TableFooter>
        )}
      </div>
    </div>
  );
}
