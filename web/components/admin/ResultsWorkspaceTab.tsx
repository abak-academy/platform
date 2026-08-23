"use client";

import { useEffect, useRef, useState } from "react";
import { CheckCircle2, Circle, Download, Eye, Loader2, XCircle } from "lucide-react";
import { toast } from "sonner";

import { RichContent } from "@/components/admin/RichContent";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";
import { useResultsWorkspace, useResultsWorkspaceAttempts, useResultsWorkspaceDetail } from "@/lib/hooks/admin-results-workspace";
import { exportAdminResults } from "@/lib/hooks/admin-results";
import { useSchools } from "@/lib/hooks/students";
import { formatChoiceAnswer } from "@/lib/option-key";
import type { AdminResultDetail, ResultsWorkspaceAttempt, ResultsWorkspaceRow, ResultsWorkspaceSummary } from "@/lib/types";

interface ResultsWorkspaceTabProps {
  examId: string;
}

const ALL_SCHOOLS_VALUE = "_all_";

function useDebouncedValue(value: string, delay: number): string {
  const [debounced, setDebounced] = useState(value);
  useEffect(() => {
    const id = setTimeout(() => setDebounced(value), delay);
    return () => clearTimeout(id);
  }, [value, delay]);
  return debounced;
}

function statusLabelKey(status: string): "results_workspace_status_completed" | "results_workspace_status_in_progress" | "results_workspace_status_not_started" {
  if (status === "completed" || status === "submitted") return "results_workspace_status_completed";
  if (status === "in_progress") return "results_workspace_status_in_progress";
  return "results_workspace_status_not_started";
}

