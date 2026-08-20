"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { ChevronRight, Calendar, Search } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ExamModal } from "@/components/admin/ExamModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { useExams } from "@/lib/hooks/admin-exams";
import { useTranslation } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import type { ExamListItem } from "@/lib/types";

const SEARCH_DEBOUNCE_MS = 300;
const STATUS_OPTIONS = ["", "draft", "published"] as const;

// Selling an exam (price/status/publish) is managed on the attached Product(s)
// via /admin/products — mirrors Course, which shows no status/price columns here.
function formatScheduled(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("id-ID", {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

export default function ExamPackagesPage() {
  const { t } = useTranslation();
  const router = useRouter();
  const role = useAuthStore((s) => s.user?.role);
  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<ExamListItem | null>(null);

  const [status, setStatus] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [q, setQ] = useState("");

  useEffect(() => {
    const id = setTimeout(() => setQ(searchInput.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(id);
  }, [searchInput]);

  const filters = useMemo(
    () => ({ q: q || undefined, status: status || undefined }),
    [q, status]
  );

  const { data, isLoading, isError, error } = useExams(filters);
  const items = data?.data ?? [];

  return (
    <div className="space-y-6 fade-in">
      <AdminPageHeader
        icon={Calendar}
        title={t("exam_packages_page_title")}
        description={t("exam_packages_page_description")}
        actions={
          role !== "admin_school" ? (
            <Button className="rounded-full" onClick={() => setShowCreate(true)}>
              {t("exam_packages_create")}
            </Button>
          ) : undefined
        }
      />

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <select
          data-slot="select"
          value={status}
          onChange={(e) => setStatus(e.target.value)}
          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
        >
          {STATUS_OPTIONS.map((s) => (
            <option key={s} value={s}>
              {s === "" ? t("tab_all") : s}
            </option>
          ))}
        </select>

        <div className="relative w-full sm:w-64">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("search")}
            className="pl-9"
          />
        </div>
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
          {error instanceof Error ? error.message : t("error_generic")}
        </div>
      )}

      {!isLoading && !isError && (
        items.length === 0 ? (
          <div className="md-card-outlined px-4 py-8 text-center text-ink-500">
            {t("exam_packages_empty")}
          </div>
        ) : (
          <div className="md-card-outlined divide-y">
            {items.map((exam) => (
              <div
                key={exam.id}
                data-testid="package-row"
                role="button"
                tabIndex={0}
                onClick={() => router.push(`/admin/exam/packages/${exam.id}`)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    router.push(`/admin/exam/packages/${exam.id}`);
                  }
                }}
                className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-surface-2"
              >
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-ink-900 truncate">{exam.title}</span>
                    {exam.is_free && (
                      <Badge variant="secondary">{t("exam_packages_modal_is_free")}</Badge>
                    )}
                  </div>
                  <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-ink-500">
                    <span>
                      {formatScheduled(exam.scheduled_at)}
                      {exam.scheduled_end_at && ` – ${formatScheduled(exam.scheduled_end_at)}`}
                    </span>
                    {exam.timer_mode && (
                      <>
                        <span aria-hidden="true">·</span>
                        <span>{exam.timer_mode}</span>
                      </>
                    )}
                    <span aria-hidden="true">·</span>
                    {/* ?? not || — a registration_count of 0 must still render */}
                    <span>
                      {t("exam_packages_registered_count").replace(
                        "{n}",
                        String(exam.registration_count ?? 0)
                      )}
                    </span>
                    <Badge variant={exam.status === "published" ? "default" : "secondary"}>
                      {exam.status ?? "draft"}
                    </Badge>
                  </div>
                </div>
                <ChevronRight className="size-4 shrink-0 text-ink-400" />
              </div>
            ))}
          </div>
        )
      )}

      <ExamModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSaved={() => setShowCreate(false)}
      />

      <ExamModal
        open={Boolean(editing)}
        exam={editing}
        onClose={() => setEditing(null)}
        onSaved={() => setEditing(null)}
      />
    </div>
  );
}