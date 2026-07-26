"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import {
  ArrowLeft,
  ListChecks,
  Pencil,
  Plus,
  Trash2,
  Trophy,
  Users,
} from "lucide-react";
import { CertificateDesignTab } from "@/components/admin/CertificateDesignTab";
import { ExamModal } from "@/components/admin/ExamModal";
import { ExamRegistrationsTab } from "@/components/admin/ExamRegistrationsTab";
import { UnderMaintenance } from "@/components/admin/UnderMaintenance";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useExam,
  useExamAnalytics,
  useExamLeaderboard,
  useGradeEssay,
  useGradingSessions,
  useReplaceExamTests,
  useSessionEssays,
} from "@/lib/hooks/admin-exams";
import { useAdminTests } from "@/lib/hooks/admin-tests";
import { useTranslation } from "@/lib/i18n";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import { useAuthStore } from "@/stores/auth";
import type { ExamLeaderboardEntry, GradingEssayItem } from "@/lib/types";

// admin_school gets a scoped view — only the exam summary + the
// registrations action (bulk order/grant), never the content-management tabs.
const SCHOOL_SCOPED_TABS: Tab[] = ["overview", "registrations"];

// Price/status/publish for an exam-type product live on the attached Product(s),
// managed via /admin/products (mirrors Course) — not here.
type Tab =
  | "overview"
  | "tests"
  | "certificate"
  | "registrations"
  | "results"
  | "grading"
  | "leaderboard";

const TAB_ORDER: Tab[] = [
  "overview",
  "tests",
  "certificate",
  "registrations",
  "results",
  "grading",
  "leaderboard",
];

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error) return err.message;
  return fallback;
}

function formatScheduled(iso?: string | null): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString("id-ID", { dateStyle: "medium", timeStyle: "short" });
}

function formatMode(mode?: string | null): string {
  if (!mode || mode === "standard") return "Standard";
  return mode.toUpperCase();
}

