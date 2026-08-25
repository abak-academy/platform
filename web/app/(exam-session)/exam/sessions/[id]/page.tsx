"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { useParams, useRouter } from "next/navigation";
import {
  AlertCircle,
  Maximize2,
  ChevronLeft,
  ChevronRight,
  Clock,
  Flag,
  BookOpen,
  Trophy,
  TriangleAlert,
} from "lucide-react";
import DOMPurify from "dompurify";
import { ApiError } from "@/lib/api";

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
import type { SessionQuestion, SessionAnswerInput } from "@/lib/types";
import { RichContent } from "@/components/admin/RichContent";
import { SectionAudioPlayer } from "./section-audio-player";
import { QUESTION_BODY_ALLOWED_TAGS } from "@/lib/question-html";
import { optionKeyLabel } from "@/lib/option-key";
import {
  loadQueue,
  saveQueue,
  clearQueue,
  backoffDelayMs,
  AUTOSAVE_DEBOUNCE_MS,
} from "@/lib/exam-session-queue";

function OptionKeyBadge({ optionKey }: { optionKey: string }) {
  return (
    <span
      data-testid={`option-key-${optionKey}`}
      className="w-6 shrink-0 text-center font-mono text-sm font-medium uppercase text-ink-600"
    >
      {optionKeyLabel(optionKey)}
    </span>
  );
}

