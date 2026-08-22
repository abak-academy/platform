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
import { useAssessment, useAssessmentAttempts } from "@/lib/hooks/admin-assessment";
import { exportAdminResults, useAdminResultDetail } from "@/lib/hooks/admin-results";
import { useSchools } from "@/lib/hooks/students";
import { formatChoiceAnswer } from "@/lib/option-key";
import type { AdminResultDetail, AssessmentAttempt, AssessmentRow, AssessmentSummary } from "@/lib/types";

interface AssessmentTabProps {
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

function statusLabelKey(status: string): "assessment_status_completed" | "assessment_status_in_progress" | "assessment_status_not_started" {
  if (status === "completed" || status === "submitted") return "assessment_status_completed";
  if (status === "in_progress") return "assessment_status_in_progress";
  return "assessment_status_not_started";
}

export function AssessmentTab({ examId }: AssessmentTabProps) {
  const { t, lang } = useTranslation();
  const dateLocale = lang === "en" ? "en-US" : "id-ID";

  const { data: schoolsData } = useSchools(true);
  const [selectedSchoolId, setSelectedSchoolId] = useState("");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search, 300);

  const [accumulated, setAccumulated] = useState<AssessmentRow[]>([]);
  const [activeCursor, setActiveCursor] = useState<string | undefined>(undefined);
  const [nextCursor, setNextCursor] = useState<string | undefined>(undefined);
  const [stableSummary, setStableSummary] = useState<AssessmentSummary | null>(null);
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

  const query = useAssessment({
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
  const attempts = useAssessmentAttempts(examId, selectedRegistrationId);
  const detail = useAdminResultDetail(selectedSessionId);
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

  const columns: DataTableColumn<AssessmentRow>[] = [
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
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <SummaryCard label={t("assessment_summary_total_registered")} value={summary.total_registered} />
          <SummaryCard label={t("assessment_summary_completion_rate")} value={`${Math.round(summary.completion_rate * 100)}%`} />
          <SummaryCard label={t("admin_exam_analytics_average_score")} value={summary.average_score.toFixed(1)} />
          <SummaryCard
            label={t("assessment_summary_violations")}
            value={summary.violation_events}
            caption={`${summary.violation_attempts} ${t("assessment_summary_violation_attempts")}`}
          />
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
          empty={t("assessment_empty")}
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

function AttemptSelector({
  attempts,
  isLoading,
  selectedSessionId,
  onSelect,
  t,
  dateLocale,
}: {
  attempts: AssessmentAttempt[];
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
      <h4 className="text-sm font-semibold text-ink-900">{t("assessment_col_attempts")}</h4>
      {attempts.map((attempt) => {
        const selected = attempt.session_id === selectedSessionId;
        return (
          <div
            key={attempt.session_id}
            className={`flex items-center justify-between rounded-xl border px-4 py-3 text-sm ${selected ? "border-primary bg-primary/5" : "border-line bg-card"}`}
          >
            <div>
              <div className="font-medium text-ink-900">
                {t("assessment_attempt_number")} {attempt.attempt_number}
                {attempt.is_latest && <span className="ml-2 rounded-full bg-primary/10 px-2 py-0.5 text-xs text-primary">{t("assessment_latest_badge")}</span>}
              </div>
              <div className="text-xs text-ink-500">
                {t(statusLabelKey(attempt.status))} · {attempt.submitted_at ? new Date(attempt.submitted_at).toLocaleString(dateLocale, { day: "2-digit", month: "short", year: "numeric" }) : "-"} · {t("assessment_col_violations")}: {attempt.violations}
              </div>
            </div>
            {attempt.result_available ? (
              <Button variant={selected ? "default" : "outline"} size="sm" onClick={() => onSelect(attempt.session_id)}>
                {t("action_view")}
              </Button>
            ) : (
              <span className="text-xs text-ink-600">{t("assessment_attempt_unavailable")}</span>
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
              const isOpen = openTopics[topic.id] ?? index === 0;
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
      <div className="mb-2 flex items-center gap-2 font-semibold text-ink-900">
        {icon}
        <span>#{index + 1}</span>
        <span>{formatChoiceAnswer(item.your_answer, item.format) || "—"}</span>
      </div>
      <div className="font-medium text-ink-900"><RichContent html={item.body} /></div>
      {item.options && item.options.length > 0 && (
        <div className="mt-3 grid gap-2 sm:grid-cols-2">
          {item.options.map((option) => {
            const selected = answerIncludes(item.your_answer, option.key);
            const optionClass = option.is_correct
              ? "border-success bg-success-bg"
              : selected
                ? "border-danger bg-danger-bg"
                : "border-line bg-surface";
            return (
              <div key={option.key} className={`rounded-lg border px-3 py-2 ${optionClass}`}>
                <div className="flex items-start gap-2">
                  <span className="mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full bg-card text-[11px] font-bold text-ink-800">{option.key.toUpperCase()}</span>
                  <div className="min-w-0 flex-1 text-ink-800">
                    <RichContent html={option.text} />
                    {option.image_url && <img src={option.image_url} alt="" className="mt-2 max-h-24 rounded-md object-contain" />}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
      <div className="mt-3 grid gap-2 text-ink-700 sm:grid-cols-2">
        <p>{t("result_your_answer")}: <span className="font-semibold text-ink-900">{formatChoiceAnswer(item.your_answer, item.format) || "—"}</span></p>
        {item.is_correct === false && (
          <p>{t("result_correct_answer")}: <span className="font-semibold text-ink-900">{formatChoiceAnswer(item.correct_answer, item.format) || "—"}</span></p>
        )}
      </div>
      {item.explanation && <p className="mt-3 rounded-lg bg-card/70 px-3 py-2 text-ink-700">{item.explanation}</p>}
    </div>
  );
}

function answerIncludes(answer: string | null | undefined, key: string): boolean {
  return (answer ?? "")
    .split(",")
    .map((part) => part.trim().toUpperCase())
    .includes(key.trim().toUpperCase());
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
