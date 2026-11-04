"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Calendar, Search, X } from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ExamModal } from "@/components/admin/ExamModal";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { useExams } from "@/lib/hooks/admin-exams";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { examModeLabel } from "@/lib/exam-mode";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { accessWindow } from "@/lib/exam-schedule";
import { useAuthStore } from "@/stores/auth";
import type { ExamListItem } from "@/lib/types";

function Sub({ children }: { children: React.ReactNode }) {
  return <div className="mt-0.5 text-xs text-ink-500">{children}</div>;
}

function plural(t: (key: I18nKey) => string, key: "exam_admin_list_sections" | "exam_admin_list_questions", n: number) {
  return t(n === 1 ? `${key}_one` : key).replace("{n}", String(n));
}

export default function ExamPackagesPage() {
  const { t, lang } = useTranslation();
  const router = useRouter();
  const role = useAuthStore((s) => s.user?.role);
  const [showCreate, setShowCreate] = useState(false);
  const [editing, setEditing] = useState<ExamListItem | null>(null);

  const [searchInput, setSearchInput] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  const q = useDebouncedValue(searchInput).trim();
  const [cursor, setCursor] = useState<string | undefined>();
  const [nextCursor, setNextCursor] = useState<string | undefined>();
  const [items, setItems] = useState<ExamListItem[]>([]);

  // A debounced search commit must restart pagination at page 1.
  useEffect(() => {
    setItems([]);
    setCursor(undefined);
    setNextCursor(undefined);
  }, [q]);

  const filters = useMemo(
    () => ({ cursor, limit: 20, q: q || undefined }),
    [cursor, q]
  );

  const { data, isLoading, isFetching, isError, error } = useExams(filters);

  useEffect(() => {
    if (!data) return;
    setItems((previous) => {
      if (!cursor) return data.data;
      const ids = new Set(previous.map((exam) => exam.id));
      return [...previous, ...data.data.filter((exam) => !ids.has(exam.id))];
    });
    setNextCursor(data.next_cursor);
  }, [cursor, data, q]);

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

      <div className="flex items-center gap-3">
        <div className="relative w-full min-w-0 flex-1">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-ink-400" />
          <Input
            ref={searchRef}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            placeholder={t("exam_packages_search_placeholder")}
            aria-label={t("exam_packages_search_placeholder")}
            className="px-9"
          />
          {searchInput && (
            <button
              type="button"
              aria-label={t("exam_packages_search_clear")}
              onClick={() => {
                setSearchInput("");
                searchRef.current?.focus();
              }}
              className="absolute right-2 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded-full text-ink-500 hover:bg-muted hover:text-ink-900"
            >
              <X className="size-4" />
            </button>
          )}
        </div>
        {!isLoading && !isError && (
          <span className="shrink-0 text-sm text-ink-500" aria-live="polite">
            {t("exam_packages_count").replace("{n}", `${items.length}${nextCursor ? "+" : ""}`)}
          </span>
        )}
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
          <div className="overflow-x-auto md-card-outlined">
            <table className="w-full text-sm">
              <thead className="bg-muted">
                <tr>
                  <th className="min-w-[14rem] px-4 py-3 text-left font-medium">{t("exam_admin_col_exam")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("exam_admin_col_composition")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("exam_admin_col_delivery")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("exam_admin_col_time")}</th>
                  <th className="px-4 py-3 text-left font-medium">{t("exam_admin_col_access")}</th>
                </tr>
              </thead>
              <tbody>
                {items.map((exam) => {
                  const composable = exam.format_version === 2;
                  const perSection = composable && exam.timer_mode === "per_test";
                  const access = accessWindow(exam.scheduled_at ?? null, exam.scheduled_end_at ?? null, lang);
                  const open = () => router.push(`/admin/exam/packages/${exam.id}`);
                  return (
                    <tr
                      key={exam.id}
                      data-testid="package-row"
                      tabIndex={0}
                      onClick={open}
                      onKeyDown={(e) => {
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          open();
                        }
                      }}
                      className="cursor-pointer border-t align-top transition-colors hover:bg-muted/40"
                    >
                      <td className="px-4 py-3">
                        <div className="font-medium text-ink-900">{exam.title}</div>
                        {/* ?? not || — a registration_count of 0 must still render */}
                        <Sub>
                          {t("exam_packages_registered_count").replace(
                            "{n}",
                            String(exam.registration_count ?? 0)
                          )}
                        </Sub>
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        {composable ? (
                          <>
                            <div>{plural(t, "exam_admin_list_sections", exam.section_count ?? 0)}</div>
                            <Sub>{plural(t, "exam_admin_list_questions", exam.question_count ?? 0)}</Sub>
                          </>
                        ) : (
                          <>
                            <div>{t("exam_admin_legacy_format")}</div>
                            <Sub>{t("exam_admin_legacy_settings")}</Sub>
                          </>
                        )}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        {composable ? (
                          <>
                            <div>
                              {t(
                                exam.navigation_policy === "sequential"
                                  ? "exam_admin_list_nav_sequential"
                                  : "exam_admin_list_nav_free"
                              )}
                            </div>
                            <Sub>{t("exam_admin_presentation_per_section")}</Sub>
                          </>
                        ) : (
                          <>
                            <div>{examModeLabel(exam.mode, t)}</div>
                            <Sub>{t("exam_admin_legacy_format")}</Sub>
                          </>
                        )}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        {perSection ? (
                          <>
                            <div>{t("exam_admin_time_per_section")}</div>
                            {exam.sections_duration_minutes != null && (
                              <Sub>
                                {t("exam_admin_time_sections_total").replace(
                                  "{n}",
                                  String(exam.sections_duration_minutes)
                                )}
                              </Sub>
                            )}
                          </>
                        ) : exam.duration_minutes == null ? (
                          "—"
                        ) : (
                          <>
                            <div>
                              {t(composable ? "exam_admin_time_overall" : "exam_admin_time_legacy").replace(
                                "{n}",
                                String(exam.duration_minutes)
                              )}
                            </div>
                            <Sub>{t(composable ? "exam_admin_time_overall_hint" : "exam_admin_time_legacy_hint")}</Sub>
                          </>
                        )}
                      </td>
                      <td className="whitespace-nowrap px-4 py-3">
                        {access ? (
                          <>
                            <div>{`${access.from.date}, ${access.from.time} WIB`}</div>
                            <Sub>
                              {access.to
                                ? t("exam_admin_access_until").replace(
                                    "{date}",
                                    `${access.to.date}, ${access.to.time} WIB`
                                  )
                                : t("exam_admin_access_no_end")}
                            </Sub>
                          </>
                        ) : (
                          t("exam_admin_access_none")
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )
      )}

      {!isLoading && !isError && (
        <p className="text-xs text-ink-500">{t("exam_admin_products_footnote")}</p>
      )}

      {nextCursor && !isError && (
        <div className="text-center">
          <Button
            variant="outline"
            onClick={() => setCursor(nextCursor)}
            disabled={isFetching}
          >
            {isFetching ? t("sys_loading") : t("load_more")}
          </Button>
        </div>
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