function formatTime(seconds: number): string {
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  return `${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
}

const EXPIRY_MAX_ATTEMPTS = 3;
const VIOLATION_GRACE_MS = 3000;
const VIOLATION_DUPLICATE_SUPPRESS_MS = 5000;

function isTransientExpiryError(error: unknown): boolean {
  if (!(error instanceof ApiError)) return true;
  return error.status === 408 || error.status === 429 || error.status >= 500;
}

function isAlreadySubmittedError(error: unknown): boolean {
  return error instanceof ApiError && error.code === "already_submitted";
}

async function retryExpiryStep<T>(action: () => Promise<T>): Promise<T> {
  for (let attempt = 0; attempt < EXPIRY_MAX_ATTEMPTS; attempt += 1) {
    try {
      return await action();
    } catch (error) {
      if (!isTransientExpiryError(error) || attempt === EXPIRY_MAX_ATTEMPTS - 1) {
        throw error;
      }
      await new Promise((resolve) => setTimeout(resolve, backoffDelayMs(attempt)));
    }
  }
  throw new Error("expiry recovery exhausted");
}

// Identifies a queued/save-payload entry by its full content, not just its
// question id — so an ack for an older value never removes a newer,
// still-unacknowledged value for the same question from the durable queue.
function queueEntryKey(entry: {
  question_id: string;
  answer: string;
  flagged_for_review?: boolean;
}): string {
  // `\0` here is the two-char escape, not a literal NUL — a real 0x00 in
  // this file makes git/GitHub treat the whole .tsx as binary.
  return `${entry.question_id}\0${entry.answer}\0${Boolean(entry.flagged_for_review)}`;
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
  const logViolationRef = useRef(logViolation);
  logViolationRef.current = logViolation;

  const [redirecting, setRedirecting] = useState(false);
  const [fullscreenGranted, setFullscreenGranted] = useState(false);
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [flagged, setFlagged] = useState<Record<string, boolean>>({});
  const [currentQIndex, setCurrentQIndex] = useState(0);
  const [navExpanded, setNavExpanded] = useState(false);
  const [remaining, setRemaining] = useState<number>(0);
  const [showConfirm, setShowConfirm] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [expiryRecoveryFailed, setExpiryRecoveryFailed] = useState(false);
  const [showViolationOverlay, setShowViolationOverlay] = useState(false);
  const [saveStatus, setSaveStatus] = useState<"saved" | "saving" | "unsaved">(
    "saved",
  );
  const autoSubmittedRef = useRef(false);
  const submittingRef = useRef(false);
  const examBodyRef = useRef<HTMLDivElement>(null);
  const questionPaneRef = useRef<HTMLDivElement>(null);
  const navToggleRef = useRef<HTMLButtonElement>(null);
  const autoAdvanceRef = useRef(false);
  const violationCountRef = useRef(0);
  const violationTimersRef = useRef<
    Partial<Record<"fullscreen_exit" | "tab_switch", ReturnType<typeof setTimeout>>>
  >({});
  const lastViolationAtRef = useRef<
    Partial<Record<"fullscreen_exit" | "tab_switch", number>>
  >({});
  const answersRef = useRef(answers);
  answersRef.current = answers;
  const flaggedRef = useRef(flagged);
  flaggedRef.current = flagged;
  // Mirrors `remaining` for effects that must read the freshly-hydrated value
  // within the same mount pass — the hydration effect below writes this ref
  // synchronously (unlike setRemaining, which only applies on the next
  // render), so an effect declared later in that same pass never reads the
  // stale default of 0 and treats an untouched timer as already expired.
  const remainingRef = useRef(remaining);
  remainingRef.current = remaining;
  // Sectioned mode only: the active section's question ids. Null in standard
  // mode. buildSavePayload filters against this so a save never carries answers
  // from a submitted (locked) section — the backend rejects the whole batch
  // otherwise (ErrSectionLocked), silently dropping every section past the first.
  const activeQuestionIdsRef = useRef<Set<string> | null>(null);
  // Mirrors `currentQIndex` for the save pipeline, plus the last position the
  // server acknowledged — used to decide whether a position-only change (no
  // answer edits) still needs to go out (FR-35/FR-36).
  const currentQIndexRef = useRef(currentQIndex);
  currentQIndexRef.current = currentQIndex;
  const lastSavedPositionRef = useRef(0);
  // Guards the reconnect-hydration effect below to run exactly once per
  // mount: every subsequent `session` change is a refetch our OWN save
  // triggered (invalidateQueries), and re-seeding answers/flags/position
  // from that stale-by-definition snapshot would clobber a local edit made
  // after the save was issued but before the refetch landed (FR-37, NFR-R5).
  const hasHydratedRef = useRef(false);
  // Guards the section-change effect further below the same way, for the
  // position/timer reset that effect owns (Task 19 fixed the equivalent bug
  // for standard mode; sectioned mode needs its own guard since it runs on
  // every active_test_id change, including the very first one at mount).
  const sectionMountRef = useRef(true);

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

  // ── Durable autosave: debounce on change, retry with backoff, replay the
  // localStorage queue on reconnect (FR-31..FR-34, FR-37, NFR-P3, NFR-R5) ────
  const debounceTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const retryTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const pendingChangeRef = useRef(false);
  const retryAttemptRef = useRef(0);
  // Sequence number of the most recently ISSUED save. Two saves can still be
  // outstanding at once (the hook serializes the actual network calls, but
  // an older request can still settle its callbacks before a newer one that
  // was dispatched later — see attemptSave below). A stale ack must never
  // repaint the "saved" indicator over a newer save that is still pending or
  // has failed (FR-34, NFR-R5).
  const saveSeqRef = useRef(0);

  const clearRetryTimer = useCallback(() => {
    if (retryTimerRef.current) {
      clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  }, []);

  // Sends payload to the server and retries on failure with exponential
  // backoff until it succeeds. submittingRef guards against racing the
  // submit round-trip (same guard the old 30s tick used).
  const attemptSave = useCallback(
    (payload: SessionAnswerInput[]) => {
      if (submittingRef.current) return;
      const position = currentQIndexRef.current;
      const positionChanged = position !== lastSavedPositionRef.current;
      if (payload.length === 0 && !positionChanged) {
        setSaveStatus("saved");
        return;
      }
      setSaveStatus("saving");
      const mySeq = ++saveSeqRef.current;
      saveAnswers.mutate({ answers: payload, current_position: position }, {
        onSuccess: () => {
          // Drop exactly the queued entries this save's payload matches
          // byte-for-byte — content, not just question id, so a newer
          // unacknowledged edit for the same question (queued by a later,
          // still-outstanding save) is never discarded by an older save's
          // ack (FR-32, NFR-R5).
          const acked = new Set(payload.map(queueEntryKey));
          const stillPending = loadQueue(sessionId).filter(
            (q) => !acked.has(queueEntryKey(q)),
          );
          if (stillPending.length > 0) {
            saveQueue(sessionId, stillPending);
          } else {
            clearQueue(sessionId);
          }
          // A newer save may have been issued since this one started, and
          // may still be pending or may itself have already failed — its
          // outcome owns the indicator and the acknowledged position. A
          // stale ack must never repaint "saved" over that (FR-34).
          if (mySeq === saveSeqRef.current) {
            retryAttemptRef.current = 0;
            lastSavedPositionRef.current = position;
            setSaveStatus("saved");
          }
        },
        onError: () => {
          if (submittingRef.current) return;
          if (mySeq !== saveSeqRef.current) return; // superseded by a newer attempt
          clearRetryTimer();
          const delay = backoffDelayMs(retryAttemptRef.current);
          retryAttemptRef.current += 1;
          setSaveStatus("unsaved");
          retryTimerRef.current = setTimeout(() => {
            retryTimerRef.current = null;
            // Rebuild from current state rather than reusing this attempt's
            // own payload or re-reading localStorage: either can be stale by
            // the time the backoff elapses, and buildSavePayload always
            // reflects whatever the user has actually typed.
            attemptSave(buildSavePayload());
          }, delay);
        },
      });
    },
    [sessionId, saveAnswers, clearRetryTimer, buildSavePayload],
  );

  const flushDebouncedSave = useCallback(() => {
    debounceTimerRef.current = null;
    if (!pendingChangeRef.current) return;
    pendingChangeRef.current = false;
    const payload = buildSavePayload();
    saveQueue(sessionId, payload);
    clearRetryTimer();
    retryAttemptRef.current = 0;
    attemptSave(payload);
  }, [buildSavePayload, sessionId, clearRetryTimer, attemptSave]);

  // At most one save per debounce window (NFR-P3): the first change in a
  // window starts the timer; later changes in the same window just mark
  // pending — flushDebouncedSave reads the latest state via buildSavePayload
  // when the window elapses, then the next change starts a fresh window.
  const scheduleAutosave = useCallback(() => {
    setSaveStatus("unsaved");
    pendingChangeRef.current = true;
    if (debounceTimerRef.current) return;
    debounceTimerRef.current = setTimeout(
      flushDebouncedSave,
      AUTOSAVE_DEBOUNCE_MS,
    );
  }, [flushDebouncedSave]);

  useEffect(() => {
    return () => {
      if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current);
      clearRetryTimer();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
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

  // Initialize from session data (reconnect). Runs exactly once (see
  // hasHydratedRef above) — this is a mount-time reconnect concern, not a
  // "keep local state synced with the server" concern; local edits are the
  // source of truth for `answers`/`flagged` for the rest of the session.
  useEffect(() => {
    if (!session || hasHydratedRef.current) return;
    hasHydratedRef.current = true;
    const initAnswers: Record<string, string> = {};
    const initFlags: Record<string, boolean> = {};
    for (const a of session.answers) {
      if (a.answer != null && a.answer !== "") initAnswers[a.question_id] = a.answer;
      if (a.flagged_for_review) initFlags[a.question_id] = true;
    }
    // Overlay any not-yet-acknowledged local queue on top of server state —
    // a question absent from the queue keeps its server value untouched
    // (FR-37, NFR-R5); a question the queue holds shows the local edit until
    // the replay effect confirms it with the server.
    for (const q of loadQueue(sessionId)) {
      initAnswers[q.question_id] = q.answer;
      if (q.flagged_for_review) initFlags[q.question_id] = true;
      else delete initFlags[q.question_id];
    }
    setAnswers(initAnswers);
    setFlagged(initFlags);
    // Position is always seeded from the server response, never from
    // localStorage — it must survive a reconnect on a different device
    // (FR-36). Written to the ref synchronously for the same reason as
    // remainingRef above.
    const savedPosition = session.current_position ?? 0;
    currentQIndexRef.current = savedPosition;
    lastSavedPositionRef.current = savedPosition;
    setCurrentQIndex(savedPosition);
    if (isSectioned && session.active_test_id) {
      const sec = session.tests.find((t) => t.id === session.active_test_id);
      const nextRemaining = sec?.remaining_seconds ?? 0;
      remainingRef.current = nextRemaining;
      setRemaining(nextRemaining);
    } else {
      remainingRef.current = session.remaining_seconds;
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

  // Replay any queue left over from a previous tab/session, and again
  // whenever connectivity returns (FR-33, FR-37). Declared after the
  // hydration effect above so the UI shows the queued values first — replay
  // clears the queue on ack, and hydration must have already read it.
  useEffect(() => {
    if (!sessionId) return;
    const replay = () => {
      const queued = loadQueue(sessionId);
      if (queued.length > 0) {
        clearRetryTimer();
        retryAttemptRef.current = 0;
        attemptSave(queued);
      }
    };
    replay();
    window.addEventListener("online", replay);
    return () => window.removeEventListener("online", replay);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // Sectioned mode: land on the new section's first question when the active
  // section actually changes (e.g. advancing), else a shorter next section
  // leaves currentQIndex out of range (blank panel). The FIRST commit for a
  // session is the initial mount, not a section change — that position
  // already comes from the server hydration above (FR-36) and must not be
  // stomped back to 0, nor written back to the server as 0 on the next save
  // (lastSavedPositionRef). Standard mode has no sections to reset for.
  useEffect(() => {
    if (!isSectioned) return;
    if (sectionMountRef.current) {
      sectionMountRef.current = false;
      return;
    }
    setCurrentQIndex(0);
    currentQIndexRef.current = 0;
    lastSavedPositionRef.current = 0;
    const nextRemaining = activeTest?.remaining_seconds ?? 0;
    remainingRef.current = nextRemaining;
    setRemaining(nextRemaining);
    autoSubmittedRef.current = false;
    autoAdvanceRef.current = false;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isSectioned, session?.active_test_id]);

  // Untimed exams (timer_mode=per_test → duration_minutes null) get no countdown
  // and must never auto-submit: the backend reports remaining_seconds=0 for them.
  const hasTimer = isSectioned
    ? (activeTest?.duration_minutes ?? 0) > 0
    : session?.duration_minutes != null;

  const runExpiryRecovery = useCallback(async () => {
    if (!session) return;
    setExpiryRecoveryFailed(false);
    setSubmitting(true);
    submittingRef.current = true;
    if (debounceTimerRef.current) clearTimeout(debounceTimerRef.current);
    clearRetryTimer();
    pendingChangeRef.current = false;

    try {
      const payload = buildSavePayload();
      try {
        await retryExpiryStep(() =>
          saveAnswers.mutateAsync({
            answers: payload,
            current_position: currentQIndexRef.current,
          }),
        );
        clearQueue(sessionId);
      } catch (error) {
        if (isAlreadySubmittedError(error)) throw error;
      }

      if (isSectioned) {
        const sectionId = session.active_test_id;
        if (!sectionId) return;
        const result = await retryExpiryStep(() =>
          advanceSection.mutateAsync(sectionId),
        );
        if (!result.completed) {
          setSubmitting(false);
          submittingRef.current = false;
          return;
        }
      }

      await retryExpiryStep(() => submitSession.mutateAsync());
      clearQueue(sessionId);
      setRedirecting(true);
      router.replace(`/exam/sessions/${sessionId}/result`);
    } catch (error) {
      if (isAlreadySubmittedError(error)) {
        clearQueue(sessionId);
        setRedirecting(true);
        router.replace(`/exam/sessions/${sessionId}/result`);
        return;
      }
      setSubmitting(false);
      submittingRef.current = false;
      setExpiryRecoveryFailed(true);
    }
  }, [
    session,
    isSectioned,
    sessionId,
    saveAnswers,
    advanceSection,
    submitSession,
    router,
    buildSavePayload,
    clearRetryTimer,
  ]);

  // Timer countdown
  useEffect(() => {
    if (!session || !hasTimer || session.status !== "in_progress" || remainingRef.current <= 0)
      return;
    const id = setInterval(() => {
      setRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session, hasTimer, remaining <= 0]);

  // Start one locked recovery cycle when the active timer expires.
  useEffect(() => {
    if (
      !session ||
      !hasTimer ||
      session.status !== "in_progress" ||
      remainingRef.current > 0
    )
      return;
    if (isSectioned) {
      if (autoAdvanceRef.current) return;
      autoAdvanceRef.current = true;
    } else {
      if (autoSubmittedRef.current) return;
      autoSubmittedRef.current = true;
    }
    void runExpiryRecovery();
  }, [remaining <= 0, isSectioned, runExpiryRecovery, session, hasTimer]);

  const clearPendingViolation = useCallback(
    (type: "fullscreen_exit" | "tab_switch") => {
      const timer = violationTimersRef.current[type];
      if (timer) {
        clearTimeout(timer);
        delete violationTimersRef.current[type];
      }
    },
    [],
  );

  const showTrackedViolation = useCallback(
    (type: "fullscreen_exit" | "tab_switch") => {
      const now = Date.now();
      const last = lastViolationAtRef.current[type];
      if (last != null && now - last < VIOLATION_DUPLICATE_SUPPRESS_MS) return;
      lastViolationAtRef.current[type] = now;
      logViolationRef.current.mutate(type);
      violationCountRef.current += 1;
      setShowViolationOverlay(true);
    },
    [],
  );

  const scheduleViolation = useCallback(
    (type: "fullscreen_exit" | "tab_switch", stillViolating: () => boolean) => {
      if (violationTimersRef.current[type]) return;
      violationTimersRef.current[type] = setTimeout(() => {
        delete violationTimersRef.current[type];
        if (stillViolating()) {
          showTrackedViolation(type);
        }
      }, VIOLATION_GRACE_MS);
    },
    [showTrackedViolation],
  );

  // Violation logging
  useEffect(() => {
    if (!sessionId || session?.status !== "in_progress") return;
    const onFullscreen = () => {
      if (!document.fullscreenElement) {
        scheduleViolation("fullscreen_exit", () => !document.fullscreenElement);
      } else {
        clearPendingViolation("fullscreen_exit");
      }
    };
    const onVisibility = () => {
      if (document.hidden) {
        scheduleViolation("tab_switch", () => document.hidden);
      } else {
        clearPendingViolation("tab_switch");
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
      clearPendingViolation("fullscreen_exit");
      clearPendingViolation("tab_switch");
    };
  }, [sessionId, session?.status, scheduleViolation, clearPendingViolation]);

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

  const setAnswer = useCallback(
    (questionId: string, value: string) => {
      setAnswers((prev) => ({ ...prev, [questionId]: value }));
      scheduleAutosave();
    },
    [scheduleAutosave],
  );

  const toggleFlag = useCallback(
    (questionId: string) => {
      setFlagged((prev) => ({ ...prev, [questionId]: !prev[questionId] }));
      scheduleAutosave();
    },
    [scheduleAutosave],
  );

  // Navigation rides the same debounced save pipeline as answers/flags so the
  // position travels with the next save payload (FR-35/FR-36).
  const goToQuestion = useCallback(
    (index: number) => {
      setCurrentQIndex(index);
      scheduleAutosave();
      for (const el of [examBodyRef.current, questionPaneRef.current]) {
        if (typeof el?.scrollTo === "function") {
          el.scrollTo({ top: 0 });
        } else if (el) {
          el.scrollTop = 0;
        }
      }
    },
    [scheduleAutosave],
  );

  const handleSubmit = useCallback(async () => {
    if (submitting) return;
    setSubmitting(true);
    submittingRef.current = true;
    const arr = buildSavePayload();
    if (arr.length > 0) {
      try {
        await saveAnswers.mutateAsync({
          answers: arr,
          current_position: currentQIndexRef.current,
        });
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
  const answeredCount = Object.values(answers).filter(
    (answer) => answer !== "",
  ).length;
  const isFlagged = currentQ ? flagged[currentQ.id] ?? false : false;
  const timerExpired = hasTimer && remaining <= 0;
  const currentTest =
    session.tests.find((t) => t.id === currentQ?.test_id) ??
    session.tests[0];
  const currentTestTitle = currentTest?.title ?? "";
  // In sectioned mode (utbk/ielts), use the mode label for the top bar title
  // to avoid duplicating the first section's title (which appears as the section label below).
  // In standard mode, show the title of the test that owns the current question
  // (not tests[0]), so a multi-test paper does not keep the first test's name.
  const examTitle = isSectioned
    ? session?.mode === "utbk"
      ? t("exam_packages_modal_mode_utbk")
      : t("exam_packages_modal_mode_ielts")
    : currentTestTitle;
  const examSubtitle = isSectioned ? activeTest?.title : currentTest?.subject;

  return (
    // Dynamic viewport height keeps the exam shell clear of mobile browser chrome.
    <div
      data-testid="exam-overlay"
      className="fixed inset-0 z-40 flex h-[100dvh] flex-col bg-background"
    >
      {/* Top bar */}
      <div
        data-testid="exam-top-bar"
        className="grid shrink-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-2 border-b border-line bg-surface px-4 py-2 sm:flex sm:flex-wrap sm:gap-x-4 lg:px-5"
      >
        <div className="flex min-w-0 items-center gap-2 sm:w-auto sm:flex-1 sm:shrink">
          <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand-600 text-white shadow-sm">
            <Trophy className="size-4" />
          </span>
          <div className="min-w-0">
            <div
              data-testid="exam-title"
              className="truncate text-sm font-bold text-ink-900"
            >
              {examTitle}
            </div>
            {examSubtitle && (
              <div className="truncate text-xs text-ink-500">
                {examSubtitle}
              </div>
            )}
          </div>
        </div>
        {!isSectioned && (
          <Button
            type="button"
            variant="destructive"
            size="sm"
            onClick={() => setShowConfirm(true)}
            disabled={timerExpired || submitting}
            className="justify-self-end rounded-full bg-[var(--color-submit)] px-4 font-bold text-white shadow-sm hover:bg-[var(--color-submit-hover)] sm:order-3"
          >
            {t("submit")}
          </Button>
        )}
        <div className="col-span-2 flex min-w-0 items-center gap-3 text-xs text-ink-500 sm:order-2 sm:col-span-1 sm:ml-auto sm:gap-4">
          <div className="whitespace-nowrap">
            {answeredCount}/{questionsToShow.length}{" "}
            {t("session_legend_answered").toLowerCase()}
          </div>
          <div
            data-testid="save-indicator"
            className="min-w-[4.75rem] whitespace-nowrap"
          >
            {saveStatus === "saved"
              ? t("session_save_saved")
              : saveStatus === "saving"
                ? t("session_save_saving")
                : t("session_save_unsaved")}
          </div>
          {hasTimer && (
            <div
              className={`ml-auto inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1 text-base font-mono font-bold lg:px-3 lg:text-lg ${
                timerExpired
                  ? "bg-danger-bg text-danger"
                  : "bg-brand-50 text-brand-700"
              }`}
            >
              <Clock className="size-4" />
              {formatTime(remaining)}
            </div>
          )}
        </div>
      </div>

      {/* Body: question pane (1fr) + nav rail (280px) */}
      <div
        ref={examBodyRef}
        data-testid="exam-body"
        className="grid flex-1 grid-cols-1 overflow-y-auto lg:grid-cols-[minmax(0,1fr)_280px] lg:overflow-hidden"
      >
        {/* Question pane */}
        <div
          ref={questionPaneRef}
          data-testid="exam-question-pane"
          className="px-4 py-4 lg:overflow-y-auto lg:px-6 lg:py-6"
        >
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
            <div className="mb-4 flex items-center justify-between gap-3">
              <div className="flex min-w-0 items-center gap-2 text-sm text-ink-600">
                <BookOpen className="size-4" />
                <span className="whitespace-nowrap">
                  {t("session_question")} {Math.min(currentQIndex + 1, questionsToShow.length)} {t("of")}{" "}
                  {questionsToShow.length}
                </span>
                {currentQ && (
                  <span className="hidden rounded-md bg-line-2 px-2 py-1 text-[11px] font-bold uppercase tracking-wide text-ink-500 sm:inline-flex">
                    {t(("fmt_" + currentQ.format) as I18nKey)}
                  </span>
                )}
              </div>
              {currentQ && (
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => toggleFlag(currentQ.id)}
                  disabled={timerExpired}
                  className={
                    isFlagged
                      ? "border-warn/60 bg-surface text-warn shadow-sm hover:bg-warn-bg"
                      : "border-warn/40 text-warn hover:bg-warn-bg"
                  }
                >
                  <Flag className="size-3.5" />
                  {isFlagged ? t("unflag") : t("flag")}
                </Button>
              )}
            </div>

            {/* Question card */}
            {currentQ && (
              <Card className="mb-4 p-5 sm:p-7">
                <div className="mb-5 text-base text-ink-900">
                  <RichContent html={currentQ.body} />
                </div>

                {renderAnswerInput(
                  currentQ,
                  answers[currentQ.id] ?? "",
                  (val) => setAnswer(currentQ.id, val),
                  timerExpired,
                )}

                {(answers[currentQ.id] ?? "") !== "" && (
                  <div className="mt-4 flex justify-end">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setAnswer(currentQ.id, "")}
                      disabled={timerExpired}
                      className="border-line text-ink-700"
                    >
                      {t("clear_answer")}
                    </Button>
                  </div>
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
                onClick={() => goToQuestion(Math.max(0, currentQIndex - 1))}
              >
                <ChevronLeft className="size-4" />
              </Button>

              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={currentQIndex >= questionsToShow.length - 1}
                onClick={() =>
                  goToQuestion(
                    Math.min(questionsToShow.length - 1, currentQIndex + 1),
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
          className="mt-5 rounded-t-2xl border-t border-line bg-surface p-4 shadow-[0_-8px_24px_rgba(21,24,58,0.06)] lg:mt-0 lg:rounded-none lg:border-t-0 lg:border-l lg:p-5 lg:shadow-none lg:overflow-y-auto"
        >
          <div className="mb-4 hidden text-xs font-bold uppercase tracking-[0.12em] text-ink-500 lg:block">
            {t("session_question")}
          </div>
          <button
            ref={navToggleRef}
            type="button"
            data-testid="exam-nav-toggle"
            aria-expanded={navExpanded}
            aria-controls="exam-nav-panel"
            onClick={() => setNavExpanded((expanded) => !expanded)}
            className="flex min-h-11 w-full items-center justify-center rounded-md border border-line bg-surface px-3 text-sm font-medium text-ink-700 lg:hidden"
          >
            {t("session_question")} {Math.min(currentQIndex + 1, questionsToShow.length)}/{questionsToShow.length}{" "}
            · {answeredCount} {t("session_legend_answered").toLowerCase()}
          </button>
          <div
            id="exam-nav-panel"
            className={`${navExpanded ? "mt-3 block rounded-xl border border-line bg-surface p-3 shadow-sm" : "hidden"} lg:mt-0 lg:block lg:rounded-none lg:border-0 lg:bg-transparent lg:p-0 lg:shadow-none`}
          >
            <div className="grid grid-cols-5 gap-2">
              {questionsToShow.map((q, i) => {
                const hasAnswer = (answers[q.id] ?? "") !== "";
                const isFlagQ = flagged[q.id] ?? false;
                const isCurrent = i === currentQIndex;

                let cellClass =
                  "relative flex size-10 items-center justify-center rounded-md border text-xs font-bold transition-colors lg:size-8";
                if (hasAnswer) {
                  cellClass += " border-brand-600 bg-brand-600 text-white";
                } else {
                  cellClass += " border-line bg-surface text-ink-700 hover:border-brand-300 hover:bg-brand-50";
                }
                if (isCurrent) {
                  cellClass += " ring-2 ring-brand-600 ring-offset-2 ring-offset-surface";
                }

                return (
                  <button
                    key={q.id}
                    type="button"
                    onClick={() => {
                      goToQuestion(i);
                      setNavExpanded(false);
                      navToggleRef.current?.focus({ preventScroll: true });
                    }}
                    className={cellClass}
                    data-testid={`session-nav-${i}`}
                  >
                    {i + 1}
                    {isFlagQ && (
                      <span className="absolute -right-1 -top-1 size-2.5 rounded-full bg-warn ring-2 ring-surface" />
                    )}
                  </button>
                );
              })}
            </div>

            {/* Legend */}
            <div className="mt-5 flex flex-col gap-2 border-t border-line pt-4">
              <LegendItem
                swatchClassName="border border-brand-600 bg-brand-600"
                label={t("session_legend_answered")}
              />
              <LegendItem
                swatchClassName="border border-line bg-surface"
                label={t("session_legend_not_answered")}
              />
              <LegendItem
                swatchClassName="scale-75 rounded-full border border-warn bg-warn"
                label={t("session_legend_flagged")}
              />
            </div>
          </div>
        </div>
      </div>

      {expiryRecoveryFailed && (
        <Card className="fixed bottom-5 left-1/2 z-50 flex -translate-x-1/2 items-center gap-3 border-danger/30 px-4 py-3">
          <span className="text-sm text-ink-700">
            {t("session_expiry_recovery_failed")}
          </span>
          <Button size="sm" onClick={runExpiryRecovery}>
            {t("retry")}
          </Button>
        </Card>
      )}

      {/* Submit confirmation dialog */}
      <Dialog open={showConfirm} onOpenChange={setShowConfirm}>
        <DialogContent className="max-w-sm rounded-2xl p-5 sm:max-w-md">
          <DialogHeader className="gap-2 text-center">
            <DialogTitle>{t("submit_confirm")}</DialogTitle>
            <DialogDescription>
              {answeredCount}/{questionsToShow.length} {t("session_question").toLowerCase()}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="flex-col gap-2 sm:flex-col sm:justify-start">
            <Button
              variant="destructive"
              onClick={handleSubmit}
              disabled={submitting}
              className="h-10 w-full rounded-full bg-[var(--color-submit)] font-bold text-white hover:bg-[var(--color-submit-hover)]"
            >
              {submitting ? t("sys_loading") : t("submit")}
            </Button>
            <DialogClose asChild>
              <Button variant="outline" className="h-10 w-full rounded-xl">
                {t("cancel")}
              </Button>
            </DialogClose>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Violation warning overlay */}
      {showViolationOverlay && (
        <div
          data-testid="violation-overlay"
          className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-danger/15 p-4 backdrop-blur-sm"
        >
          <Card
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="violation-warning-title"
            aria-describedby="violation-warning-message violation-warning-count"
            className="w-full max-w-[22rem] gap-0 rounded-[18px] border-2 border-danger bg-surface p-6 text-center shadow-xl sm:h-[19.375rem]"
          >
            <div className="flex h-full flex-col items-center">
              <div
                data-testid="violation-warning-icon"
                className="mb-4 flex size-16 items-center justify-center rounded-full bg-danger-bg text-danger"
                aria-hidden="true"
              >
                <TriangleAlert className="size-7" />
              </div>
              <h2
                id="violation-warning-title"
                className="mb-2 font-serif text-lg font-bold text-ink-900"
              >
                {t("violation_warning")}
              </h2>
              <p id="violation-warning-message" className="text-sm leading-5 text-ink-600">
                {t("violation_warning_body")}
              </p>
              <p
                id="violation-warning-count"
                data-testid="violation-warning-count"
                className="mt-2 text-xs font-semibold text-danger"
              >
                {t("violation_warning_count").replace(
                  "{n}",
                  String(violationCountRef.current),
                )}
              </p>
              <Button
                onClick={handleViolationReturn}
                autoFocus
                className="mt-5 h-11 w-full rounded-full bg-brand-600 font-bold text-white shadow-[0_8px_14px_rgba(61,77,219,0.30)] hover:bg-brand-700"
                data-testid="violation-return-button"
              >
                {t("return_to_exam")}
              </Button>
            </div>
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

  // data-rich-content: picks up the shared authored-content CSS in globals.css
  // (list bullets, table borders, empty-paragraph height) — this container
  // bypasses RichContent, so without the marker those rules never apply here.
  return <div ref={containerRef} data-rich-content className="text-base text-ink-900" />;
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
            className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
              currentValue === opt.key
                ? "border-brand-500 bg-brand-50"
                : "border-line hover:bg-surface-2"
            } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
          >
            <OptionKeyBadge optionKey={opt.key} />
            <input
              type="radio"
              name={`q-${question.id}`}
              value={opt.key}
              checked={currentValue === opt.key}
              onChange={() => onChange(opt.key)}
              disabled={disabled}
              className="size-4 accent-brand-600"
            />
            <div className="min-w-0 flex-1 break-words text-sm text-ink-800">
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
            className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
              selectedKeys.includes(opt.key)
                ? "border-brand-500 bg-brand-50"
                : "border-line hover:bg-surface-2"
            } ${disabled ? "cursor-not-allowed opacity-60" : ""}`}
          >
            <OptionKeyBadge optionKey={opt.key} />
            <input
              type="checkbox"
              checked={selectedKeys.includes(opt.key)}
              onChange={() => toggle(opt.key)}
              disabled={disabled}
              className="size-4 accent-brand-600"
            />
            <div className="min-w-0 flex-1 break-words text-sm text-ink-800">
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
