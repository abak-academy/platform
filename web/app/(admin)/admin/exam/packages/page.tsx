"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { ChevronRight, Calendar } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ExamModal } from "@/components/admin/ExamModal";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import { useExams } from "@/lib/hooks/admin-exams";
import { useTranslation } from "@/lib/i18n";
import { useAuthStore } from "@/stores/auth";
import type { ExamListItem } from "@/lib/types";

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

  const { data, isLoading, isError, error } = useExams();
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

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 5 }).map((_, i) => (
            <Skeleton key={i} className="h-12 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
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