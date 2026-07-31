"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  AlertCircle,
  Maximize2,
  ChevronLeft,
  ChevronRight,
  Flag,
  BookOpen,
} from "lucide-react";
import DOMPurify from "dompurify";

import {
  useReconnectSession,
  useSaveAnswers,
  useSubmitSession,
  useLogViolation,
  useAdvanceSection,
} from "@/lib/hooks/exam";
import { useTranslation, DICT } from "@/lib/i18n";
type I18nKey = keyof typeof DICT.id;
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogHeader,
  DialogFooter,
  DialogClose,
} from "@/components/ui/dialog";
import type { SessionQuestion } from "@/lib/types";
import { RichContent } from "@/components/admin/RichContent";
import { SectionAudioPlayer } from "./section-audio-player";
import { QUESTION_BODY_ALLOWED_TAGS } from "@/lib/question-html";

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

export default function SessionPage() {
  const { t } = useTranslation();
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const sessionId = params?.id ?? "";

  const {
    data: session,
    isLoading,
    isError,
    error,
    refetch,
  } = useReconnectSession(sessionId);
  const saveAnswers = useSaveAnswers(sessionId);
  const submitSession = useSubmitSession(sessionId);
  const logViolation = useLogViolation(sessionId);
  const advanceSection = useAdvanceSection(sessionId);

  const [redirecting, setRedirecting] = useState(false);
  const [fullscreenGranted, setFullscreenGranted] = useState(false);
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [flagged, setFlagged] = useState<Record<string, boolean>>({});
  const [currentQIndex, setCurrentQIndex] = useState(0);
  const [remaining, setRemaining] = useState<number>(0);
  const [showConfirm, setShowConfirm] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [showViolationOverlay, setShowViolationOverlay] = useState(false);
  const autoSubmittedRef = useRef(false);
  const submittingRef = useRef(false);
  const autoAdvanceRef = useRef(false);
  const violationCountRef = useRef(0);
  const answersRef = useRef(answers);
  answersRef.current = answers;
  const flaggedRef = useRef(flagged);
  flaggedRef.current = flagged;
  // Sectioned mode only: the active section's question ids. Null in standard
  // mode. buildSavePayload filters against this so a save never carries answers
  // from a submitted (locked) section — the backend rejects the whole batch
  // otherwise (ErrSectionLocked), silently dropping every section past the first.
  const activeQuestionIdsRef = useRef<Set<string> | null>(null);

  // buildSavePayload unions answered and flagged questions so a flag on an
  // unanswered question still persists server-side.
  const buildSavePayload = useCallback(() => {
    const curAnswers = answersRef.current;
    const curFlags = flaggedRef.current;
    const ids = new Set([
      ...Object.keys(curAnswers),
      ...Object.keys(curFlags).filter((id) => curFlags[id]),
    ]);
    const activeIds = activeQuestionIdsRef.current;
    const scoped = activeIds
      ? [...ids].filter((qid) => activeIds.has(qid))
      : [...ids];
    return scoped.map((qid) => ({
      question_id: qid,
      answer: curAnswers[qid] ?? "",
      flagged_for_review: curFlags[qid] ?? false,
    }));
  }, []);

  const allQuestions = session
    ? session.tests.flatMap((t) => t.questions)
    : [];

  const isSectioned =
    session?.mode === "utbk" || session?.mode === "ielts";
  const activeTest = isSectioned
    ? session?.tests.find((t) => t.id === session.active_test_id)
    : null;
  const activeQuestions =
    isSectioned && activeTest ? activeTest.questions : allQuestions;
  activeQuestionIdsRef.current =
    isSectioned && activeTest
      ? new Set(activeTest.questions.map((q) => q.id))
      : null;

  // Initialize from session data (reconnect)
  useEffect(() => {
    if (!session) return;
    const initAnswers: Record<string, string> = {};
    const initFlags: Record<string, boolean> = {};
    for (const a of session.answers) {
      if (a.answer != null && a.answer !== "") initAnswers[a.question_id] = a.answer;
      if (a.flagged_for_review) initFlags[a.question_id] = true;
    }
    setAnswers(initAnswers);
    setFlagged(initFlags);
    if (isSectioned && session.active_test_id) {
      const sec = session.tests.find((t) => t.id === session.active_test_id);
      setRemaining(sec?.remaining_seconds ?? 0);
    } else {
      setRemaining(session.remaining_seconds);
    }
    autoSubmittedRef.current = false;
    autoAdvanceRef.current = false;
    if (session.status === "submitted") {
      setRedirecting(true);
      router.replace(`/exam/sessions/${sessionId}/result`);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session]);

  // Sectioned mode: land on the new section's first question when it changes,
  // else a shorter next section leaves currentQIndex out of range (blank panel).
  useEffect(() => {
    setCurrentQIndex(0);
  }, [session?.active_test_id]);

  // Untimed exams (timer_mode=per_test → duration_minutes null) get no countdown
  // and must never auto-submit: the backend reports remaining_seconds=0 for them.
  const hasTimer = isSectioned
    ? (activeTest?.duration_minutes ?? 0) > 0
    : session?.duration_minutes != null;

  // Timer countdown
  useEffect(() => {
    if (!session || !hasTimer || session.status !== "in_progress" || remaining <= 0)
      return;
    const id = setInterval(() => {
      setRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, hasTimer, remaining <= 0]);

  // Auto-submit when timer expires (standard mode only)
  useEffect(() => {
    if (
      !session ||
      !hasTimer ||
      session.status !== "in_progress" ||
      remaining > 0 ||
      autoSubmittedRef.current ||
      isSectioned
    )
      return;
    autoSubmittedRef.current = true;
    const doSubmit = async () => {
      submittingRef.current = true;
      const arr = buildSavePayload();
      if (arr.length > 0) {
        try {
          await saveAnswers.mutateAsync(arr);
        } catch {
          /* best-effort */
        }
      }
      submitSession.mutate(undefined, {
        onSuccess: () => {
          setRedirecting(true);
          router.replace(`/exam/sessions/${sessionId}/result`);
        },
      });
    };
    doSubmit();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [remaining <= 0]);

  // Auto-advance when section timer expires (sectioned mode)
  useEffect(() => {
    if (
      !session ||
      !isSectioned ||
      !hasTimer ||
      session.status !== "in_progress" ||
      remaining > 0 ||
      autoAdvanceRef.current
    )
      return;
    autoAdvanceRef.current = true;
    const doAdvance = async () => {
      // Always attempt save before advance; answersRef reflects render-phase
      // state and may not yet include effect-hydrated answers on first fire.
      const arr = buildSavePayload();
      try {
        await saveAnswers.mutateAsync(arr);
      } catch {
        /* best-effort */
      }
      const sectionId = session.active_test_id;
      if (!sectionId) return;
      try {
        const result = await advanceSection.mutateAsync(sectionId);
        if (result.completed) {
          // Last section — submit now
          submitSession.mutate(undefined, {
            onSuccess: () => {
              setRedirecting(true);
              router.replace(`/exam/sessions/${sessionId}/result`);
            },
          });
        }
        // Non-last section: cache invalidation refetches session,
        // init effect picks up the new active section's timer.
      } catch {
        // Allow retry on failure
        autoAdvanceRef.current = false;
      }
    };
    doAdvance();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [remaining <= 0, isSectioned]);

  // Periodic save every 30s
  useEffect(() => {
    if (!sessionId || session?.status !== "in_progress") return;
    const id = setInterval(() => {
      // A save landing while the submit round-trip is in flight could race the
      // grading write server-side — skip autosaves once submit has started.
      if (submittingRef.current) return;
      const arr = buildSavePayload();
      if (arr.length > 0) {
        saveAnswers.mutate(arr);
      }
    }, 30000);
    return () => clearInterval(id);
  }, [sessionId, session?.status, saveAnswers, buildSavePayload]);

  // Violation logging
  useEffect(() => {
    if (!sessionId || session?.status !== "in_progress") return;
    const onFullscreen = () => {
      if (!document.fullscreenElement) {
        logViolation.mutate("fullscreen_exit");
        violationCountRef.current += 1;
        setShowViolationOverlay(true);
      }
    };
    const onVisibility = () => {
      if (document.hidden) {
        logViolation.mutate("tab_switch");
        violationCountRef.current += 1;
        setShowViolationOverlay(true);
      }
    };
    const onCopy = () => logViolation.mutate("copy_attempt");
    document.addEventListener("fullscreenchange", onFullscreen);
    document.addEventListener("visibilitychange", onVisibility);
    document.addEventListener("copy", onCopy);
    return () => {
      document.removeEventListener("fullscreenchange", onFullscreen);
      document.removeEventListener("visibilitychange", onVisibility);
      document.removeEventListener("copy", onCopy);
    };
  }, [sessionId, session?.status, logViolation]);

  // Request fullscreen
  const enterFullscreen = useCallback(async () => {
    try {
      if (document.documentElement.requestFullscreen) {
        await document.documentElement.requestFullscreen();
      }
    } catch {
      /* non-critical */
    }
    setFullscreenGranted(true);
  }, []);

  const handleViolationReturn = useCallback(async () => {
    try {
      if (document.documentElement.requestFullscreen) {
        await document.documentElement.requestFullscreen();
      }
    } catch {
      /* non-critical */
    }
    setShowViolationOverlay(false);
  }, []);

  const setAnswer = useCallback((questionId: string, value: string) => {
    setAnswers((prev) => ({ ...prev, [questionId]: value }));
  }, []);

  const toggleFlag = useCallback((questionId: string) => {
    setFlagged((prev) => ({ ...prev, [questionId]: !prev[questionId] }));
  }, []);

  const handleSubmit = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    submittingRef.current = true;
    const arr = buildSavePayload();
    if (arr.length > 0) {
      try {
        await saveAnswers.mutateAsync(arr);
      } catch {
        /* best-effort */
      }
    }
    submitSession.mutate(undefined, {
      onSuccess: () => {
        setShowConfirm(false);
        setSubmitting(false);
        setRedirecting(true);
        if (document.fullscreenElement) {
          document.exitFullscreen().catch(() => {});
        }
        router.replace(`/exam/sessions/${sessionId}/result`);
      },
      onError: () => {
        setSubmitting(false);
        submittingRef.current = false;
        setShowConfirm(false);
      },
    });
  }, [submitting, saveAnswers, submitSession, router, sessionId, buildSavePayload]);

  // ── Error state (check before !session to handle query error) ────────

  if (isError) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <Card className="border-danger/30 bg-danger-bg px-5 py-4">
          <div className="flex items-center gap-3">
            <AlertCircle className="size-5 text-danger" />
            <div className="flex-1 text-sm text-ink-700">
              {t("sys_error_load")}
              {error instanceof Error && error.message
                ? ` ${error.message}`
                : ""}
            </div>
            <Button variant="outline" size="sm" onClick={() => refetch()}>
              {t("retry")}
            </Button>
          </div>
        </Card>
      </div>
    );
  }

  // ── Loading state ─────────────────────────────────────────────────────

  if (isLoading || !session) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <p className="mb-4 text-sm text-ink-500">{t("sys_loading")}</p>
        <Skeleton className="mb-6 h-8 w-2/3" />
        <Skeleton className="mb-4 h-64 w-full rounded-lg" />
        <Skeleton className="h-10 w-32" />
      </div>
    );
  }

  // ── Redirecting to result ───────────────────────────────────────────────

  if (redirecting) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <p className="mb-4 text-sm text-ink-500">{t("sys_loading")}</p>
        <Skeleton className="h-40 w-full rounded-lg" />
      </div>
    );
  }

  // ── Fullscreen gate ───────────────────────────────────────────────────

  if (!fullscreenGranted) {
    return (
      <div className="mx-auto flex max-w-md flex-col items-center justify-center px-4 py-24 text-center">
        <Maximize2 className="mb-4 size-12 text-brand-600" />
        <h1 className="mb-4 text-xl font-bold text-ink-900">
          {t("fullscreen_required")}
        </h1>
        <Button onClick={enterFullscreen} data-testid="enter-fullscreen">
          <Maximize2 className="size-4" />
          {t("start_exam")}
        </Button>
      </div>
    );
  }

  // ── Active exam ───────────────────────────────────────────────────────

  const questionsToShow = activeQuestions;
  const currentQ = questionsToShow[currentQIndex];
  const answeredCount = Object.keys(answers).length;
  const isFlagged = currentQ ? flagged[currentQ.id] ?? false : false;
  const timerExpired = hasTimer && remaining <= 0;
  // In sectioned mode (utbk/ielts), use the mode label for the top bar title
  // to avoid duplicating the first section's title (which appears as the section label below).
  // In standard mode, the first test's title is a reasonable stand-in for exam heading.
  const examTitle = isSectioned
    ? session?.mode === "utbk"
      ? t("exam_packages_modal_mode_utbk")
      : t("exam_packages_modal_mode_ielts")
    : session.tests[0]?.title ?? "";

  return (
    <div data-testid="exam-overlay" className="fixed inset-0 z-40 flex flex-col bg-background">
      {/* Top bar */}
      <div
        data-testid="exam-top-bar"
        className="flex shrink-0 items-center gap-4 border-b border-line bg-surface-2 px-5 py-3"
      >
        <div className="min-w-0">
          <div className="truncate text-sm font-semibold text-ink-900">
            {examTitle}
          </div>
          {isSectioned && (
            <div className="truncate text-xs text-ink-500">
              {activeTest?.title ?? ""}
            </div>
          )}
        </div>
        <div className="flex-1" />
        <div className="whitespace-nowrap text-xs text-ink-500">
          {answeredCount}/{questionsToShow.length}{" "}
          {t("session_legend_answered").toLowerCase()}
        </div>
        {hasTimer && (
          <div
            className={`rounded-md px-3 py-1 text-lg font-mono font-bold ${
              timerExpired
                ? "bg-danger-bg text-danger"
                : "bg-surface-2 text-ink-900"
            }`}
          >
            {formatTime(remaining)}
          </div>
        )}
        {!isSectioned && (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            onClick={() => setShowConfirm(true)}
            disabled={timerExpired || submitting}
          >
            {t("submit")}
          </Button>
        )}
      </div>

      {/* Body: question pane (1fr) + nav rail (280px) */}
      <div
        data-testid="exam-body"
        className="grid flex-1 grid-cols-[1fr_280px] overflow-hidden"
      >
        {/* Question pane */}
        <div className="overflow-y-auto px-6 py-6">
          <div className="mx-auto max-w-3xl">
            {/* Section rail (sectioned mode only) */}
            {isSectioned && (
              <div
                data-testid="section-rail"
                className="mb-4 flex gap-2 overflow-x-auto"
              >
                {session.tests.map((test, i) => {
                  const isActive = test.id === session.active_test_id;
                  const isSubmitted = test.status === "submitted";
                  let railClass =
                    "flex shrink-0 items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium";
                  if (isActive) {
                    railClass += " bg-brand-600 text-white";
                  } else if (isSubmitted) {
                    railClass += " bg-surface-2 text-ink-500";
                  } else {
                    railClass += " bg-surface-2 text-ink-400";
                  }
                  return (
                    <div
                      key={test.id}
                      data-testid={`section-rail-item-${i}`}
                      className={railClass}
                    >
                      <span>{test.title}</span>
                      <span>
                        {isSubmitted ? "✓" : isActive ? "●" : "○"}
                      </span>
                    </div>
                  );
                })}
              </div>
            )}

            {/* Section audio player */}
            {isSectioned && activeTest?.audio_url && (
              <SectionAudioPlayer
                audioUrl={activeTest.audio_url}
                playLimit={activeTest.audio_play_limit}
                testId="section-audio-player"
              />
            )}

            {/* Per-question audio player */}
            {currentQ?.audio_url && (
              <SectionAudioPlayer
                audioUrl={currentQ.audio_url}
                testId="question-audio-player"
              />
            )}

            {/* Question count + flag toggle */}
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-2 text-sm text-ink-600">
                <BookOpen className="size-4" />
                <span>
                  {t("session_question")} {Math.min(currentQIndex + 1, questionsToShow.length)} {t("of")}{" "}
                  {questionsToShow.length}
                </span>
              </div>
              {currentQ && (
                <Button
                  type="button"
                  variant={isFlagged ? "default" : "outline"}
                  size="sm"
                  onClick={() => toggleFlag(currentQ.id)}
                >
                  <Flag className="size-3.5" />
                  {isFlagged ? t("unflag") : t("flag")}
                </Button>
              )}
            </div>

            {/* Question card */}
            {currentQ && (
              <Card className="mb-4 p-5">
                <div className="mb-2 text-xs uppercase tracking-wide text-ink-500">
                  {t(("fmt_" + currentQ.format) as I18nKey)}
                </div>
                <div className="mb-4 text-base text-ink-900">
                  <RichContent html={currentQ.body} />
                </div>

                {renderAnswerInput(
                  currentQ,
                  answers[currentQ.id] ?? "",
                  (val) => setAnswer(currentQ.id, val),
                  timerExpired,
                )}
              </Card>
            )}

            {/* Navigation buttons */}
            <div className="flex items-center justify-between">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={currentQIndex === 0}
                onClick={() => setCurrentQIndex((i) => Math.max(0, i - 1))}
              >
                <ChevronLeft className="size-4" />
              </Button>

              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={currentQIndex >= questionsToShow.length - 1}
                onClick={() =>
                  setCurrentQIndex((i) =>
                    Math.min(questionsToShow.length - 1, i + 1),
                  )
                }
              >
                <ChevronRight className="size-4" />
              </Button>
            </div>
          </div>
        </div>

        {/* Nav rail */}
        <div
          data-testid="exam-nav-rail"
          className="overflow-y-auto border-l border-line bg-surface-2 p-5"
        >
          <div className="grid grid-cols-5 gap-2">
            {questionsToShow.map((q, i) => {
              const hasAnswer = answers[q.id] != null;
              const isFlagQ = flagged[q.id] ?? false;
              const isCurrent = i === currentQIndex;

              let cellClass = "flex size-8 items-center justify-center rounded-md text-xs font-medium transition-colors";
              if (isCurrent) {
                cellClass += " bg-brand-600 text-white";
              } else if (hasAnswer && isFlagQ) {
                cellClass += " border border-warning/30 bg-warning-bg text-warning";
              } else if (hasAnswer) {
                cellClass += " bg-brand-50 text-brand-700";
              } else if (isFlagQ) {
                cellClass += " border border-warning/30 text-warning";
              } else {
                cellClass += " bg-surface-2 text-ink-600 hover:bg-surface-3";
              }

              return (
                <button
                  key={q.id}
                  type="button"
                  onClick={() => setCurrentQIndex(i)}
                  className={cellClass}
                  data-testid={`session-nav-${i}`}
                >
                  {i + 1}
                </button>
              );
            })}
          </div>

          {/* Legend */}
          <div className="mt-5 flex flex-col gap-2">
            <LegendItem
              swatchClassName="bg-brand-600"
              label={t("session_legend_answered")}
            />
            <LegendItem
              swatchClassName="border border-line bg-surface"
              label={t("session_legend_not_answered")}
            />
            <LegendItem
              swatchClassName="border border-warning/30 bg-warning-bg"
              label={t("session_legend_flagged")}
            />
          </div>
        </div>
      </div>

      {/* Submit confirmation dialog */}
      <Dialog open={showConfirm} onOpenChange={setShowConfirm}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("submit_confirm")}</DialogTitle>
            <DialogDescription>
              {answeredCount}/{questionsToShow.length} {t("session_question").toLowerCase()}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <DialogClose asChild>
              <Button variant="outline">{t("cancel")}</Button>
            </DialogClose>
            <Button
              variant="destructive"
              onClick={handleSubmit}
              disabled={submitting}
            >
              {submitting ? t("sys_loading") : t("submit")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Violation warning overlay */}
      {showViolationOverlay && (
        <div
          data-testid="violation-overlay"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/50"
        >
          <Card className="mx-4 max-w-md p-6">
            <h2 className="mb-4 text-lg font-bold text-ink-900">
              {t("violation_warning")}
            </h2>
            <p className="mb-6 text-sm text-ink-600">
              {t("violation_warning_body").replace(
                "{n}",
                String(violationCountRef.current),
              )}
            </p>
            <Button
              onClick={handleViolationReturn}
              className="w-full"
              data-testid="violation-return-button"
            >
              {t("return_to_exam")}
            </Button>
          </Card>
        </div>
      )}
    </div>
  );
}

