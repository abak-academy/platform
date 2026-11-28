"use client";

import { Suspense, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import Link from "next/link";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import {
  ArrowDown,
  ArrowRight,
  ArrowUp,
  Calendar,
  Clock,
  Eye,
  ListChecks,
  Plus,
  SlidersHorizontal,
  Trash2,
  Trophy,
  Users,
  type LucideIcon,
} from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { ResultsWorkspaceTab } from "@/components/admin/ResultsWorkspaceTab";
import { CertificateDesignTab } from "@/components/admin/CertificateDesignTab";
import { ExamAdditionalRulesCard, ExamWhatsAppCard } from "@/components/admin/ExamEventInfo";
import { ExamAddTestDialog } from "@/components/admin/ExamAddTestDialog";
import { ExamModal } from "@/components/admin/ExamModal";
import { ExamRegistrationsTab } from "@/components/admin/ExamRegistrationsTab";
import { CompetitionResultPanel } from "@/components/admin/CompetitionResultPanel";
import { ExamResultsTab } from "@/components/admin/ExamResultsTab";
import { UnderMaintenance } from "@/components/admin/UnderMaintenance";
import { Button } from "@/components/ui/button";
import { DataTable, type DataTableColumn } from "@/components/ui/data-table";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { TabCount, Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  useExam,
  useExamAnalytics,
  useExamLeaderboard,
  useGradeEssay,
  useExamRegistrationCounts,
  useGradingSessions,
  useReplaceExamTests,
  useSessionEssays,
  useSetCertificateEnabled,
  useSetCardEnabled,
} from "@/lib/hooks/admin-exams";
import { useAdminTests } from "@/lib/hooks/admin-tests";
import { hasCapability } from "@/lib/hooks/use-capability";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { isHttpUrl } from "@/lib/exam-event-info";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import { useAuthStore } from "@/stores/auth";
import type {
  ExamLeaderboardEntry,
  ExamResultConfig,
  GradingEssayItem,
  GradingSessionItem,
  Test,
} from "@/lib/types";

// admin_school gets scoped operational tabs, never content-management tabs.
const SCHOOL_SCOPED_TABS: Tab[] = ["overview", "registrations", "results"];

// Price/status/publish for an exam-type product live on the attached Product(s),
// managed via /admin/products (mirrors Course) — not here.
type Tab =
  | "overview"
  | "tests"
  | "certificate"
  | "registrations"
  | "results"
  | "grading"
  | "leaderboard"
  | "results_workspace";

const TAB_ORDER: Tab[] = [
  "overview",
  "tests",
  "certificate",
  "registrations",
  "results",
  "grading",
  "leaderboard",
];

// Roles holding results-workspace:read get the participant-centric Results
// workspace (Issue 124) in place of the separate Results/Leaderboard tabs.
const RESULTS_WORKSPACE_TABS: Tab[] = [
  "overview",
  "tests",
  "certificate",
  "registrations",
  "results_workspace",
  "grading",
];

const RESULT_CONFIG_KEYS = {
  hidden: "exam_packages_modal_result_config_hidden",
  score_only: "exam_packages_modal_result_config_score_only",
  score_pembahasan: "exam_packages_modal_result_config_score_pembahasan",
} as const satisfies Record<ExamResultConfig, string>;

function resultConfigKey(value?: string | null) {
  return value && value in RESULT_CONFIG_KEYS ? RESULT_CONFIG_KEYS[value as ExamResultConfig] : null;
}

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

export default function ExamPackageDetailPage() {
  return (
    <Suspense fallback={null}>
      <ExamPackageDetail />
    </Suspense>
  );
}

function ExamPackageDetail() {
  const params = useParams<{ id: string }>();
  const tabParam = useSearchParams()?.get("tab");
  const id = params?.id ?? "";
  const { t } = useTranslation();
  const role = useAuthStore((s) => s.user?.role);
  const isSchoolScoped = role === "admin_school";
  const hasResultsWorkspace = hasCapability(role, "results-workspace:read");
  const canWriteCompetitionResults = hasCapability(role, "competition-results:write");
  const canPreviewStudent = hasCapability(role, "exam-preview:read");
  const canAccessResultsAndRegistrations =
    role === "admin_school" || role === "super_admin" || role === "admin_exam";

  const [pickedTab, setTab] = useState<Tab | null>(null);
  const [editOpen, setEditOpen] = useState(false);
  const [addOpen, setAddOpen] = useState(false);
  const [confirmDisableCertificate, setConfirmDisableCertificate] = useState(false);

  const { data, isLoading, isError, error, refetch } = useExam(id);
  const { data: registrationCounts } = useExamRegistrationCounts(id);
  const registeredCount = registrationCounts ? registrationCounts.total - registrationCounts.revoked : undefined;
  const policyLocked = data?.format_version === 2 && data.has_sessions === true;
  const replaceTests = useReplaceExamTests(id);
  const setCertificateEnabled = useSetCertificateEnabled(id);
  const setCardEnabled = useSetCardEnabled(id);

  const visibleTabs = (
    isSchoolScoped ? SCHOOL_SCOPED_TABS : hasResultsWorkspace ? RESULTS_WORKSPACE_TABS : TAB_ORDER
  ).filter((key) => key !== "certificate" || data?.certificate_enabled);
  const tab: Tab = pickedTab ?? (visibleTabs.find((key) => key === tabParam) ?? "overview");

  async function handleEnableCertificate() {
    try {
      await setCertificateEnabled.mutateAsync(true);
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }

  async function handleConfirmDisableCertificate() {
    try {
      await setCertificateEnabled.mutateAsync(false);
      setConfirmDisableCertificate(false);
    } catch (err) {
      toast.error(errorMessage(err, t("error_generic")));
    }
  }
  const [testSearch, setTestSearch] = useState("");
  const testQ = useDebouncedValue(testSearch).trim();
  const [testCursor, setTestCursor] = useState<string | undefined>(undefined);
  const [testEntries, setTestEntries] = useState<Test[]>([]);
  const [addedTests, setAddedTests] = useState<Record<string, Test>>({});

  const {
    data: availableResp,
    isLoading: availableLoading,
    isFetching: availableFetching,
  } = useAdminTests({ q: testQ || undefined, cursor: testCursor }, !isSchoolScoped);

  const [lbEntries, setLbEntries] = useState<ExamLeaderboardEntry[]>([]);
  const [lbCursor, setLbCursor] = useState<string | undefined>(undefined);

  const legacyLeaderboardEnabled = !isSchoolScoped && !hasResultsWorkspace;
  const { data: analytics, isLoading: analyticsLoading } = useExamAnalytics(id, legacyLeaderboardEnabled);
  const lb = useExamLeaderboard(
    id,
    lbCursor ? { cursor: lbCursor, limit: 20 } : { limit: 20 },
    legacyLeaderboardEnabled,
  );

  interface PendingSection {
    id: string;
    title: string;
    subject: string;
    section_type?: string;
    duration_minutes: number;
  }

  const [presentations, setPresentations] = useState<Record<string, "one_per_page" | "all_on_page">>({});
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
    setPresentations(Object.fromEntries(data.tests.map(entry => [entry.test_id, entry.question_presentation ?? "one_per_page"])));
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

  function handleAddTest(testId: string) {
    const added = testEntries.find((test) => test.id === testId);
    if (added) setAddedTests((prev) => ({ ...prev, [testId]: added }));
    setAttachedIds((prev) => [...prev, testId]);
  }

  function handleRemoveTest(testId: string) {
    setAttachedIds((prev) => prev.filter((entry) => entry !== testId));
  }

  function handleMoveTest(index: number, delta: -1 | 1) {
    setAttachedIds((prev) => {
      const target = index + delta;
      if (target < 0 || target >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
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
      await replaceTests.mutateAsync(data?.format_version === 2 ? attachedIds.map(test_id => ({ test_id, question_presentation: presentations[test_id] ?? "one_per_page" })) : attachedIds);
      toast.success(t("changes_saved"));
      refetch();
    } catch (e) {
      if (e instanceof ApiError && e.code === "exam_has_sessions") {
        toast.error(t("exam_admin_has_sessions_error"));
        refetch();
        return;
      }
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

  // Reset test-picker pagination when exam or search changes
  useEffect(() => {
    setTestCursor(undefined);
    setTestEntries([]);
  }, [id, testQ]);

  // Accumulate leaderboard pages
  useEffect(() => {
    if (!lb.data) return;
    if (!lbCursor) {
      setLbEntries(lb.data.data ?? []);
    } else {
      setLbEntries((prev) => [...prev, ...(lb.data.data ?? [])]);
    }
  }, [lb.data]);

  // Accumulate test-picker pages
  useEffect(() => {
    if (!availableResp) return;
    if (!testCursor) {
      setTestEntries(availableResp.data ?? []);
    } else {
      setTestEntries((prev) => [...prev, ...(availableResp.data ?? [])]);
    }
  }, [availableResp]);

  function handleLoadMoreTests() {
    if (availableResp?.next_cursor) {
      setTestCursor(availableResp.next_cursor);
    }
  }

  function handleLoadMore() {
    if (lb.data?.next_cursor) {
      setLbCursor(lb.data.next_cursor);
    }
  }

  const title = data?.title ?? t("exam_packages_page_title");
  const materialConflicts = (data?.format_version === 2 ? attachedIds : []).flatMap((testId, idx) => {
    const entry = data?.tests.find((e) => e.test_id === testId);
    return entry?.material_question_positions?.length && presentations[testId] === "all_on_page"
      ? [
          t("exam_admin_material_conflict")
            .replace("{section}", `#${idx + 1} · ${entry.test.title}`)
            .replace("{positions}", entry.material_question_positions.join(", ")),
        ]
      : [];
  });

  const sectionMeta = (testId: string) =>
    data?.tests.find((e) => e.test_id === testId)?.test ?? addedTests[testId] ?? testEntries.find((test) => test.id === testId);
  const sectionMetas = attachedIds.map(sectionMeta);
  const totalQuestions = sectionMetas.reduce((sum, meta) => sum + (meta?.question_count ?? 0), 0);
  const totalMinutes = sectionMetas.reduce((sum, meta) => sum + (meta?.duration_minutes ?? 0), 0);
  const sectionsDirty =
    data != null &&
    (attachedIds.length !== data.tests.length ||
      attachedIds.some(
        (testId, idx) =>
          testId !== data.tests[idx].test_id ||
          (presentations[testId] ?? "one_per_page") !== (data.tests[idx].question_presentation ?? "one_per_page"),
      ));
  const resultKey = resultConfigKey(data?.result_config);
  const activeLabel = (on?: boolean) => t(on ? "status_label_active" : "status_label_inactive");

  const gradingColumns: DataTableColumn<GradingSessionItem>[] = [
    { key: "student", header: t("grading_col_student"), className: "font-medium", cell: (session) => session.student_name },
    { key: "submitted", header: t("grading_col_submitted"), cell: (session) => formatScheduled(session.submitted_at) },
    { key: "ungraded", header: t("grading_col_ungraded"), cell: (session) => session.ungraded_essay_count },
    {
      key: "action",
      header: "",
      align: "right",
      cell: (session) => (
        <Button
          type="button"
          size="sm"
          variant="outline"
         
          onClick={() => setSelectedSessionId(session.session_id)}
        >
          {t("competition_view_detail")}
        </Button>
      ),
    },
  ];

  const leaderboardColumns: DataTableColumn<ExamLeaderboardEntry>[] = [
    { key: "rank", header: t("admin_exam_leaderboard_col_rank"), className: "font-medium", cell: (entry) => `#${entry.rank}` },
    { key: "student", header: t("admin_exam_leaderboard_col_student"), cell: (entry) => entry.student_name },
    { key: "score", header: t("admin_exam_leaderboard_col_score"), align: "right", className: "font-medium", cell: (entry) => entry.score },
  ];

  return (
    <div className="flex flex-col gap-5 fade-in">
      <AdminPageHeader
        back={{ href: "/admin/exam/packages", label: t("exam_packages_page_title") }}
        title={title}
        description={
          data
            ? [
                registeredCount != null ? t("exam_packages_registered_count").replace("{n}", String(registeredCount)) : null,
                t(data.has_sessions ? "exam_admin_meta_has_sessions" : "exam_admin_meta_no_sessions"),
              ]
                .filter(Boolean)
                .join(" · ")
            : undefined
        }
        actions={
          data && (canPreviewStudent || !isSchoolScoped) ? (
            <>
              {canPreviewStudent && (
                <Button asChild variant="outline">
                  <Link href={`/admin/exam/packages/${id}/preview`}>
                    <Eye className="size-4" />
                    {t("admin_exam_preview_student")}
                  </Link>
                </Button>
              )}
              {!isSchoolScoped && (
                <Button variant="dark" onClick={() => setEditOpen(true)}>
                  <SlidersHorizontal className="size-4" />
                  {t("exam_admin_settings")}
                </Button>
              )}
            </>
          ) : undefined
        }
      />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-[16px] bg-danger-50 p-4 text-danger-600">
          {errorMessage(error, t("error_generic"))}
        </div>
      )}

      {!isLoading && !isError && data && (
        <>
          <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)}>
            <TabsList>
              {visibleTabs.map((key) => (
                <TabsTrigger key={key} value={key}>
                  {t(`admin_exam_detail_tab_${key}` as const)}
                  {key === "tests" && <TabCount>{attachedIds.length}</TabCount>}
                  {key === "registrations" && registeredCount != null && <TabCount>{registeredCount}</TabCount>}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>

          {tab === "overview" && (
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
              <div className="flex min-w-0 flex-col gap-4">
                <AdminCard id="exam-info-title" title={t("exam_admin_overview_title")}>
                  <div className="divide-y divide-line">
                    <OverviewGroup icon={Calendar} title={t("exam_admin_overview_group_schedule")}>
                      <OverviewRow label={t("exam_admin_overview_start")} value={formatScheduled(data.scheduled_at)} />
                      <OverviewRow label={t("exam_admin_overview_end")} value={formatScheduled(data.scheduled_end_at)} />
                    </OverviewGroup>
                    <OverviewGroup icon={Users} title={t("exam_admin_overview_group_access")}>
                      <OverviewRow label={t("exam_admin_overview_free")} value={activeLabel(data.is_free)} />
                      <OverviewRow
                        label={t("exam_packages_overview_max_attempts")}
                        value={
                          data.max_attempts == null
                            ? t("unlimited")
                            : data.max_attempts <= 1
                              ? t("exam_packages_overview_no_retake")
                              : t("exam_admin_overview_attempts_count").replace("{n}", String(data.max_attempts))
                        }
                      />
                      <OverviewRow
                        label={t("exam_admin_overview_checkin")}
                        value={
                          data.requires_checkin
                            ? t("exam_admin_overview_checkin_required")
                            : t("exam_admin_overview_checkin_not_required")
                        }
                      />
                      {data.requires_checkin && (
                        <>
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
                        </>
                      )}
                    </OverviewGroup>
                    <OverviewGroup icon={Eye} title={t("exam_admin_overview_group_results")}>
                      <OverviewRow
                        label={t("exam_packages_overview_result_config")}
                        value={resultKey ? t(resultKey) : "—"}
                      />
                      {data.result_config !== "hidden" && (
                        <OverviewRow
                          label={t("exam_packages_overview_result_release_at")}
                          value={formatScheduled(data.result_release_at)}
                        />
                      )}
                      <OverviewRow label={t("exam_admin_overview_leaderboard")} value={activeLabel(data.allow_leaderboard)} />
                      <OverviewRow
                        label={t("exam_admin_overview_rules")}
                        value={
                          data.additional_rules_url && isHttpUrl(data.additional_rules_url) ? (
                            <a
                              href={data.additional_rules_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="font-medium [overflow-wrap:anywhere] text-brand-700 hover:underline"
                            >
                              {t("exam_admin_overview_rules_link")}
                            </a>
                          ) : data.additional_rules_note ? (
                            t("exam_admin_overview_rules_note")
                          ) : (
                            t("exam_admin_overview_rules_none")
                          )
                        }
                      />
                    </OverviewGroup>
                  </div>
                </AdminCard>

                <AdminCard
                  id="exam-sections-title"
                  title={t("exam_admin_overview_sections_title")}
                  actions={
                    !isSchoolScoped && (
                      <Button type="button" variant="outline" size="sm" onClick={() => setTab("tests")}>
                        {t("exam_admin_overview_sections_manage")}
                      </Button>
                    )
                  }
                >
                  {data.tests.length === 0 ? (
                    <p className="text-sm text-ink-600">{t("exam_admin_overview_sections_empty")}</p>
                  ) : (
                    <ol className="divide-y divide-line">
                      {data.tests.map((entry, idx) => (
                        <li key={entry.id} className="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
                          <div className="flex min-w-0 items-center gap-3">
                            <NumberChip n={idx + 1} />
                            <div className="min-w-0">
                              <div className="truncate text-sm font-medium">{entry.test.title}</div>
                              {data.format_version === 2 && (
                                <div className="text-xs text-ink-600">
                                  {t(entry.question_presentation === "all_on_page" ? "format_all_on_page" : "format_one_per_page")}
                                </div>
                              )}
                            </div>
                          </div>
                          <span className="shrink-0 text-xs text-ink-600">
                            {data.timer_mode === "per_test"
                              ? `${entry.test.duration_minutes ?? 0} ${t("minutes")}`
                              : t("exam_admin_overview_section_timer")}
                          </span>
                        </li>
                      ))}
                    </ol>
                  )}
                </AdminCard>

                {!isSchoolScoped && (
                  <AdminCard
                    id="exam-card-title"
                    title={t("exam_card_enabled_label")}
                    description={t("exam_card_enabled_hint")}
                    actions={
                      <Button
                        type="button"
                        variant={data.card_enabled ? "outline" : "default"}
                        size="sm"
                       
                        disabled={setCardEnabled.isPending}
                        onClick={async () => {
                          try {
                            await setCardEnabled.mutateAsync(!data.card_enabled);
                            toast.success(t("exam_card_enabled_saved"));
                          } catch (err) {
                            toast.error(err instanceof Error ? err.message : t("error_generic"));
                          }
                        }}
                      >
                        {data.card_enabled ? t("exam_card_enabled_on") : t("exam_card_enabled_off")}
                      </Button>
                    }
                  />
                )}

                {!isSchoolScoped && (
                  <AdminCard
                    id="exam-certificate-title"
                    title={t("admin_exam_detail_certificate_status_label")}
                    description={
                      (data.certificate_enabled
                        ? t("admin_exam_detail_certificate_enabled_desc")
                        : t("admin_exam_detail_certificate_disabled_desc")) +
                      (data.certificate_enabled && data.certificate_template
                        ? ` ${t("exam_admin_certificate_template")}: ${data.certificate_template}`
                        : "")
                    }
                    actions={
                      data.certificate_enabled ? (
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                         
                          disabled={setCertificateEnabled.isPending}
                          onClick={() => setConfirmDisableCertificate(true)}
                        >
                          {t("admin_exam_detail_certificate_disable")}
                        </Button>
                      ) : (
                        <Button
                          type="button"
                          size="sm"
                         
                          disabled={setCertificateEnabled.isPending}
                          onClick={handleEnableCertificate}
                        >
                          {t("admin_exam_detail_certificate_enable")}
                        </Button>
                      )
                    }
                  >
                    {confirmDisableCertificate && (
                      <div className="rounded-[16px] bg-danger-50 p-4 text-sm">
                        <p className="font-bold">
                          {t("admin_exam_detail_certificate_disable_confirm_title")}
                        </p>
                        <p className="mt-1 text-ink-600">
                          {t("admin_exam_detail_certificate_disable_confirm_desc")}
                        </p>
                        <div className="mt-3 flex gap-2">
                          <Button
                            type="button"
                            variant="destructive"
                            size="sm"
                           
                            disabled={setCertificateEnabled.isPending}
                            onClick={handleConfirmDisableCertificate}
                          >
                            {t("admin_exam_detail_certificate_disable_confirm_action")}
                          </Button>
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                           
                            onClick={() => setConfirmDisableCertificate(false)}
                          >
                            {t("cancel")}
                          </Button>
                        </div>
                      </div>
                    )}
                  </AdminCard>
                )}
              </div>
              <div className="flex min-w-0 flex-col gap-4 lg:sticky lg:top-4 lg:self-start">
                {registeredCount != null && (
                  <AdminCard id="exam-registrants-title" title={t("admin_exam_detail_tab_registrations")}>
                    <div
                      data-testid="exam-registrants-count"
                      className="font-[family-name:var(--font-display)] text-[40px] leading-[1.1] font-extrabold tracking-[-0.03em] text-ink-900"
                    >
                      {registeredCount}
                    </div>
                  </AdminCard>
                )}
                <ExamWhatsAppCard url={data.whatsapp_group_url} canManage={!isSchoolScoped} />
              </div>
            </div>
          )}

          {tab === "tests" && (
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1.6fr)_minmax(0,1fr)]">
              <AdminCard
                id="exam-sections-title"
                title={t("exam_admin_overview_sections_title")}
                description={t("exam_admin_sections_desc")}
                actions={
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                   
                    onClick={() => setAddOpen(true)}
                    disabled={policyLocked}
                  >
                    <Plus className="size-3.5" />
                    {t("exam_admin_sections_add")}
                  </Button>
                }
                footer={
                  <div className="flex flex-wrap items-center justify-between gap-3">
                    <span className="text-sm text-ink-600">
                      {t(sectionsDirty ? "exam_admin_sections_status_dirty" : "exam_admin_sections_status_clean")}
                    </span>
                    <Button
                      type="button"
                     
                      onClick={handleSaveTests}
                      disabled={replaceTests.isPending || policyLocked}
                    >
                      {replaceTests.isPending ? t("saving") : t("admin_exam_detail_tests_save")}
                    </Button>
                  </div>
                }
              >
                {data.format_version !== 2 && data.mode && data.mode !== "standard" && (
                  <div className="mb-3 flex flex-wrap gap-2">
                    {data.mode === "utbk" && (
                      <Button type="button" size="sm" variant="outline" onClick={handleUtbkPreset}>
                        {t("tests_preset_utbk")}
                      </Button>
                    )}
                    {data.mode === "ielts" && (
                      <Button type="button" size="sm" variant="outline" onClick={handleIeltsPreset}>
                        {t("tests_preset_ielts")}
                      </Button>
                    )}
                  </div>
                )}
                {policyLocked && <p className="mb-3 text-sm text-ink-600">{t("exam_admin_policy_locked")}</p>}
                {attachedIds.length === 0 && pendingSections.length === 0 ? (
                  <div className="rounded-[var(--radius-card)] border-2 border-dashed border-line p-6 text-center text-sm text-ink-600">
                    {t("exam_admin_sections_empty")}
                  </div>
                ) : (
                  <ul className="space-y-3">
                    {attachedIds.map((testId, idx) => {
                      const meta = sectionMeta(testId);
                      const entry = data.tests.find((e) => e.test_id === testId);
                      const presentation = presentations[testId] ?? "one_per_page";
                      const materialCount = entry?.material_question_positions?.length ?? 0;
                      const title = meta?.title ?? testId;
                      return (
                        <li key={`${testId}-${idx}`} className="rounded-[16px] bg-surface-sunken px-3.5 py-3">
                          <div className="flex items-start gap-3">
                            <NumberChip n={idx + 1} />
                            <div className="min-w-0 flex-1">
                              <h3 className="text-sm font-semibold [overflow-wrap:anywhere]">
                                <Link href={`/admin/exam/tests/${testId}`} className="hover:underline">{title}</Link>
                              </h3>
                              {meta && (
                                <p className="text-xs text-ink-600">
                                  {meta.subject} · {t("exam_admin_section_questions").replace("{n}", String(meta.question_count ?? 0))}
                                </p>
                              )}
                            </div>
                            <div className="flex shrink-0 items-center gap-1">
                              <Button
                                type="button"
                                size="icon-xs"
                                variant="ghost"
                               
                                onClick={() => handleMoveTest(idx, -1)}
                                disabled={policyLocked || idx === 0}
                                aria-label={`${t("exam_admin_section_move_up")} ${title}`}
                              >
                                <ArrowUp className="size-3" />
                              </Button>
                              <Button
                                type="button"
                                size="icon-xs"
                                variant="ghost"
                               
                                onClick={() => handleMoveTest(idx, 1)}
                                disabled={policyLocked || idx === attachedIds.length - 1}
                                aria-label={`${t("exam_admin_section_move_down")} ${title}`}
                              >
                                <ArrowDown className="size-3" />
                              </Button>
                              <Button
                                type="button"
                                size="icon-xs"
                                variant="ghost"
                               
                                onClick={() => handleRemoveTest(testId)}
                                disabled={policyLocked}
                                aria-label={`${t("admin_exam_detail_tests_remove")} ${title}`}
                              >
                                <Trash2 className="size-3" />
                              </Button>
                            </div>
                          </div>
                          {data.format_version === 2 && (
                            <>
                              <div className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_160px]">
                                <div className="grid content-start gap-1.5">
                                  <span className="text-xs text-ink-600">{t("format_presentation")}</span>
                                  <select
                                    aria-label={`${t("format_presentation")} ${title}`}
                                    aria-describedby={`presentation-hint-${testId}`}
                                    className="h-[var(--spacing-control-sm)] w-full rounded-full border-[1.5px] border-line bg-surface px-3 text-[13px] font-bold"
                                    value={presentation}
                                    onChange={(e) => setPresentations((prev) => ({ ...prev, [testId]: e.target.value as "one_per_page" | "all_on_page" }))}
                                    disabled={replaceTests.isPending || policyLocked}
                                  >
                                    <option value="one_per_page">{t("format_one_per_page")}</option>
                                    <option value="all_on_page">{t("format_all_on_page")}</option>
                                  </select>
                                  <small id={`presentation-hint-${testId}`} className="text-xs text-ink-600">
                                    {t(presentation === "one_per_page" ? "exam_admin_presentation_one_hint" : "exam_admin_presentation_all_hint")}
                                  </small>
                                </div>
                                <div className="grid content-start gap-1.5">
                                  <span className="text-xs text-ink-600">{t("exam_admin_section_time")}</span>
                                  <strong className="text-sm">
                                    {data.timer_mode === "per_test"
                                      ? `${meta?.duration_minutes ?? 0} ${t("minutes")}`
                                      : t("exam_admin_section_time_overall")}
                                  </strong>
                                  <small className="text-xs text-ink-600">
                                    {data.timer_mode === "per_test"
                                      ? t("exam_admin_section_time_from_test")
                                      : t("exam_admin_section_time_overall_note").replace("{n}", String(data.duration_minutes ?? 0))}
                                  </small>
                                </div>
                              </div>
                              <div className="mt-4 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-3 text-xs text-ink-600">
                                <span>
                                  {entry && t(entry.test.has_shared_material ? "exam_admin_section_material" : "exam_admin_section_no_material")}
                                  {materialCount > 0 && ` · ${t("exam_admin_section_material_per_question").replace("{n}", String(materialCount))}`}
                                </span>
                                <Link href={`/admin/exam/tests/${testId}`} className="font-medium text-brand-700 hover:underline">
                                  {t("exam_admin_section_source")}
                                </Link>
                              </div>
                            </>
                          )}
                        </li>
                      );
                    })}
                  </ul>
                )}
                {pendingSections.length > 0 && (
                  <div className="mt-3">
                    <h4 className="text-label mb-2 text-sm font-medium text-ink-600">
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
                            <div className="text-label text-ink-600">
                              {ps.subject} · {ps.section_type ? `${ps.section_type} · ` : ""}
                              {ps.duration_minutes} {t("minutes")}
                            </div>
                          </div>
                          <Button
                            type="button"
                            size="icon-xs"
                            variant="ghost"
                           
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
              </AdminCard>

              <div className="min-w-0 space-y-4 lg:sticky lg:top-4 lg:self-start">
                <ExamAdditionalRulesCard
                  exam={data}
                  summary={
                    data.format_version === 2 && (
                      <div className="space-y-3">
                        <RuleLine
                          icon={ArrowRight}
                          title={t(data.navigation_policy === "sequential" ? "format_navigation_sequential" : "format_navigation_free")}
                          body={t(data.navigation_policy === "sequential" ? "exam_admin_rules_nav_sequential" : "exam_admin_rules_nav_free")}
                        />
                        <RuleLine
                          icon={Clock}
                          title={t(data.timer_mode === "per_test" ? "format_timer_section" : "format_timer_overall")}
                          body={
                            data.timer_mode === "per_test"
                              ? t("exam_admin_rules_timer_per_test")
                                  .replace("{minutes}", String(totalMinutes))
                                  .replace("{sections}", String(attachedIds.length))
                              : t("exam_admin_rules_timer_overall").replace("{n}", String(data.duration_minutes ?? 0))
                          }
                        />
                        <RuleLine
                          icon={ListChecks}
                          title={t("exam_admin_rules_counts")
                            .replace("{questions}", String(totalQuestions))
                            .replace("{sections}", String(attachedIds.length))}
                          body={`${t(data.randomize ? "exam_admin_rules_randomized" : "exam_admin_rules_in_order")} ${t("exam_admin_rules_presentation_per_section")}`}
                        />
                      </div>
                    )
                  }
                  footnote={t("exam_admin_rules_footnote")}
                />
                {data.format_version === 2 &&
                  (materialConflicts.length > 0 ? (
                    <div role="alert" className="rounded-[16px] bg-danger-50 p-4 text-sm text-danger-600">
                      <strong className="block">{t("exam_admin_valid_conflict_title")}</strong>
                      {materialConflicts.map((message) => (
                        <p key={message} className="mt-1">{message}</p>
                      ))}
                    </div>
                  ) : attachedIds.length > 0 ? (
                    <div className="rounded-[16px] bg-teal-50 p-4 text-sm text-teal-700">
                      <strong className="block">{t("exam_admin_valid_title")}</strong>
                      {t("exam_admin_valid_body")}
                    </div>
                  ) : (
                    <div className="rounded-[16px] bg-gold-50 p-4 text-sm text-gold-700">
                      <strong className="block">{t("exam_admin_valid_empty_title")}</strong>
                      {t("exam_admin_valid_empty_body")}
                    </div>
                  ))}
              </div>
            </div>
          )}

          {tab === "certificate" && data.certificate_enabled && (
            <CertificateDesignTab examId={id} exam={data} onSaved={refetch} />
          )}

          {tab === "registrations" && (
            canAccessResultsAndRegistrations ? (
              <ExamRegistrationsTab examId={id} examName={data.title} />
            ) : (
              <UnderMaintenance icon={Users} title={t("admin_exam_detail_tab_registrations")} />
            )
          )}
          {tab === "results" && (
            canAccessResultsAndRegistrations ? (
              <>
                <ExamResultsTab examId={id} />
                <CompetitionResultPanel examId={id} canWrite={canWriteCompetitionResults} />
              </>
            ) : (
              <UnderMaintenance icon={ListChecks} title={t("admin_exam_detail_tab_results")} />
            )
          )}
          {tab === "results_workspace" && hasResultsWorkspace && (
            <>
              <ResultsWorkspaceTab examId={id} />
              <CompetitionResultPanel examId={id} canWrite={canWriteCompetitionResults} />
            </>
          )}
          {tab === "grading" && (
            <div className="md-card-outlined flex flex-col gap-4">
              <div className="flex items-center justify-between">
                <h2 className="type-h2">
                  {t("grading_sessions_title")}
                </h2>
                {selectedSessionId && (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                   
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
                    <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
                      {errorMessage(gradingError, t("error_generic"))}
                    </div>
                  )}

                  {!gradingLoading && !gradingIsError && (
                    <DataTable<GradingSessionItem>
                      columns={gradingColumns}
                      rows={gradingResp?.data ?? []}
                      rowKey={(session) => session.session_id}
                      empty={t("grading_sessions_empty")}
                    />
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
                    <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
                      {errorMessage(essaysError, t("error_generic"))}
                    </div>
                  )}

                  {!essaysLoading && !essaysIsError && (
                    <ul className="flex flex-col gap-4">
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
            <div className="flex flex-col gap-4">
              {/* Analytics summary */}
              <div className="md-card-outlined">
                <div className="mb-4 flex items-center gap-2">
                  <Trophy className="size-5" />
                  <h2 className="type-h2">
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
                    <div className="rounded-[16px] bg-surface-sunken p-4">
                      <div className="text-label text-sm text-ink-600">
                        {t("admin_exam_analytics_average_score")}
                      </div>
                      <div className="mt-1 text-2xl font-bold">
                        {analytics.average_score.toFixed(1)}
                      </div>
                    </div>
                    <div className="rounded-[16px] bg-surface-sunken p-4">
                      <div className="text-label text-sm text-ink-600">
                        {t("admin_exam_analytics_completion_rate")}
                      </div>
                      <div className="mt-1 text-2xl font-bold">
                        {Math.round(analytics.completion_rate * 100)}%
                      </div>
                    </div>
                    <div className="rounded-[16px] bg-surface-sunken p-4">
                      <div className="mb-2 text-label text-sm text-ink-600">
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

              <DataTable<ExamLeaderboardEntry>
                columns={leaderboardColumns}
                rows={lbEntries}
                rowKey={(entry) => entry.session_id}
                empty={
                  lb.isLoading && lbEntries.length === 0
                    ? t("sys_loading")
                    : t("admin_exam_leaderboard_empty")
                }
                footer={
                  lb.data?.next_cursor ? (
                    <div className="flex justify-center p-4">
                      <Button
                        type="button"
                        variant="outline"
                       
                        onClick={handleLoadMore}
                        disabled={lb.isFetching}
                      >
                        {lb.isFetching ? t("sys_loading") : t("sys_load_more")}
                      </Button>
                    </div>
                  ) : undefined
                }
              />
            </div>
          )}
        </>
      )}

      <ExamAddTestDialog
        open={addOpen}
        onClose={() => setAddOpen(false)}
        tests={testEntries}
        attachedIds={attachedIds}
        search={testSearch}
        onSearch={setTestSearch}
        loading={availableLoading}
        fetching={availableFetching}
        total={availableResp?.total}
        hasMore={Boolean(availableResp?.next_cursor)}
        onLoadMore={handleLoadMoreTests}
        onAdd={handleAddTest}
      />

      <ExamModal
        open={editOpen}
        exam={data ?? null}
        policyLocked={policyLocked}
        onSessionsConflict={() => refetch()}
        onClose={() => setEditOpen(false)}
        onSaved={() => {
          setEditOpen(false);
          refetch();
        }}
      />
    </div>
  );
}

function AdminCard({
  id,
  title,
  description,
  actions,
  footer,
  children,
}: {
  id: string;
  title: string;
  description?: string;
  actions?: React.ReactNode;
  footer?: React.ReactNode;
  children?: React.ReactNode;
}) {
  return (
    <section aria-labelledby={id} className="md-card-outlined min-w-0">
      <div className={cn("flex flex-wrap items-start justify-between gap-3", (children || footer) && "mb-3.5")}>
        <div className="min-w-0">
          <h2 id={id} className="type-h2">{title}</h2>
          {description && <p className="mt-1 text-sm text-ink-600">{description}</p>}
        </div>
        {actions}
      </div>
      {children}
      {footer && <div className="mt-4 border-t border-line pt-4">{footer}</div>}
    </section>
  );
}

function OverviewGroup({ icon: Icon, title, children }: { icon: LucideIcon; title: string; children: React.ReactNode }) {
  return (
    <section className="py-5 first:pt-0 last:pb-0">
      <div className="mb-3 flex items-center gap-2">
        <Icon className="size-[18px] text-brand-600" />
        <h3 className="type-h3">{title}</h3>
      </div>
      <dl className="flex flex-col text-sm">{children}</dl>
    </section>
  );
}

function NumberChip({ n }: { n: number }) {
  return (
    <span className="flex size-7 shrink-0 items-center justify-center rounded-[10px] bg-brand-900 font-[family-name:var(--font-display)] text-[13px] font-extrabold text-white">
      {n}
    </span>
  );
}

function RuleLine({ icon: Icon, title, body }: { icon: LucideIcon; title: string; body: string }) {
  return (
    <div className="flex items-start gap-3 text-sm">
      <Icon className="mt-0.5 size-4 shrink-0 text-ink-600" />
      <div className="min-w-0">
        <strong className="block">{title}</strong>
        <p className="text-ink-600">{body}</p>
      </div>
    </div>
  );
}

function OverviewRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-t border-line py-2.5 text-sm leading-[1.55]">
      <dt className="text-ink-600">{label}</dt>
      <dd className="text-right font-bold text-ink-900">{value}</dd>
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
    <li className="flex flex-col gap-3 rounded-[var(--radius-card)] border border-line p-4">
      <div className="text-sm font-medium">
        {stripHtmlToPlainText(essay.body)}
      </div>
      <div className="space-y-1">
        <div className="text-label text-ink-600">
          {t("grading_essay_answer_label")}
        </div>
        <div className="whitespace-pre-wrap rounded-[16px] bg-surface-sunken p-3 text-sm">
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
          <div className="text-label text-ink-600">
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
            className="flex w-full rounded-md border-[1.5px] border-line bg-surface px-3.5 py-3 text-sm outline-none focus-visible:border-brand-600 focus-visible:ring-4 focus-visible:ring-brand-50 disabled:pointer-events-none disabled:opacity-45"
          />
        </div>
      </div>
      <div className="flex justify-end">
        <Button type="button" size="sm" onClick={onSave} disabled={saving}>
          {saving ? t("saving") : t("grading_save")}
        </Button>
      </div>
    </li>
  );
}