export default function ExamPackageDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const { t } = useTranslation();
  const role = useAuthStore((s) => s.user?.role);
  const isSchoolScoped = role === "admin_school";
  const visibleTabs = isSchoolScoped ? SCHOOL_SCOPED_TABS : TAB_ORDER;

  const [tab, setTab] = useState<Tab>("overview");
  const [editOpen, setEditOpen] = useState(false);

  const { data, isLoading, isError, error, refetch } = useExam(id);
  const replaceTests = useReplaceExamTests(id);
  const { data: availableResp, isLoading: availableLoading } = useAdminTests(
    undefined,
    !isSchoolScoped,
  );
  const availableTests = availableResp?.data ?? [];

  const [lbEntries, setLbEntries] = useState<ExamLeaderboardEntry[]>([]);
  const [lbCursor, setLbCursor] = useState<string | undefined>(undefined);

  const { data: analytics, isLoading: analyticsLoading } = useExamAnalytics(id, !isSchoolScoped);
  const lb = useExamLeaderboard(
    id,
    lbCursor ? { cursor: lbCursor, limit: 20 } : { limit: 20 },
    !isSchoolScoped,
  );

  interface PendingSection {
    id: string;
    title: string;
    subject: string;
    section_type?: string;
    duration_minutes: number;
  }

  const [attachedIds, setAttachedIds] = useState<string[]>([]);
  const [pendingSections, setPendingSections] = useState<PendingSection[]>([]);

  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(null);
  const [scoreInputs, setScoreInputs] = useState<
    Record<string, { score: string; comment: string }>
  >({});

  const {
    data: gradingResp,
    isLoading: gradingLoading,
    isError: gradingIsError,
    error: gradingError,
  } = useGradingSessions(id, !isSchoolScoped);
  const {
    data: essaysResp,
    isLoading: essaysLoading,
    isError: essaysIsError,
    error: essaysError,
  } = useSessionEssays(selectedSessionId ?? undefined, !isSchoolScoped);
  const gradeEssay = useGradeEssay(selectedSessionId ?? "");

  useEffect(() => {
    if (!data) return;
    setAttachedIds(data.tests.map((entry) => entry.test_id));
  }, [data]);

  useEffect(() => {
    if (!essaysResp) return;
    const next: Record<string, { score: string; comment: string }> = {};
    for (const essay of essaysResp.data) {
      next[essay.question_id] = {
        score: essay.score != null ? String(essay.score) : "",
        comment: essay.grader_comment ?? "",
      };
    }
    setScoreInputs(next);
  }, [essaysResp]);

  const availableToAdd = useMemo(() => {
    const attached = new Set(attachedIds);
    return availableTests.filter((test) => !attached.has(test.id));
  }, [availableTests, attachedIds]);

  function handleAddTest(testId: string) {
    setAttachedIds((prev) => [...prev, testId]);
  }

  function handleRemoveTest(testId: string) {
    setAttachedIds((prev) => prev.filter((entry) => entry !== testId));
  }

  const UTBK_PRESETS: PendingSection[] = [
    { id: "utbk-tps", title: "TPS - Potensi Skolastik", subject: "TPS", duration_minutes: 60 },
    { id: "utbk-pm", title: "Penalaran Matematika", subject: "Matematika", duration_minutes: 60 },
    { id: "utbk-li", title: "Literasi Bahasa Indonesia", subject: "Bahasa Indonesia", duration_minutes: 45 },
    { id: "utbk-lb", title: "Literasi Bahasa Inggris", subject: "Bahasa Inggris", duration_minutes: 45 },
  ];

  const IELTS_PRESETS: PendingSection[] = [
    { id: "ielts-listening", title: "IELTS Listening", subject: "Bahasa Inggris", section_type: "listening", duration_minutes: 30 },
    { id: "ielts-reading", title: "IELTS Reading", subject: "Bahasa Inggris", section_type: "reading", duration_minutes: 60 },
    { id: "ielts-writing", title: "IELTS Writing", subject: "Bahasa Inggris", section_type: "writing", duration_minutes: 60 },
  ];

  function handleUtbkPreset() {
    setPendingSections(UTBK_PRESETS);
  }

  function handleIeltsPreset() {
    setPendingSections(IELTS_PRESETS);
  }

  function removePendingSection(id: string) {
    setPendingSections((prev) => prev.filter((s) => s.id !== id));
  }

  async function handleSaveTests() {
    if (!id) return;
    try {
      await replaceTests.mutateAsync(attachedIds);
      toast.success(t("changes_saved"));
      refetch();
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    }
  }

  async function handleSaveEssay(essay: GradingEssayItem) {
    const input = scoreInputs[essay.question_id];
    const scoreNum = Number(input?.score);
    if (
      !input?.score ||
      !Number.isInteger(scoreNum) ||
      scoreNum < 0 ||
      scoreNum > essay.point_correct
    ) {
      toast.error(t("grading_save_failed"));
      return;
    }
    try {
      await gradeEssay.mutateAsync({
        question_id: essay.question_id,
        score: scoreNum,
        comment: input.comment.trim() ? input.comment.trim() : undefined,
      });
      toast.success(t("grading_saved"));
    } catch (e) {
      toast.error(errorMessage(e, t("grading_save_failed")));
    }
  }

  // Reset leaderboard pagination when exam changes
  useEffect(() => {
    setLbEntries([]);
    setLbCursor(undefined);
  }, [id]);

  // Accumulate leaderboard pages
  useEffect(() => {
    if (!lb.data) return;
    if (!lbCursor) {
      setLbEntries(lb.data.data ?? []);
    } else {
      setLbEntries((prev) => [...prev, ...(lb.data.data ?? [])]);
    }
  }, [lb.data]);

  function handleLoadMore() {
    if (lb.data?.next_cursor) {
      setLbCursor(lb.data.next_cursor);
    }
  }

  const title = data?.title ?? t("exam_packages_page_title");

  return (
    <div className="space-y-6 fade-in">
      <Link
        href="/admin/exam/packages"
        className="inline-flex items-center gap-1.5 text-sm font-medium text-ink-500 hover:text-ink-900"
      >
        <ArrowLeft className="size-4" />
        {t("exam_packages_page_title")}
      </Link>

      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-serif text-[27px] font-semibold tracking-tight text-ink-900">
            {title}
          </h1>
          {data && (
            <p className="mt-1.5 text-sm text-ink-500">
              {formatMode(data.mode)} · {formatScheduled(data.scheduled_at)}
            </p>
          )}
        </div>
        {data && (
          <Badge variant={data.status === "published" ? "default" : "secondary"}>
            {data.status ?? "draft"}
          </Badge>
        )}
      </div>

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
          {errorMessage(error, t("error_generic"))}
        </div>
      )}

      {!isLoading && !isError && data && (
        <>
          <div className="flex flex-wrap gap-1 border-b">
            {visibleTabs.map((key) => (
              <button
                key={key}
                type="button"
                onClick={() => setTab(key)}
                className={
                  tab === key
                    ? "border-b-2 border-primary px-3 py-2 text-sm font-medium text-primary"
                    : "px-3 py-2 text-sm font-medium text-muted-foreground hover:text-foreground"
                }
              >
                {t(`admin_exam_detail_tab_${key}` as const)}
              </button>
            ))}
          </div>

          {tab === "overview" && (
            <div className="md-card-outlined space-y-4 p-6">
              <div className="flex items-center justify-between">
                <h2 className="text-title-large font-semibold">
                  {t("admin_exam_detail_tab_overview")}
                </h2>
                {!isSchoolScoped && (
                  <Button
                    variant="outline"
                    size="sm"
                    className="rounded-full"
                    onClick={() => setEditOpen(true)}
                  >
                    <Pencil className="mr-1 size-4" />
                    {t("admin_exam_detail_edit")}
                  </Button>
                )}
              </div>
              <dl className="grid grid-cols-1 gap-4 text-sm sm:grid-cols-2">
                <OverviewRow label="Title" value={data.title} />
                <OverviewRow
                  label="Scheduled"
                  value={
                    data.scheduled_end_at
                      ? `${formatScheduled(data.scheduled_at)} – ${formatScheduled(data.scheduled_end_at)}`
                      : formatScheduled(data.scheduled_at)
                  }
                />
                <OverviewRow label="Mode" value={formatMode(data.mode)} />
                <OverviewRow label="Timer mode" value={data.timer_mode ?? "—"} />
                <OverviewRow
                  label="Duration"
                  value={
                    data.duration_minutes != null
                      ? `${data.duration_minutes} ${t("minutes")}`
                      : "—"
                  }
                />
                <OverviewRow
                  label="Free"
                  value={data.is_free ? t("status_label_active") : t("status_label_inactive")}
                />
                <OverviewRow
                  label="Requires check-in"
                  value={
                    data.requires_checkin
                      ? t("status_label_active")
                      : t("status_label_inactive")
                  }
                />
                <OverviewRow
                  label="Leaderboard"
                  value={
                    data.allow_leaderboard
                      ? t("status_label_active")
                      : t("status_label_inactive")
                  }
                />
                <OverviewRow
                  label="Randomize"
                  value={
                    data.randomize
                      ? t("status_label_active")
                      : t("status_label_inactive")
                  }
                />
                <OverviewRow
                  label={t("exam_packages_overview_result_config")}
                  value={data.result_config ?? "—"}
                />
                <OverviewRow
                  label={t("exam_packages_overview_result_release_at")}
                  value={formatScheduled(data.result_release_at)}
                />
                <OverviewRow
                  label={t("exam_packages_overview_check_in_window")}
                  value={
                    data.check_in_window_minutes != null
                      ? `${data.check_in_window_minutes} ${t("minutes")}`
                      : "—"
                  }
                />
                <OverviewRow
                  label={t("exam_packages_overview_grace_window")}
                  value={
                    data.grace_window_minutes != null
                      ? `${data.grace_window_minutes} ${t("minutes")}`
                      : "—"
                  }
                />
                <OverviewRow
                  label={t("exam_packages_overview_max_attempts")}
                  value={
                    data.max_attempts != null ? String(data.max_attempts) : t("unlimited")
                  }
                />
                <OverviewRow
                  label={t("exam_packages_overview_certificate_template")}
                  value={data.certificate_template ?? "—"}
                />
                <OverviewRow label="Status" value={data.status ?? "—"} />
              </dl>
            </div>
          )}

          {tab === "tests" && (
            <div className="grid gap-4 lg:grid-cols-2">
              <div className="md-card-outlined p-4">
                <div className="mb-3 flex items-center justify-between">
                  <h3 className="text-title-medium font-semibold">
                    {t("admin_exam_detail_tests_attached")}
                  </h3>
                  <span className="text-label text-muted-foreground">
                    {attachedIds.length + pendingSections.length}
                  </span>
                </div>
                {data.mode && data.mode !== "standard" && (
                  <div className="mb-3 flex flex-wrap gap-2">
                    {data.mode === "utbk" && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="rounded-full"
                        onClick={handleUtbkPreset}
                      >
                        {t("tests_preset_utbk")}
                      </Button>
                    )}
                    {data.mode === "ielts" && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        className="rounded-full"
                        onClick={handleIeltsPreset}
                      >
                        {t("tests_preset_ielts")}
                      </Button>
                    )}
                  </div>
                )}
                {attachedIds.length === 0 && pendingSections.length === 0 ? (
                  <div className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
                    —
                  </div>
                ) : (
                  <ul className="space-y-2">
                    {attachedIds.map((testId, idx) => {
                      const meta = data.tests.find((e) => e.test_id === testId)?.test;
                      return (
                        <li
                          key={`${testId}-${idx}`}
                          className="flex items-center justify-between gap-2 rounded-md border p-3 text-sm"
                        >
                          <div className="min-w-0">
                            <div className="truncate font-medium">
                              #{idx + 1} · {meta?.title ?? testId}
                            </div>
                            {meta && (
                              <div className="text-label text-muted-foreground">
                                {meta.subject} · {meta.topic ?? "—"} ·{" "}
                                {meta.question_count ?? 0} soal ·{" "}
                                {meta.duration_minutes ?? 0} {t("minutes")}
                              </div>
                            )}
                          </div>
                          <Button
                            type="button"
                            size="icon-xs"
                            variant="ghost"
                            className="rounded-full"
                            onClick={() => handleRemoveTest(testId)}
                            aria-label={t("admin_exam_detail_tests_remove")}
                          >
                            <Trash2 className="size-3" />
                          </Button>
                        </li>
                      );
                    })}
                  </ul>
                )}
                {pendingSections.length > 0 && (
                  <div className="mt-3">
                    <h4 className="text-label mb-2 text-sm font-medium text-muted-foreground">
                      {t("tests_preset_added")}
                    </h4>
                    <ul className="space-y-2">
                      {pendingSections.map((ps, idx) => (
                        <li
                          key={ps.id}
                          className="flex items-center justify-between gap-2 rounded-md border border-dashed border-brand-300 p-3 text-sm"
                        >
                          <div className="min-w-0">
                            <div className="truncate font-medium">
                              #{attachedIds.length + idx + 1} · {ps.title}
                            </div>
                            <div className="text-label text-muted-foreground">
                              {ps.subject} · {ps.section_type ? `${ps.section_type} · ` : ""}
                              {ps.duration_minutes} {t("minutes")}
                            </div>
                          </div>
                          <Button
                            type="button"
                            size="icon-xs"
                            variant="ghost"
                            className="rounded-full"
                            onClick={() => removePendingSection(ps.id)}
                            aria-label={t("admin_exam_detail_tests_remove")}
                          >
                            <Trash2 className="size-3" />
                          </Button>
                        </li>
                      ))}
                    </ul>
                  </div>
                )}
                <div className="mt-4 flex justify-end">
                  <Button
                    type="button"
                    className="rounded-full"
                    onClick={handleSaveTests}
                    disabled={replaceTests.isPending}
                  >
                    {replaceTests.isPending
                      ? t("saving")
                      : t("admin_exam_detail_tests_save")}
                  </Button>
                </div>
              </div>

              <div className="md-card-outlined p-4">
                <h3 className="text-title-medium mb-3 font-semibold">
                  {t("admin_exam_detail_tests_available")}
                </h3>
                {availableLoading ? (
                  <div className="space-y-2">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <Skeleton key={i} className="h-10 w-full" />
                    ))}
                  </div>
                ) : availableToAdd.length === 0 ? (
                  <div className="rounded-md border border-dashed p-6 text-center text-sm text-muted-foreground">
                    —
                  </div>
                ) : (
                  <ul className="space-y-2">
                    {availableToAdd.map((test) => (
                      <li
                        key={test.id}
                        className="flex items-center justify-between gap-2 rounded-md border p-3 text-sm"
                      >
                        <div className="min-w-0">
                          <div className="truncate font-medium">{test.title}</div>
                          <div className="text-label text-muted-foreground">
                            {test.subject} · {test.topic} · {test.duration_minutes}{" "}
                            {t("minutes")}
                          </div>
                        </div>
                        <Button
                          type="button"
                          size="icon-xs"
                          variant="outline"
                          className="rounded-full"
                          onClick={() => handleAddTest(test.id)}
                          aria-label={t("admin_exam_detail_tests_add")}
                        >
                          <Plus className="size-3" />
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>
          )}

          {tab === "certificate" && (
            <CertificateDesignTab examId={id} exam={data} onSaved={refetch} />
          )}

          {tab === "registrations" && (
            role === "admin_school" || role === "super_admin" ? (
              <ExamRegistrationsTab examId={id} examName={data.title} />
            ) : (
              <UnderMaintenance icon={Users} title={t("admin_exam_detail_tab_registrations")} />
            )
          )}
          {tab === "results" && (
            <UnderMaintenance icon={ListChecks} title={t("admin_exam_detail_tab_results")} />
          )}
          {tab === "grading" && (
            <div className="md-card-outlined space-y-4 p-6">
              <div className="flex items-center justify-between">
                <h2 className="text-title-large font-semibold">
                  {t("grading_sessions_title")}
                </h2>
                {selectedSessionId && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="rounded-full"
                    onClick={() => setSelectedSessionId(null)}
                  >
                    {t("grading_back")}
                  </Button>
                )}
              </div>

              {!selectedSessionId && (
                <>
                  {gradingLoading && (
                    <div className="space-y-2">
                      {Array.from({ length: 3 }).map((_, i) => (
                        <Skeleton key={i} className="h-10 w-full" />
                      ))}
                    </div>
                  )}

                  {gradingIsError && (
                    <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
                      {errorMessage(gradingError, t("error_generic"))}
                    </div>
                  )}

                  {!gradingLoading && !gradingIsError && (
                    <div className="overflow-x-auto md-card-outlined">
                      <table className="w-full text-sm">
                        <thead className="bg-muted">
                          <tr>
                            <th className="px-4 py-3 text-left font-medium">
                              {t("grading_col_student")}
                            </th>
                            <th className="px-4 py-3 text-left font-medium">
                              {t("grading_col_submitted")}
                            </th>
                            <th className="px-4 py-3 text-left font-medium">
                              {t("grading_col_ungraded")}
                            </th>
                            <th className="px-4 py-3 text-right font-medium" />
                          </tr>
                        </thead>
                        <tbody>
                          {(gradingResp?.data ?? []).map((session) => (
                            <tr
                              key={session.session_id}
                              className="border-t transition-colors hover:bg-muted/40"
                            >
                              <td className="px-4 py-3 font-medium">
                                {session.student_name}
                              </td>
                              <td className="px-4 py-3">
                                {formatScheduled(session.submitted_at)}
                              </td>
                              <td className="px-4 py-3">
                                {session.ungraded_essay_count}
                              </td>
                              <td className="px-4 py-3 text-right">
                                <Button
                                  type="button"
                                  size="sm"
                                  variant="outline"
                                  className="rounded-full"
                                  onClick={() => setSelectedSessionId(session.session_id)}
                                >
                                  {t("competition_view_detail")}
                                </Button>
                              </td>
                            </tr>
                          ))}
                          {(gradingResp?.data.length ?? 0) === 0 && (
                            <tr>
                              <td colSpan={4} className="px-4 py-8 text-center text-muted-foreground">
                                {t("grading_sessions_empty")}
                              </td>
                            </tr>
                          )}
                        </tbody>
                      </table>
                    </div>
                  )}
                </>
              )}

              {selectedSessionId && (
                <>
                  {essaysLoading && (
                    <div className="space-y-2">
                      {Array.from({ length: 2 }).map((_, i) => (
                        <Skeleton key={i} className="h-24 w-full" />
                      ))}
                    </div>
                  )}

                  {essaysIsError && (
                    <div className="rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-destructive">
                      {errorMessage(essaysError, t("error_generic"))}
                    </div>
                  )}

                  {!essaysLoading && !essaysIsError && (
                    <ul className="space-y-4">
                      {(essaysResp?.data ?? []).map((essay) => (
                        <GradingEssayCard
                          key={essay.question_id}
                          essay={essay}
                          input={scoreInputs[essay.question_id] ?? { score: "", comment: "" }}
                          onChange={(next) =>
                            setScoreInputs((prev) => ({ ...prev, [essay.question_id]: next }))
                          }
                          onSave={() => handleSaveEssay(essay)}
                          saving={
                            gradeEssay.isPending &&
                            gradeEssay.variables?.question_id === essay.question_id
                          }
                          t={t}
                        />
                      ))}
                    </ul>
                  )}
                </>
              )}
            </div>
          )}
          {tab === "leaderboard" && (
            <div className="space-y-6">
              {/* Analytics summary */}
              <div className="md-card-outlined p-6">
                <div className="mb-4 flex items-center gap-2">
                  <Trophy className="size-5" />
                  <h2 className="text-title-large font-semibold">
                    {t("admin_exam_detail_tab_leaderboard")}
                  </h2>
                </div>

                {analyticsLoading && (
                  <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                    {Array.from({ length: 3 }).map((_, i) => (
                      <Skeleton key={i} className="h-24 w-full" />
                    ))}
                  </div>
                )}

                {analytics && (
                  <div className="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
                    <div className="rounded-lg border p-4">
                      <div className="text-label text-sm text-muted-foreground">
                        {t("admin_exam_analytics_average_score")}
                      </div>
                      <div className="mt-1 text-2xl font-bold">
                        {analytics.average_score.toFixed(1)}
                      </div>
                    </div>
                    <div className="rounded-lg border p-4">
                      <div className="text-label text-sm text-muted-foreground">
                        {t("admin_exam_analytics_completion_rate")}
                      </div>
                      <div className="mt-1 text-2xl font-bold">
                        {Math.round(analytics.completion_rate * 100)}%
                      </div>
                    </div>
                    <div className="rounded-lg border p-4">
                      <div className="mb-2 text-label text-sm text-muted-foreground">
                        {t("admin_exam_analytics_distribution")}
                      </div>
                      <div className="space-y-1">
                        {analytics.distribution.map((bucket) => (
                          <div
                            key={bucket.label}
                            className="flex justify-between text-sm"
                          >
                            <span>{bucket.label}</span>
                            <span className="font-medium">{bucket.count}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>
                )}
              </div>

              {/* Leaderboard table */}
              <div className="overflow-x-auto md-card-outlined">
                <table className="w-full text-sm">
                  <thead className="bg-muted">
                    <tr>
                      <th className="px-4 py-3 text-left font-medium">
                        {t("admin_exam_leaderboard_col_rank")}
                      </th>
                      <th className="px-4 py-3 text-left font-medium">
                        {t("admin_exam_leaderboard_col_student")}
                      </th>
                      <th className="px-4 py-3 text-right font-medium">
                        {t("admin_exam_leaderboard_col_score")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {lb.isLoading && lbEntries.length === 0 && (
                      <tr>
                        <td
                          colSpan={3}
                          className="px-4 py-8 text-center text-muted-foreground"
                        >
                          {t("sys_loading")}
                        </td>
                      </tr>
                    )}
                    {!lb.isLoading && lbEntries.length === 0 && (
                      <tr>
                        <td
                          colSpan={3}
                          className="px-4 py-8 text-center text-muted-foreground"
                        >
                          {t("admin_exam_leaderboard_empty")}
                        </td>
                      </tr>
                    )}
                    {lbEntries.map((entry) => (
                      <tr
                        key={entry.session_id}
                        className="border-t transition-colors hover:bg-muted/40"
                      >
                        <td className="px-4 py-3 font-medium">
                          #{entry.rank}
                        </td>
                        <td className="px-4 py-3">{entry.student_name}</td>
                        <td className="px-4 py-3 text-right font-medium">
                          {entry.score}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Load more */}
              {lb.data?.next_cursor && (
                <div className="flex justify-center">
                  <Button
                    type="button"
                    variant="outline"
                    className="rounded-full"
                    onClick={handleLoadMore}
                    disabled={lb.isFetching}
                  >
                    {lb.isFetching ? t("sys_loading") : t("sys_load_more")}
                  </Button>
                </div>
              )}
            </div>
          )}
        </>
      )}

      <ExamModal
        open={editOpen}
        exam={data ?? null}
        onClose={() => setEditOpen(false)}
        onSaved={() => {
          setEditOpen(false);
          refetch();
        }}
      />
    </div>
  );
}

function OverviewRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <dt className="text-label text-muted-foreground">{label}</dt>
      <dd className="text-sm">{value}</dd>
    </div>
  );
}

function GradingEssayCard({
  essay,
  input,
  onChange,
  onSave,
  saving,
  t,
}: {
  essay: GradingEssayItem;
  input: { score: string; comment: string };
  onChange: (next: { score: string; comment: string }) => void;
  onSave: () => void;
  saving: boolean;
  t: ReturnType<typeof useTranslation>["t"];
}) {
  return (
    <li className="space-y-3 rounded-md border p-4">
      <div className="text-sm font-medium">
        {stripHtmlToPlainText(essay.body)}
      </div>
      <div className="space-y-1">
        <div className="text-label text-muted-foreground">
          {t("grading_essay_answer_label")}
        </div>
        <div className="whitespace-pre-wrap rounded-md border bg-muted/40 p-3 text-sm">
          {essay.answer?.trim() ? essay.answer : t("grading_essay_no_answer")}
        </div>
      </div>
      <div className="grid gap-3 sm:grid-cols-[160px_1fr]">
        <div className="grid gap-2">
          <Label htmlFor={`grading-score-${essay.question_id}`}>
            {t("grading_score_label")}
          </Label>
          <Input
            id={`grading-score-${essay.question_id}`}
            type="number"
            min={0}
            max={essay.point_correct}
            value={input.score}
            onChange={(e) => onChange({ ...input, score: e.target.value })}
            disabled={saving}
          />
          <div className="text-label text-muted-foreground">
            {t("grading_score_range_hint").replace("{max}", String(essay.point_correct))}
          </div>
        </div>
        <div className="grid gap-2">
          <Label htmlFor={`grading-comment-${essay.question_id}`}>
            {t("grading_comment_label")}
          </Label>
          <textarea
            id={`grading-comment-${essay.question_id}`}
            data-slot="textarea"
            value={input.comment}
            onChange={(e) => onChange({ ...input, comment: e.target.value })}
            placeholder={t("grading_comment_placeholder")}
            rows={2}
            disabled={saving}
            className="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
          />
        </div>
      </div>
      <div className="flex justify-end">
        <Button type="button" size="sm" className="rounded-full" onClick={onSave} disabled={saving}>
          {saving ? t("saving") : t("grading_save")}
        </Button>
      </div>
    </li>
  );
}