function LegendItem({
  swatchClassName,
  label,
}: {
  swatchClassName: string;
  label: string;
}) {
  return (
    <div className="flex items-center gap-2 text-xs text-ink-500">
      <span className={`size-4 rounded ${swatchClassName}`} />
      {label}
    </div>
  );
}

// Helper: sanitize HTML using same allowlist as RichContent
const ALLOWED_TAGS = QUESTION_BODY_ALLOWED_TAGS;
// Must stay identical to RichContent's list: this pass runs first, so anything
// missing here is already gone before RichContent's own allowlist is consulted.
const ALLOWED_ATTR = ["src", "alt", "style", "colspan", "rowspan"];

function sanitizeForRichContent(html: string): string {
  return DOMPurify.sanitize(html, { ALLOWED_TAGS, ALLOWED_ATTR });
}

// Component: render multi_blank with inline inputs
function MultiBlankInput({
  sanitizedHtml,
  blanks,
  currentValue,
  onChange,
  disabled,
}: {
  sanitizedHtml: string;
  blanks: number[] | undefined;
  currentValue: string;
  onChange: (val: string) => void;
  disabled: boolean;
}) {
  const containerRef = useRef<HTMLDivElement>(null);
  const blankValuesRef = useRef<string[]>([]);

  // Parse the current value as JSON array and sync to ref
  // This effect ONLY syncs the ref, does not rebuild DOM
  useEffect(() => {
    let parsed: string[] = [];
    if (currentValue) {
      try {
        parsed = JSON.parse(currentValue);
        if (!Array.isArray(parsed)) {
          parsed = [];
        }
      } catch {
        parsed = [];
      }
    }

    // Pad array to match blank count
    if (blanks) {
      while (parsed.length < blanks.length) {
        parsed.push("");
      }
    }

    blankValuesRef.current = parsed;
  }, [currentValue, blanks]);

  // Build DOM structure only when structure changes (sanitized HTML, blank count, disabled state)
  // NOT when values change (currentValue)
  useEffect(() => {
    const container = containerRef.current;
    if (!container || !blanks || blanks.length === 0) return;

    // Clear and set initial HTML
    container.innerHTML = sanitizedHtml;

    // Walk text nodes looking for {{N}} tokens
    const walker = document.createTreeWalker(
      container,
      NodeFilter.SHOW_TEXT,
      null,
    );

    const nodesToProcess: Array<{
      node: Node;
      matches: Array<{ text: string; num: number }>;
    }> = [];
    let currentNode: Node | null;

    while ((currentNode = walker.nextNode())) {
      const text = currentNode.textContent || "";
      const regex = /\{\{(\d+)\}\}/g;
      const matches: Array<{ text: string; num: number }> = [];
      let match;

      while ((match = regex.exec(text)) !== null) {
        matches.push({
          text: match[0],
          num: parseInt(match[1], 10),
        });
      }

      if (matches.length > 0) {
        nodesToProcess.push({ node: currentNode, matches });
      }
    }

    // Process in reverse order to avoid node shifting
    for (let i = nodesToProcess.length - 1; i >= 0; i--) {
      const { node, matches } = nodesToProcess[i];
      const text = node.textContent || "";
      let lastIndex = 0;
      const fragment = document.createDocumentFragment();

      for (const m of matches) {
        const index = text.indexOf(m.text, lastIndex);
        if (index !== -1) {
          // Add text before the token
          if (index > lastIndex) {
            fragment.appendChild(
              document.createTextNode(text.slice(lastIndex, index)),
            );
          }

          // Create input for this blank (only if token is in 1..N range)
          const blankIndex = m.num - 1;
          if (blankIndex >= 0 && blankIndex < blanks.length) {
            const input = document.createElement("input");
            input.type = "text";
            // Read from ref, which always has the latest values
            input.value = blankValuesRef.current[blankIndex] || "";
            input.disabled = disabled;
            input.className =
              "mx-1 inline-block w-20 border-b-2 border-brand-500 bg-transparent text-sm text-ink-900 outline-none disabled:opacity-60 disabled:cursor-not-allowed";
            input.addEventListener("change", () => {
              // Read current values from ref (always latest, no stale closure)
              const newValues = [...blankValuesRef.current];
              newValues[blankIndex] = input.value;
              onChange(JSON.stringify(newValues));
            });
            input.addEventListener("input", () => {
              // Read current values from ref (always latest, no stale closure)
              const newValues = [...blankValuesRef.current];
              newValues[blankIndex] = input.value;
              onChange(JSON.stringify(newValues));
            });
            fragment.appendChild(input);
          }

          lastIndex = index + m.text.length;
        }
      }

      // Add remaining text
      if (lastIndex < text.length) {
        fragment.appendChild(document.createTextNode(text.slice(lastIndex)));
      }

      // Replace the node
      node.parentNode?.replaceChild(fragment, node);
    }
  }, [sanitizedHtml, blanks, disabled]);

  if (!blanks || blanks.length === 0) {
    return <RichContent html={sanitizedHtml} />;
  }

  return <div ref={containerRef} className="text-base text-ink-900" />;
}

