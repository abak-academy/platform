"use client";

import { useEffect, useRef, useState } from "react";
import { Download, Eye, Loader2 } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Skeleton } from "@/components/ui/skeleton";
import { useAssessment, useAssessmentAttempts } from "@/lib/hooks/admin-assessment";
import { useAdminResultDetail, exportAdminResults } from "@/lib/hooks/admin-results";
import { useSchools } from "@/lib/hooks/students";
import type { AssessmentRow, AssessmentAttempt } from "@/lib/types";

interface AssessmentTabProps {
  examId: string;
}

// Radix Select forbids an empty-string item value, so "every school" needs its
// own sentinel; it maps back to "" (no school_id param, meaning "all schools").
const ALL_SCHOOLS_VALUE = "_all_";

function statusBadgeClass(status: string): string {
  if (status === "completed") return "bg-success-bg text-success";
  if (status === "in_progress") return "bg-warning-bg text-warning";
  return "bg-surface-2 text-ink-500";
}

export function AssessmentTab({ examId }: AssessmentTabProps) {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const { data: schoolsData } = useSchools(true);
  const [selectedSchoolId, setSelectedSchoolId] = useState<string>("");
  const [search, setSearch] = useState("");

  const [accumulated, setAccumulated] = useState<AssessmentRow[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);

  const filterKey = `${examId}:${search}:${selectedSchoolId}`;
  const filterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      filterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useAssessment({
    examId,
    q: search || undefined,
    schoolId: selectedSchoolId || undefined,
    cursor: activeCursor,
    limit: 20,
    enabled: Boolean(examId),
  });

  useEffect(() => {
    if (!query.data) return;
    if (filterKey !== filterKeyRef.current) return;

    setAccumulated((prev) => {
      const rows = query.data!.data ?? [];
      if (activeCursor === undefined) return rows;
      const ids = new Set(prev.map((r) => r.registration_id));
      const fresh = rows.filter((r) => !ids.has(r.registration_id));
      return [...prev, ...fresh];
    });
    setNextCursor(query.data.next_cursor || undefined);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, filterKey]);

  const handleLoadMore = () => {
    if (nextCursor) setActiveCursor(nextCursor);
  };

  const [exporting, setExporting] = useState(false);
  const handleExport = async () => {
    if (!examId) return;
    setExporting(true);
    try {
      await exportAdminResults(examId, selectedSchoolId || undefined);
    } catch {
      // Export errors handled silently — the CSV download is best-effort
    } finally {
      setExporting(false);
    }
  };

  const [selectedRegistrationId, setSelectedRegistrationId] = useState<string>("");
  const selectedRow = accumulated.find((r) => r.registration_id === selectedRegistrationId);

  const summary = query.data?.summary;

  const columns: DataTableColumn<AssessmentRow>[] = [
    {
      key: "name",
      header: t("th_name"),
      cell: (row) => (
        <div>
          <div className="font-medium text-ink-900">{row.student_name}</div>
          <div className="text-xs text-ink-500">{row.username ?? "-"}</div>
        </div>
      ),
    },
    {
      key: "school",
      header: t("schools_th_school"),
      cell: (row) => <span className="text-xs text-ink-600">{row.school_name || "-"}</span>,
    },
    {
      key: "status",
      header: t("assessment_col_status"),
      cell: (row) => (
        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${statusBadgeClass(row.status)}`}>
          {t(`assessment_status_${row.status}` as const)}
        </span>
      ),
    },
    {
      key: "rank",
      header: t("admin_exam_leaderboard_col_rank"),
      cell: (row) => <span className="text-xs text-ink-600">{row.rank ?? "-"}</span>,
    },
    {
      key: "score",
      header: t("school_reports_col_score"),
      cell: (row) => <span className="text-xs text-ink-600">{row.score ?? "-"}</span>,
    },
    {
      key: "attempts",
      header: t("assessment_col_attempts"),
      cell: (row) => <span className="text-xs text-ink-600">{row.attempts_count}</span>,
    },
    {
      key: "violations",
      header: t("assessment_col_violations"),
      cell: (row) => <span className="text-xs text-ink-600">{row.latest_violations}</span>,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      cell: (row) => (
        <Button
          variant="ghost"
          size="icon"
          aria-label={t("action_view")}
          onClick={() => setSelectedRegistrationId(row.registration_id)}
        >
          <Eye className="size-4" />
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      {summary && (
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <div className="rounded-lg border p-4">
            <div className="text-label text-sm text-ink-500">{t("assessment_summary_total_registered")}</div>
            <div className="mt-1 text-2xl font-bold">{summary.total_registered}</div>
          </div>
          <div className="rounded-lg border p-4">
            <div className="text-label text-sm text-ink-500">{t("assessment_summary_completion_rate")}</div>
            <div className="mt-1 text-2xl font-bold">{Math.round(summary.completion_rate * 100)}%</div>
          </div>
          <div className="rounded-lg border p-4">
            <div className="text-label text-sm text-ink-500">{t("admin_exam_analytics_average_score")}</div>
            <div className="mt-1 text-2xl font-bold">{summary.average_score.toFixed(1)}</div>
          </div>
          <div className="rounded-lg border p-4">
            <div className="text-label text-sm text-ink-500">{t("assessment_summary_violations")}</div>
            <div className="mt-1 text-2xl font-bold">{summary.violation_events}</div>
            <div className="text-xs text-ink-500">
              {summary.violation_attempts} {t("assessment_summary_violation_attempts")}
            </div>
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <p className="text-xs text-ink-500">{t("assessment_search_label")}</p>
            <Input
              className="mt-1 h-9 w-[220px] text-xs"
              placeholder={t("assessment_search_placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div>
            <p className="text-xs text-ink-500">{t("select_school")}</p>
            <Select
              value={selectedSchoolId || ALL_SCHOOLS_VALUE}
              onValueChange={(v) => setSelectedSchoolId(v === ALL_SCHOOLS_VALUE ? "" : v)}
            >
              <SelectTrigger className="mt-1 h-9 w-[240px] text-xs" aria-label={t("select_school")}>
                <SelectValue placeholder={t("students_all_schools")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_SCHOOLS_VALUE}>{t("students_all_schools")}</SelectItem>
                {(schoolsData ?? []).map((s) => (
                  <SelectItem key={s.id} value={s.id}>
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <Button size="sm" onClick={handleExport} disabled={exporting || !examId}>
          {exporting ? (
            <Loader2 className="mr-1 size-4 animate-spin" />
          ) : (
            <Download className="mr-1 size-4" />
          )}
          {exporting ? t("school_reports_export_loading") : t("school_reports_export")}
        </Button>
      </div>

      {query.isLoading && accumulated.length === 0 ? (
        <div className="py-12 text-center text-ink-500">{t("sys_loading_data")}</div>
      ) : query.isError && accumulated.length === 0 ? (
        <div className="py-12 text-center text-ink-500">{t("sys_error_load")}</div>
      ) : (
        <DataTable
          columns={columns}
          rows={accumulated}
          rowKey={(row) => row.registration_id}
          empty={t("assessment_empty")}
          footer={
            nextCursor && (
              <div className="border-t border-line px-4 py-3 text-center">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleLoadMore}
                  disabled={query.isFetching}
                >
                  {query.isFetching ? t("sys_loading") : t("sys_load_more")}
                </Button>
              </div>
            )
          }
        />
      )}

      <Dialog
        open={selectedRegistrationId !== ""}
        onOpenChange={(open) => {
          if (!open) setSelectedRegistrationId("");
        }}
      >
        <DialogContent className="sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle className="font-serif">{t("assessment_drawer_title")}</DialogTitle>
          </DialogHeader>
          {selectedRow && (
            <AssessmentDrawerContent
              examId={examId}
              row={selectedRow}
              t={t as unknown as (key: string) => string}
              dateLocale={dateLocale}
            />
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function AssessmentDrawerContent({
  examId,
  row,
  t,
  dateLocale,
}: {
  examId: string;
  row: AssessmentRow;
  t: (key: string) => string;
  dateLocale: string;
}) {
  const attempts = useAssessmentAttempts(examId, row.registration_id);
  const [selectedSessionId, setSelectedSessionId] = useState<string>("");
  const detail = useAdminResultDetail(selectedSessionId);

  const isEligible = (attempt: AssessmentAttempt) => attempt.result_available;

  return (
    <div className="space-y-4">
      <div className="text-sm text-ink-600">
        <p>
          <span className="font-semibold text-ink-900">{row.student_name}</span> · Username:{" "}
          {row.username ?? "-"}
        </p>
        <p className="text-xs text-ink-500">{row.school_name || "-"}</p>
      </div>

      {attempts.isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 2 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      )}

      {!attempts.isLoading && (attempts.data?.data ?? []).length === 0 && (
        <p className="py-4 text-center text-sm text-ink-500">{t("assessment_no_attempts")}</p>
      )}

      {!attempts.isLoading && (attempts.data?.data ?? []).length > 0 && (
        <ul className="space-y-2">
          {(attempts.data?.data ?? []).map((attempt) => (
            <li
              key={attempt.session_id}
              className="flex items-center justify-between rounded-lg border p-3"
            >
              <div className="text-sm">
                <div className="font-medium text-ink-900">
                  {t("assessment_attempt_number")} {attempt.attempt_number}
                  {attempt.is_latest && (
                    <span className="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">
                      {t("assessment_latest_badge")}
                    </span>
                  )}
                </div>
                <div className="text-xs text-ink-500">
                  {attempt.status} ·{" "}
                  {attempt.submitted_at
                    ? new Date(attempt.submitted_at).toLocaleString(dateLocale, {
                        day: "2-digit",
                        month: "short",
                        year: "numeric",
                      })
                    : "-"}{" "}
                  · {t("assessment_col_violations")}: {attempt.violations}
                </div>
              </div>
              {isEligible(attempt) ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setSelectedSessionId(attempt.session_id)}
                >
                  {t("action_view")}
                </Button>
              ) : (
                <span className="text-xs text-ink-400">{t("assessment_attempt_unavailable")}</span>
              )}
            </li>
          ))}
        </ul>
      )}

      {selectedSessionId && (
        <div className="rounded-lg border p-4">
          {detail.isLoading ? (
            <div className="py-4 text-center text-ink-500">{t("sys_loading_data")}</div>
          ) : detail.data ? (
            <div className="space-y-3">
              <div className="grid grid-cols-4 gap-2 text-center text-xs">
                <div className="rounded-lg bg-surface-2 p-2">
                  <div className="text-lg font-bold text-ink-900">{detail.data.score}</div>
                  <div className="text-ink-500">{t("school_reports_detail_score")}</div>
                </div>
                <div className="rounded-lg bg-success-bg p-2">
                  <div className="text-lg font-bold text-success">{detail.data.correct_count}</div>
                  <div className="text-ink-500">{t("school_reports_detail_correct")}</div>
                </div>
                <div className="rounded-lg bg-danger-bg p-2">
                  <div className="text-lg font-bold text-danger">{detail.data.wrong_count}</div>
                  <div className="text-ink-500">{t("school_reports_detail_wrong")}</div>
                </div>
                <div className="rounded-lg bg-surface-2 p-2">
                  <div className="text-lg font-bold text-ink-900">{detail.data.empty_count}</div>
                  <div className="text-ink-500">{t("school_reports_detail_empty")}</div>
                </div>
              </div>
              {detail.data.breakdown && detail.data.breakdown.length > 0 && (
                <div>
                  <h4 className="mb-2 text-sm font-semibold text-ink-900">{t("result_by_topic")}</h4>
                  <div className="space-y-1">
                    {detail.data.breakdown.map((b) => (
                      <div
                        key={b.test_id}
                        className="flex items-center justify-between rounded-md bg-surface-2 px-3 py-2 text-xs"
                      >
                        <span className="text-ink-700">{b.title}</span>
                        <span className="font-semibold text-ink-900">
                          {b.earned}/{b.max}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}
