"use client";

import { useState } from "react";
import { ClipboardList, Pencil, Plus, ShieldCheck, Trash2, type LucideIcon } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { EmptyState } from "@/components/ui/empty-state";
import { cn } from "@/lib/utils";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ToolbarCount, ToolbarSearch } from "@/components/admin/ListToolbar";
import { useAdminAuditLog } from "@/lib/hooks/admin-audit";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import type { AuditLogEntry } from "@/lib/types";

function actionLabel(action: string): string {
  const verb = action.split(".").pop() ?? action;
  return verb.replace(/_/g, " ");
}

function actionIcon(action: string): LucideIcon {
  if (action.includes("create") || action.includes("add")) return Plus;
  if (action.includes("delete") || action.includes("remove")) return Trash2;
  if (action.includes("change") || action.includes("update")) return Pencil;
  return ShieldCheck;
}

function actionNode(action: string): string {
  if (action.includes("create") || action.includes("add")) return "bg-teal-600 text-white";
  if (action.includes("delete") || action.includes("remove")) return "bg-danger-50 text-danger-600";
  return "bg-surface-sunken text-ink-700";
}

export default function SystemAuditPage() {
  const { t, lang } = useTranslation();
  const [search, setSearch] = useState("");
  // Search is a server filter, so it must be debounced — otherwise every
  // keystroke fires a new request.
  const debouncedSearch = useDebouncedValue(search);
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const { data: entries = [], isLoading, error } = useAdminAuditLog(
    debouncedSearch.trim() ? { q: debouncedSearch } : undefined
  );

  if (isLoading) {
    return (
      <div className="space-y-4 fade-in">
        <AdminPageHeader title={t("audit_title")} description={t("sys_loading")} />
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
        <AdminPageHeader
          title={t("audit_title")}
          description={t("sys_error_title")}
        />
        <div className="py-12 text-center text-ink-500">{msg}</div>
      </div>
    );
  }

  return (
    <div className="space-y-4 fade-in">
      <AdminPageHeader title={t("audit_title")} description={t("audit_subtitle")} />

      <div className="md-card-outlined md-card-flush">
        <div className="flex flex-wrap items-center gap-2.5 border-b border-line px-4 py-3.5">
          <ToolbarSearch
            value={search}
            onChange={setSearch}
            placeholder={t("audit_search_placeholder")}
            clearLabel={t("list_toolbar_clear_search")}
          />
          <ToolbarCount>{t("adm_logs_count").replace("{n}", String(entries.length))}</ToolbarCount>
        </div>
        {entries.length === 0 ? (
          <EmptyState icon={ClipboardList} title={t("audit_empty")} />
        ) : (
          <ol className="p-6">
            {entries.map((a, i) => {
              const Icon = actionIcon(a.action);
              return (
                <li key={a.id} className="relative grid grid-cols-[34px_1fr_auto] gap-3 pb-[18px] last:pb-0">
                  {i < entries.length - 1 && (
                    <span aria-hidden className="absolute top-[34px] bottom-0 left-[16px] w-0.5 bg-line" />
                  )}
                  <span className={cn("z-10 grid size-[34px] place-items-center rounded-xl", actionNode(a.action))}>
                    <Icon className="size-4" aria-hidden />
                  </span>
                  <div className="min-w-0">
                    <p className="text-sm font-bold text-ink-900">
                      {a.actor_name ?? "System"}{" "}
                      <span className="font-semibold text-ink-700 capitalize">{actionLabel(a.action)}</span>
                    </p>
                    <p className="text-[12.5px] text-ink-600">
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
      </div>
    </div>
  );
}
