"use client";

import { useEffect, useRef, useState } from "react";
import { Download, Eye, Loader2 } from "lucide-react";
import { toast } from "sonner";

import { RichContent } from "@/components/admin/RichContent";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { useTranslation } from "@/lib/i18n";
import { useAssessment } from "@/lib/hooks/admin-assessment";
import { exportAdminResults, useAdminResultDetail } from "@/lib/hooks/admin-results";
import { useSchools } from "@/lib/hooks/students";
import { formatChoiceAnswer } from "@/lib/option-key";
import type { AdminResultDetail, AssessmentRow, AssessmentSummary } from "@/lib/types";

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

  const filterKey = `${examId}:${debouncedSearch}:${selectedSchoolId}`;
  const filterKeyRef = useRef(filterKey);

  useEffect(() => {
    if (filterKey !== filterKeyRef.current) {
      setAccumulated([]);
      setActiveCursor(undefined);
      setNextCursor(undefined);
      setSelectedRegistrationId("");
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
  const selectedSessionId = selectedRow?.latest_session_id ?? "";
  const detail = useAdminResultDetail(selectedSessionId);
  const summary = query.data?.summary ?? stableSummary;

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
          variant={selectedRegistrationId === row.registration_id ? "outline" : "ghost"}
          size="icon"
          aria-label={t("action_view")}
          disabled={!row.latest_session_id}
          onClick={() => setSelectedRegistrationId((current) => current === row.registration_id ? "" : row.registration_id)}
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

      {selectedRow && (
        <div className="rounded-2xl border border-line bg-card p-5 shadow-sm">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div>
              <h3 className="font-serif text-lg font-semibold text-ink-900">{t("school_reports_detail_title")}</h3>
              <p className="mt-1 text-sm text-ink-600">
                <span className="font-semibold text-ink-900">{selectedRow.student_name}</span> · Username: {selectedRow.username ?? "-"}
              </p>
              <p className="text-xs text-ink-500">{selectedRow.school_name || "-"}</p>
            </div>
            <Button variant="outline" size="sm" onClick={() => setSelectedRegistrationId("")}>{t("cancel")}</Button>
          </div>
          {detail.isLoading ? (
            <div className="py-8 text-center text-ink-500">{t("sys_loading_data")}</div>
          ) : detail.data ? (
            <ResultDetailPanel detail={detail.data} t={t} dateLocale={dateLocale} />
          ) : (
            <div className="py-8 text-center text-ink-500">{t("sys_error_load")}</div>
          )}
        </div>
      )}
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

function ResultDetailPanel({
  detail,
  t,
  dateLocale,
}: {
  detail: AdminResultDetail;
  t: ReturnType<typeof useTranslation>["t"];
  dateLocale: string;
}) {
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

      {detail.breakdown && detail.breakdown.length > 0 && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">{t("result_by_topic")}</h4>
          <div className="space-y-1">
            {detail.breakdown.map((b) => (
              <div key={b.test_id} className="flex items-center justify-between rounded-md bg-surface-2 px-3 py-2 text-xs">
                <span className="text-ink-700">{b.title}</span>
                <span className="font-semibold text-ink-900">{b.earned}/{b.max}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {detail.pembahasan && detail.pembahasan.length > 0 && (
        <div>
          <h4 className="mb-2 text-sm font-semibold text-ink-900">{t("result_pembahasan")}</h4>
          <div className="max-h-80 space-y-2 overflow-y-auto">
            {detail.pembahasan.map((p, index) => (
              <div key={p.question_id} className="rounded-md border border-line bg-surface-2 px-3 py-2 text-xs">
                <div className="mb-1 font-semibold text-ink-900">#{index + 1}</div>
                <div className="font-medium text-ink-900"><RichContent html={p.body} /></div>
                <p className="mt-1 text-ink-600">{t("result_your_answer")}: {formatChoiceAnswer(p.your_answer, p.format) || "—"}</p>
                <p className="text-ink-600">{t("result_correct_answer")}: {formatChoiceAnswer(p.correct_answer, p.format) || "—"}</p>
                {p.explanation && <p className="mt-1 text-ink-500 italic">{p.explanation}</p>}
              </div>
            ))}
          </div>
        </div>
      )}
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