export function ResultsWorkspaceTab({ examId }: ResultsWorkspaceTabProps) {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const { data: schoolsData } = useSchools(true);
  const [selectedSchoolId, setSelectedSchoolId] = useState("");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);

  const [accumulated, setAccumulated] = useState<ResultsWorkspaceRow[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [stableSummary, setStableSummary] = useState<ResultsWorkspaceSummary | null>(null);
  const [selectedRegistrationId, setSelectedRegistrationId] = useState("");
  const [selectedSessionId, setSelectedSessionId] = useState("");

  const filterKey = `${examId}:${debouncedSearch}:${selectedSchoolId}`;
  const filterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      setSelectedRegistrationId("");
      setSelectedSessionId("");
      filterKeyRef.current = filterKey;
    }
  }, [filterKey]);

  const query = useResultsWorkspace({
    examId,
    q: debouncedSearch || undefined,
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
    setStableSummary(query.data.summary);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, filterKey]);

  const selectedRow = accumulated.find((r) => r.registration_id === selectedRegistrationId);
  const attempts = useResultsWorkspaceAttempts(examId, selectedRegistrationId);
  const detail = useResultsWorkspaceDetail(examId, selectedSessionId);
  const summary = query.data?.summary ?? stableSummary;

  useEffect(() => {
    if (!selectedRow) {
      setSelectedSessionId("");
      return;
    }
    setSelectedSessionId(selectedRow.latest_session_id ?? "");
  }, [selectedRow?.registration_id, selectedRow?.latest_session_id]);

  const [exporting, setExporting] = useState(false);
  const handleExport = async () => {
    if (!examId) return;
    setExporting(true);
    try {
      await exportAdminResults(examId, selectedSchoolId || undefined, debouncedSearch || undefined);
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("sys_error_load"));
    } finally {
      setExporting(false);
    }
  };

  const columns: DataTableColumn<ResultsWorkspaceRow>[] = [
    {
      key: "rank",
      header: t("admin_exam_leaderboard_col_rank"),
      cell: (row) => <span className="text-xs font-semibold text-ink-700">{row.rank ?? "-"}</span>,
    },
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
      key: "score",
      header: t("school_reports_col_score"),
      cell: (row) => <span className="text-xs text-ink-600">{row.score == null ? "-" : row.score.toFixed(1)}</span>,
    },
    {
      key: "attempts",
      header: t("results_workspace_col_attempts"),
      cell: (row) => <span className="text-xs text-ink-600">{row.attempts_count}</span>,
    },
    {
      key: "violations",
      header: t("results_workspace_col_violations"),
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
          disabled={!row.latest_session_id}
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
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-5">
          <SummaryCard label={t("results_workspace_summary_total_registered")} value={summary.total_registered} />
          <SummaryCard
            label={t("results_workspace_summary_completion_rate")}
            value={`${Math.round(summary.completion_rate * 100)}%`}
            caption={`${summary.completed_participants}/${summary.total_registered}`}
          />
          <SummaryCard label={t("admin_exam_analytics_average_score")} value={summary.average_score.toFixed(1)} />
          <SummaryCard
            label={t("results_workspace_summary_violations")}
            value={summary.violation_events}
            caption={`${summary.violation_attempts} ${t("results_workspace_summary_violation_attempts")}`}
          />
          <ScoreDistributionCard label={t("admin_exam_analytics_distribution")} distribution={summary.distribution} />
        </div>
      )}

      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <p className="text-xs text-ink-500">{t("results_workspace_search_label")}</p>
            <Input
              className="mt-1 h-9 w-[220px] text-xs"
              placeholder={t("results_workspace_search_placeholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>
          <div>
            <p className="text-xs text-ink-500">{t("select_school")}</p>
            <Select value={selectedSchoolId || ALL_SCHOOLS_VALUE} onValueChange={(v) => setSelectedSchoolId(v === ALL_SCHOOLS_VALUE ? "" : v)}>
              <SelectTrigger className="mt-1 h-9 w-[240px] text-xs" aria-label={t("select_school")}>
                <SelectValue placeholder={t("students_all_schools")} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={ALL_SCHOOLS_VALUE}>{t("students_all_schools")}</SelectItem>
                {(schoolsData ?? []).map((s) => (
                  <SelectItem key={s.id} value={s.id}>{s.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <Button size="sm" onClick={handleExport} disabled={exporting || !examId}>
          {exporting ? <Loader2 className="mr-1 size-4 animate-spin" /> : <Download className="mr-1 size-4" />}
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
          empty={t("results_workspace_empty")}
          footer={nextCursor && (
            <div className="border-t border-line px-4 py-3 text-center">
              <Button variant="outline" size="sm" onClick={() => setActiveCursor(nextCursor)} disabled={query.isFetching}>
                {query.isFetching ? t("sys_loading") : t("sys_load_more")}
              </Button>
            </div>
          )}
        />
      )}

      <Dialog
        open={selectedRegistrationId !== ""}
        onOpenChange={(open) => {
          if (!open) setSelectedRegistrationId("");
        }}
      >
        <DialogContent className="max-h-[88vh] overflow-hidden sm:max-w-5xl">
          <DialogHeader>
            <DialogTitle className="font-serif">{t("school_reports_detail_title")}</DialogTitle>
          </DialogHeader>
          <div className="max-h-[72vh] overflow-y-auto pr-1">
            {selectedRow && (
              <div className="space-y-4">
                <div className="text-sm text-ink-600">
                  <p>
                    <span className="font-semibold text-ink-900">{selectedRow.student_name}</span> · Username: {selectedRow.username ?? "-"}
                  </p>
                  <p className="text-xs text-ink-500">{selectedRow.school_name || "-"}</p>
                </div>
                <AttemptSelector
                  attempts={attempts.data?.data ?? []}
                  isLoading={attempts.isLoading}
                  selectedSessionId={selectedSessionId}
                  onSelect={setSelectedSessionId}
                  t={t}
                  dateLocale={dateLocale}
                />
                {detail.isLoading ? (
                  <div className="py-8 text-center text-ink-500">{t("sys_loading_data")}</div>
                ) : detail.data ? (
                  <ResultDetailPanel detail={detail.data} t={t} dateLocale={dateLocale} />
                ) : selectedSessionId ? (
                  <div className="py-8 text-center text-ink-500">{t("sys_error_load")}</div>
                ) : null}
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function SummaryCard({ label, value, caption }: { label: string; value: string | number; caption?: string }) {
  return (
    <div className="rounded-xl border border-line bg-card p-4 shadow-sm">
      <div className="text-label text-sm text-ink-600">{label}</div>
      <div className="mt-1 text-2xl font-bold text-ink-950">{value}</div>
      {caption && <div className="text-xs text-ink-600">{caption}</div>}
    </div>
  );
}

function ScoreDistributionCard({
  label,
  distribution,
}: {
  label: string;
  distribution: { label: string; count: number }[];
}) {
  const maxCount = Math.max(1, ...distribution.map((bucket) => bucket.count));
  return (
    <div className="rounded-xl border border-line bg-card p-4 shadow-sm">
      <div className="text-label text-sm text-ink-600">{label}</div>
      <div className="mt-3 space-y-1.5">
        {distribution.map((bucket) => (
          <div key={bucket.label} className="grid grid-cols-[44px_1fr_20px] items-center gap-2 text-xs text-ink-600">
            <span>{bucket.label}</span>
            <div className="h-2 overflow-hidden rounded-full bg-surface-2">
              <div
                className="h-full rounded-full bg-primary"
                style={{ width: `${Math.max(6, (bucket.count / maxCount) * 100)}%` }}
              />
            </div>
            <span className="text-right font-semibold text-ink-900">{bucket.count}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

function AttemptSelector({
  attempts,
  isLoading,
  selectedSessionId,
  onSelect,
  t,
  dateLocale,
}: {
  attempts: ResultsWorkspaceAttempt[];
  isLoading: boolean;
  selectedSessionId: string;
  onSelect: (sessionId: string) => void;
  t: ReturnType<typeof useTranslation>["t"];
  dateLocale: string;
}) {
  if (isLoading) {
    return <div className="rounded-xl border border-line bg-surface-2 px-4 py-3 text-sm text-ink-500">{t("sys_loading_data")}</div>;
  }
  if (attempts.length === 0) return null;

  return (
    <div className="space-y-2">
      <h4 className="text-sm font-semibold text-ink-900">{t("results_workspace_col_attempts")}</h4>
      {attempts.map((attempt) => {
        const selected = attempt.session_id === selectedSessionId;
        return (
          <div
            key={attempt.session_id}
            className={`flex items-center justify-between rounded-xl border px-4 py-3 text-sm ${selected ? "border-primary bg-primary/5" : "border-line bg-card"}`}
          >
            <div>
              <div className="font-medium text-ink-900">
                {t("results_workspace_attempt_number")} {attempt.attempt_number}
                {attempt.is_latest && <span className="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{t("results_workspace_latest_badge")}</span>}
              </div>
              <div className="text-xs text-ink-500">
                {t(statusLabelKey(attempt.status))} · {attempt.submitted_at ? new Date(attempt.submitted_at).toLocaleString(dateLocale, { day: "2-digit", month: "short", year: "numeric" }) : "-"} · {t("results_workspace_col_violations")}: {attempt.violations}
              </div>
            </div>
            {attempt.result_available ? (
              <Button variant={selected ? "default" : "outline"} size="sm" onClick={() => onSelect(attempt.session_id)}>
                {t("action_view")}
              </Button>
            ) : (
              <span className="text-xs text-ink-600">{t("results_workspace_attempt_unavailable")}</span>
            )}
          </div>
        );
      })}
    </div>
  );
}


function ResultDetailPanel({
  detail,
  t,
  dateLocale,
}: {
  detail: AdminResultDetail;
  t: ReturnType<typeof useTranslation>["t"];
  dateLocale: string;
}) {
  const topics = groupPembahasanByTopic(detail);
  const [openTopics, setOpenTopics] = useState<Record<string, boolean>>({});

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-4 gap-2 text-center text-xs">
        <MetricCard value={detail.score} label={t("school_reports_detail_score")} />
        <MetricCard value={detail.correct_count} label={t("school_reports_detail_correct")} tone="success" />
        <MetricCard value={detail.wrong_count} label={t("school_reports_detail_wrong")} tone="danger" />
        <MetricCard value={detail.empty_count} label={t("school_reports_detail_empty")} />
      </div>

      <div className="text-xs text-ink-500">
        {t("school_reports_col_submitted")}: {new Date(detail.submitted_at).toLocaleString(dateLocale, { day: "2-digit", month: "short", year: "numeric" })}
      </div>

      {topics.length > 0 && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">{t("result_pembahasan")}</h4>
          <div className="space-y-2">
            {topics.map((topic, index) => {
              const isOpen = openTopics[topic.id] ?? false;
              const breakdown = detail.breakdown?.find((b) => b.test_id === topic.id || b.title === topic.title);
              return (
                <div key={topic.id} className="overflow-hidden rounded-xl border border-line bg-card">
                  <button
                    type="button"
                    className="flex w-full items-center justify-between gap-3 bg-surface-2 px-4 py-3 text-left text-sm"
                    onClick={() => setOpenTopics((prev) => ({ ...prev, [topic.id]: !isOpen }))}
                  >
                    <span className="font-semibold text-ink-900">{topic.title}</span>
                    <span className="text-xs font-semibold text-ink-700">
                      {breakdown ? `${breakdown.earned}/${breakdown.max}` : `${topic.items.length} soal`}
                    </span>
                  </button>
                  {isOpen && (
                    <div className="max-h-96 space-y-2 overflow-y-auto p-3">
                      {topic.items.map((item, questionIndex) => (
                        <QuestionReviewCard key={item.question_id} item={item} index={questionIndex} t={t} />
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}

function groupPembahasanByTopic(detail: AdminResultDetail) {
  const groups = new Map<string, { id: string; title: string; items: NonNullable<AdminResultDetail["pembahasan"]> }>();
  for (const item of detail.pembahasan ?? []) {
    const fallback = detail.breakdown?.[0];
    const id = item.test_id ?? fallback?.test_id ?? "general";
    const title = item.test_title ?? fallback?.title ?? "Explanation";
    const existing = groups.get(id) ?? { id, title, items: [] };
    existing.items.push(item);
    groups.set(id, existing);
  }
  return Array.from(groups.values());
}

function QuestionReviewCard({
  item,
  index,
  t,
}: {
  item: NonNullable<AdminResultDetail["pembahasan"]>[number];
  index: number;
  t: ReturnType<typeof useTranslation>["t"];
}) {
  const tone = item.is_correct === true ? "correct" : item.is_correct === false ? "wrong" : "empty";
  const toneClass = tone === "correct"
    ? "border-success/30 bg-success-bg"
    : tone === "wrong"
      ? "border-danger/30 bg-danger-bg"
      : "border-line bg-surface-2";
  const icon = tone === "correct"
    ? <CheckCircle2 className="size-4 text-success" />
    : tone === "wrong"
      ? <XCircle className="size-4 text-danger" />
      : <Circle className="size-4 text-ink-500" />;

  return (
    <div className={`rounded-xl border ${toneClass} px-4 py-3 text-xs`}>
      <div className="mb-2 flex items-start gap-2 font-semibold text-ink-900">
        {icon}
        <span className="shrink-0">#{index + 1}</span>
        <div className="min-w-0 flex-1"><RichContent html={item.body} /></div>
      </div>
      <div className="grid gap-2 text-ink-700 sm:grid-cols-2">
        <p>{t("result_your_answer")}: <span className="font-semibold text-ink-900">{formatChoiceAnswer(item.your_answer, item.format) || "—"}</span></p>
        {item.is_correct === false && (
          <p>{t("result_correct_answer")}: <span className="font-semibold text-ink-900">{formatChoiceAnswer(item.correct_answer, item.format) || "—"}</span></p>
        )}
      </div>
      {item.explanation && <p className="mt-3 rounded-lg bg-card/70 px-3 py-2 text-ink-700">{item.explanation}</p>}
    </div>
  );
}
function MetricCard({ value, label, tone }: { value: number; label: string; tone?: "success" | "danger" }) {
  const bg = tone === "success" ? "bg-success-bg" : tone === "danger" ? "bg-danger-bg" : "bg-surface-2";
  const color = tone === "success" ? "text-success" : tone === "danger" ? "text-danger" : "text-ink-900";
  return (
    <div className={`rounded-lg ${bg} p-2`}>
      <div className={`text-lg font-bold ${color}`}>{value}</div>
      <div className="text-ink-600">{label}</div>
    </div>
  );
}