// Component: render true_false with one true/false control per statement.
// Serialises to a JSON array of "true"/"false"/"" in statement index order.
function TrueFalseInput({
  statements,
  currentValue,
  onChange,
  disabled,
}: {
  statements: { index: number; body: string }[] | undefined;
  currentValue: string;
  onChange: (val: string) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  if (!statements || statements.length === 0) return null;

  let parsed: string[] = [];
  try {
    const p = currentValue ? JSON.parse(currentValue) : [];
    if (Array.isArray(p)) parsed = p;
  } catch {
    parsed = [];
  }
  while (parsed.length < statements.length) parsed.push("");

  const setStatement = (i: number, val: "true" | "false") => {
    const next = [...parsed];
    next[i] = val;
    onChange(JSON.stringify(next));
  };

  return (
    <div className="space-y-2">
      {statements.map((s, i) => (
        <div
          key={s.index}
          data-testid={`tf-statement-${s.index}`}
          className="flex items-center justify-between gap-3 rounded-lg border border-line p-3"
        >
          <div className="flex-1 text-sm text-ink-800">
            <RichContent html={sanitizeForRichContent(s.body)} />
          </div>
          <div className="flex items-center gap-3 text-sm">
            <label className="flex items-center gap-1">
              <input
                type="radio"
                name={`tf-${s.index}`}
                data-testid={`tf-radio-true-${s.index}`}
                checked={parsed[i] === "true"}
                onChange={() => setStatement(i, "true")}
                disabled={disabled}
                className="accent-brand-600"
              />
              {t("tests_field_statement_is_true")}
            </label>
            <label className="flex items-center gap-1">
              <input
                type="radio"
                name={`tf-${s.index}`}
                data-testid={`tf-radio-false-${s.index}`}
                checked={parsed[i] === "false"}
                onChange={() => setStatement(i, "false")}
                disabled={disabled}
                className="accent-brand-600"
              />
              {t("tests_field_statement_is_false")}
            </label>
          </div>
        </div>
      ))}
    </div>
  );
}

function renderAnswerInput(
  question: SessionQuestion,
  currentValue: string,
  onChange: (val: string) => void,
  disabled: boolean,
) {
  const { format, options, blanks } = question;

  if (format === "mcq") {
    return (
      <div className="space-y-2">
        {options.map((opt) => (
          <label
            key={opt.key}
            className={`flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors ${
              currentValue === opt.key
                ? "border-brand-500 bg-brand-50"
                : "border-line hover:bg-surface-2"
            } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
          >
            <input
              type="radio"
              name={`q-${question.id}`}
              value={opt.key}
              checked={currentValue === opt.key}
              onChange={() => onChange(opt.key)}
              disabled={disabled}
              className="size-4 accent-brand-600"
            />
            <div className="text-sm text-ink-800">
              <RichContent html={sanitizeForRichContent(opt.text)} />
            </div>
          </label>
        ))}
      </div>
    );
  }

  if (format === "multi_answer") {
    const selectedKeys = currentValue
      ? currentValue.split(",").filter(Boolean)
      : [];
    const toggle = (key: string) => {
      const next = selectedKeys.includes(key)
        ? selectedKeys.filter((k) => k !== key)
        : [...selectedKeys, key];
      onChange(next.sort().join(","));
    };
    return (
      <div className="space-y-2">
        {options.map((opt) => (
          <label
            key={opt.key}
            className={`flex cursor-pointer items-center gap-3 rounded-lg border p-3 transition-colors ${
              selectedKeys.includes(opt.key)
                ? "border-brand-500 bg-brand-50"
                : "border-line hover:bg-surface-2"
            } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
          >
            <input
              type="checkbox"
              checked={selectedKeys.includes(opt.key)}
              onChange={() => toggle(opt.key)}
              disabled={disabled}
              className="size-4 accent-brand-600"
            />
            <div className="text-sm text-ink-800">
              <RichContent html={sanitizeForRichContent(opt.text)} />
            </div>
          </label>
        ))}
      </div>
    );
  }

  if (format === "multi_blank") {
    const sanitized = sanitizeForRichContent(question.body);
    return (
      <MultiBlankInput
        sanitizedHtml={sanitized}
        blanks={blanks}
        currentValue={currentValue}
        onChange={onChange}
        disabled={disabled}
      />
    );
  }

  if (format === "short" || format === "fill_blank") {
    return (
      <input
        type="text"
        value={currentValue}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        className="w-full rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 disabled:opacity-60"
      />
    );
  }

  if (format === "true_false") {
    return (
      <TrueFalseInput
        statements={question.statements}
        currentValue={currentValue}
        onChange={onChange}
        disabled={disabled}
      />
    );
  }

  if (format === "essay") {
    return (
      <textarea
        value={currentValue}
        onChange={(e) => onChange(e.target.value)}
        disabled={disabled}
        rows={5}
        className="w-full resize-y rounded-lg border border-line bg-background px-3 py-2 text-sm text-ink-900 outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500 disabled:opacity-60"
      />
    );
  }

  return null;
}
