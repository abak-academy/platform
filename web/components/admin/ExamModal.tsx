"use client";

import { useEffect, useId, useMemo, useRef, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { ImageUploadInput } from "@/components/admin/ImageUploadInput";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Lock } from "lucide-react";
import { Label } from "@/components/ui/label";
import { isWhatsAppGroupUrl } from "@/lib/exam-event-info";
import { useTranslation, type I18nKey } from "@/lib/i18n";
import { toast } from "sonner";
import { ApiError } from "@/lib/api";
import { useCreateExam, useUpdateExam } from "@/lib/hooks/admin-exams";
import type { ExamListItem, CreateExamPayload, UpdateExamPayload, ExamResultConfig } from "@/lib/types";

interface ExamModalProps {
  open: boolean;
  onClose: () => void;
  exam?: ExamListItem | null;
  onSaved?: (exam: ExamListItem) => void;
  policyLocked?: boolean;
  onSessionsConflict?: () => void;
}

type TimerMode = "overall" | "per_test";
type ExamTab = "schedule" | "rules" | "participants" | "results" | "display";

const PANEL_CLASS = "grid gap-4";

const RESULT_CONFIGS = [
  ["hidden", "exam_packages_modal_result_config_hidden", "exam_packages_modal_result_config_hidden_desc"],
  ["score_only", "exam_packages_modal_result_config_score_only", "exam_packages_modal_result_config_score_only_desc"],
  ["score_pembahasan", "exam_packages_modal_result_config_score_pembahasan", "exam_packages_modal_result_config_score_pembahasan_desc"],
] as const satisfies readonly (readonly [ExamResultConfig, I18nKey, I18nKey])[];

// Mirrors maxCardNotes/maxCardNoteLen in backend/internal/service/exam.go; a
// longer note wraps and pushes the card's Perhatian block over the barcode.
const MAX_CARD_NOTES = 5;
const MAX_CARD_NOTE_LEN = 55;

function scheduledAtInputValue(iso?: string | null): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function scheduledAtIso(value: string): string | null {
  if (!value) return null;
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return null;
  return d.toISOString();
}

function nonNegativeInt(value: string): number | null {
  const n = value === "" ? NaN : Number(value);
  if (!Number.isInteger(n)) return null;
  return Math.max(0, n);
}

function isAtLeastTwo(value: string): boolean {
  const n = value === "" ? NaN : Number(value);
  return Number.isInteger(n) && n >= 2;
}

function inputValueFromNumber(n: number | null | undefined): string {
  return n != null ? String(n) : "";
}

const RADIO_CARD_BASE =
  "flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors";
const RADIO_CARD_ON = "border-brand-400 bg-brand-50 text-brand-800";
const RADIO_CARD_OFF = "border-line text-ink-700 hover:border-ink-300";

function ChoiceCard({
  name,
  value,
  checked,
  onChange,
  disabled,
  title,
  description,
}: {
  name: string;
  value: string;
  checked: boolean;
  onChange: () => void;
  disabled: boolean;
  title: string;
  description: string;
}) {
  const id = useId();
  return (
    <label
      className={`flex items-start gap-2 rounded-md border px-3 py-2 text-sm transition-colors ${
        checked ? RADIO_CARD_ON : RADIO_CARD_OFF
      }`}
    >
      <input
        type="radio"
        name={name}
        value={value}
        checked={checked}
        onChange={onChange}
        disabled={disabled}
        aria-labelledby={`${id}-title`}
        aria-describedby={`${id}-desc`}
        className="mt-1"
      />
      <span className="grid gap-0.5">
        <span id={`${id}-title`} className="font-medium">{title}</span>
        <small id={`${id}-desc`} className="text-xs font-normal text-ink-500">{description}</small>
      </span>
    </label>
  );
}

export function ExamModal({ open, onClose, exam, onSaved, policyLocked = false, onSessionsConflict }: ExamModalProps) {
  const { t } = useTranslation();
  const isEdit = Boolean(exam);
  const composable = !exam || exam.format_version === 2;
  const create = useCreateExam();
  const update = useUpdateExam(exam?.id ?? "");

  const [title, setTitle] = useState("");
  const [scheduledAt, setScheduledAt] = useState("");
  const [scheduledEndAt, setScheduledEndAt] = useState("");
  const [timerMode, setTimerMode] = useState<TimerMode>("overall");
  const [duration, setDuration] = useState("");
  const [isFree, setIsFree] = useState(false);
  const [requiresCheckin, setRequiresCheckin] = useState(false);
  const [allowLeaderboard, setAllowLeaderboard] = useState(false);
  const [randomize, setRandomize] = useState(false);
  const [navigation, setNavigation] = useState<"free" | "sequential">("free");
  const [mode, setMode] = useState("standard");
  const [resultConfig, setResultConfig] = useState<ExamResultConfig>("hidden");
  const [resultReleaseAt, setResultReleaseAt] = useState("");
  const [checkInWindow, setCheckInWindow] = useState("");
  const [graceWindow, setGraceWindow] = useState("");
  const [allowMultipleAttempts, setAllowMultipleAttempts] = useState(false);
  const [attemptsCount, setAttemptsCount] = useState("2");
  const [unlimitedAttempts, setUnlimitedAttempts] = useState(false);
  const [endScreenImageUrl, setEndScreenImageUrl] = useState("");
  const [endScreenPromoText, setEndScreenPromoText] = useState("");
  const [cardNotes, setCardNotes] = useState<string[]>([]);
  const [whatsappUrl, setWhatsappUrl] = useState("");

  const [tab, setTab] = useState<ExamTab>("schedule");
  const [touched, setTouched] = useState(false);

  const initializedFor = useRef<string | null>(null);

  useEffect(() => {
    if (open) {
      setTab("schedule");
      setTouched(false);
    }
  }, [open]);

  useEffect(() => {
    if (!open) {
      initializedFor.current = null;
      return;
    }
    const key = exam?.id ?? "new";
    if (exam && initializedFor.current === key) {
      setTimerMode((exam.timer_mode as TimerMode) ?? "overall");
      setDuration(exam.duration_minutes != null ? String(exam.duration_minutes) : "");
      setRandomize(Boolean(exam.randomize));
      setNavigation(exam.navigation_policy ?? "free");
      return;
    }
    initializedFor.current = key;
    if (exam) {
      setTitle(exam.title ?? "");
      setScheduledAt(scheduledAtInputValue(exam.scheduled_at));
      setScheduledEndAt(scheduledAtInputValue(exam.scheduled_end_at));
      setTimerMode((exam.timer_mode as TimerMode) ?? "overall");
      setDuration(exam.duration_minutes != null ? String(exam.duration_minutes) : "");
      setIsFree(Boolean(exam.is_free));
      setRequiresCheckin(Boolean(exam.requires_checkin));
      setAllowLeaderboard(Boolean(exam.allow_leaderboard));
      setRandomize(Boolean(exam.randomize));
      setMode(exam.mode ?? "standard");
      setNavigation(exam.navigation_policy ?? "free");
      setResultConfig((exam.result_config as ExamResultConfig) ?? "hidden");
      setResultReleaseAt(scheduledAtInputValue(exam.result_release_at));
      setCheckInWindow(inputValueFromNumber(exam.check_in_window_minutes));
      setGraceWindow(inputValueFromNumber(exam.grace_window_minutes));
      const attempts = exam.max_attempts;
      setAllowMultipleAttempts(attempts == null || attempts >= 2);
      setUnlimitedAttempts(attempts == null);
      setAttemptsCount(attempts != null && attempts >= 2 ? String(attempts) : "2");
      setEndScreenImageUrl(exam.end_screen_image_url ?? "");
      setEndScreenPromoText(exam.end_screen_promo_text ?? "");
      setCardNotes(exam.card_notes ?? []);
      setWhatsappUrl(exam.whatsapp_group_url ?? "");
    } else {
      setTitle("");
      setScheduledAt("");
      setScheduledEndAt("");
      setTimerMode("overall");
      setDuration("");
      setIsFree(false);
      setRequiresCheckin(false);
      setAllowLeaderboard(false);
      setRandomize(false);
      setMode("standard");
      setNavigation("free");
      setResultConfig("hidden");
      setResultReleaseAt("");
      setCheckInWindow("");
      setGraceWindow("");
      setAllowMultipleAttempts(false);
      setUnlimitedAttempts(false);
      setAttemptsCount("2");
      setEndScreenImageUrl("");
      setEndScreenPromoText("");
      setCardNotes([]);
      setWhatsappUrl("");
    }
  }, [open, exam]);

  const isPending = create.isPending || update.isPending;
  const durationRequired = timerMode === "overall";
  const endBeforeStart = useMemo(() => {
    if (!scheduledEndAt) return false;
    const startIso = scheduledAtIso(scheduledAt);
    const endIso = scheduledAtIso(scheduledEndAt);
    if (!startIso || !endIso) return false;
    return new Date(endIso) <= new Date(startIso);
  }, [scheduledAt, scheduledEndAt]);
  const durationMissing = durationRequired && !(duration !== "" && Number(duration) > 0);
  const whatsappInvalid = whatsappUrl.trim() !== "" && !isWhatsAppGroupUrl(whatsappUrl.trim());
  const canSubmit = useMemo(
    () =>
      title.trim() !== "" &&
      !durationMissing &&
      !(composable && timerMode === "per_test" && navigation !== "sequential") &&
      !endBeforeStart &&
      !whatsappInvalid &&
      !(allowMultipleAttempts && !unlimitedAttempts && !isAtLeastTwo(attemptsCount)) &&
      !isPending,
    [title, durationMissing, endBeforeStart, whatsappInvalid, isPending, composable, timerMode, navigation, allowMultipleAttempts, unlimitedAttempts, attemptsCount],
  );

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit || isPending) return;

    const base = {
      title: title.trim(),
      scheduled_at: scheduledAtIso(scheduledAt),
      scheduled_end_at: scheduledAtIso(scheduledEndAt),
      timer_mode: timerMode,
      is_free: isFree,
      requires_checkin: requiresCheckin,
      allow_leaderboard: allowLeaderboard,
      randomize,
      ...(composable ? { navigation_policy: navigation } : { mode }),
      result_config: resultConfig,
      result_release_at: scheduledAtIso(resultReleaseAt),
      check_in_window_minutes: nonNegativeInt(checkInWindow),
      grace_window_minutes: nonNegativeInt(graceWindow),
      max_attempts: !allowMultipleAttempts ? 1 : unlimitedAttempts ? null : Number(attemptsCount),
    };

    try {
      if (isEdit) {
        const payload: UpdateExamPayload = {
          ...base,
          duration_minutes:
            timerMode === "overall" && duration !== "" ? Number(duration) : null,
          end_screen_image_url: endScreenImageUrl.trim() || null,
          end_screen_promo_text: endScreenPromoText.trim() || null,
          card_notes: cardNotes.map((n) => n.trim()).filter(Boolean),
          ...(exam && "whatsapp_group_url" in exam ? { whatsapp_group_url: whatsappUrl.trim() || null } : {}),
        };
        const saved = await update.mutateAsync(payload);
        onSaved?.(saved);
      } else {
        const payload: CreateExamPayload = {
          ...base,
          duration_minutes:
            timerMode === "overall" && duration !== "" ? Number(duration) : null,
          whatsapp_group_url: whatsappUrl.trim() || null,
        };
        const result = await create.mutateAsync(payload);
        onSaved?.(result as ExamListItem);
      }
      onClose();
    } catch (err) {
      if (err instanceof ApiError && err.code === "exam_has_sessions") {
        toast.error(t("exam_admin_has_sessions_error"));
        onSessionsConflict?.();
        return;
      }
      toast.error(err instanceof ApiError ? err.message : t("error_generic"));
    }
  }

  const perTestNeedsSequential = composable && timerMode === "per_test" && navigation !== "sequential";
  const showErrors = isEdit || touched;
  const tabErrors: Record<ExamTab, boolean> = {
    schedule: showErrors && (title.trim() === "" || endBeforeStart || whatsappInvalid),
    rules: showErrors && (durationMissing || perTestNeedsSequential),
    participants: showErrors && allowMultipleAttempts && !unlimitedAttempts && !isAtLeastTwo(attemptsCount),
    results: false,
    display: false,
  };
  const tabs: { value: ExamTab; label: string }[] = [
    { value: "schedule", label: t("exam_admin_tab_schedule") },
    { value: "rules", label: t("exam_admin_tab_rules") },
    { value: "participants", label: t("exam_admin_tab_participants") },
    { value: "results", label: t("exam_admin_tab_results") },
    ...(isEdit ? [{ value: "display" as const, label: t("exam_admin_tab_display") }] : []),
  ];

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent className="gap-0 p-0 sm:max-w-[720px]">
        <form onSubmit={handleSubmit} onChange={() => setTouched(true)} className="grid min-w-0 grid-cols-1">
          <DialogHeader className="px-6 pt-6 pb-4">
            <DialogTitle className="font-serif">
              {isEdit
                ? t("exam_admin_settings_title")
                : t("exam_packages_modal_create_title")}
            </DialogTitle>
            <DialogDescription className="sr-only">{t("exam_admin_settings_subtitle")}</DialogDescription>
          </DialogHeader>

          <Tabs value={tab} onValueChange={(v) => setTab(v as ExamTab)} className="gap-0">
            <TabsList
              variant="line"
              className="h-auto w-full min-w-0 justify-start gap-0 overflow-x-auto border-b border-line px-6 py-0"
            >
              {tabs.map(({ value, label }) => (
                <TabsTrigger
                  key={value}
                  value={value}
                  className="h-10 flex-none rounded-none px-3 after:bottom-0 data-[state=active]:text-brand-800"
                >
                  {label}
                  {value === "rules" && policyLocked && <Lock aria-hidden className="size-3" />}
                  {tabErrors[value] && (
                    <>
                      <span aria-hidden className="size-1.5 rounded-full bg-danger" />
                      <span className="sr-only">{t("exam_admin_tab_has_error")}</span>
                    </>
                  )}
                </TabsTrigger>
              ))}
            </TabsList>

            <div className="h-[min(26rem,calc(100dvh-20rem))] min-h-48 overflow-y-auto px-6 py-5">
              <TabsContent value="schedule" forceMount hidden={tab !== "schedule"} className={PANEL_CLASS}>
                <div className="grid gap-2">
                  <Label htmlFor="exam-title">{t("exam_packages_modal_title")}</Label>
                  <Input
                    id="exam-title"
                    value={title}
                    onChange={(e) => setTitle(e.target.value)}
                    placeholder={t("exam_packages_modal_title_placeholder")}
                    disabled={isPending}
                  />
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="exam-scheduled-at">
                      {t("exam_packages_modal_scheduled_at")}
                    </Label>
                    <Input
                      id="exam-scheduled-at"
                      type="datetime-local"
                      value={scheduledAt}
                      onChange={(e) => setScheduledAt(e.target.value)}
                      disabled={isPending}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="exam-scheduled-end-at">
                      {t("exam_packages_modal_scheduled_end_at")}
                    </Label>
                    <Input
                      id="exam-scheduled-end-at"
                      type="datetime-local"
                      value={scheduledEndAt}
                      onChange={(e) => setScheduledEndAt(e.target.value)}
                      disabled={isPending}
                      aria-invalid={endBeforeStart}
                      className={endBeforeStart ? "border-danger focus-visible:ring-danger/30" : undefined}
                    />
                  </div>
                </div>
                <p className="text-xs text-ink-400">
                  {endBeforeStart
                    ? t("exam_packages_modal_end_before_start")
                    : t("exam_packages_modal_scheduled_end_hint")}
                </p>
                <div className="grid gap-2">
                  <Label htmlFor="exam-whatsapp-url">{t("exam_event_whatsapp_label")}</Label>
                  <Input
                    id="exam-whatsapp-url"
                    type="url"
                    value={whatsappUrl}
                    onChange={(e) => setWhatsappUrl(e.target.value)}
                    placeholder={t("exam_event_whatsapp_placeholder")}
                    disabled={isPending}
                    aria-invalid={whatsappInvalid}
                    className={whatsappInvalid ? "border-danger focus-visible:ring-danger/30" : undefined}
                  />
                  {whatsappInvalid ? (
                    <p role="alert" className="text-xs text-danger">{t("exam_event_whatsapp_invalid")}</p>
                  ) : (
                    <p className="text-xs text-ink-400">{t("exam_event_whatsapp_hint")}</p>
                  )}
                </div>
              </TabsContent>

              <TabsContent value="rules" forceMount hidden={tab !== "rules"} className={PANEL_CLASS}>
                {policyLocked && <p className="text-xs text-ink-500">{t("exam_admin_policy_locked")}</p>}
                {composable ? (
                  <div className="grid gap-2">
                    <Label id="exam-format_navigation">{t("format_navigation")}</Label>
                    <div role="radiogroup" aria-labelledby="exam-format_navigation" className="grid gap-2 sm:grid-cols-2">
                      {(["free", "sequential"] as const).map((value) => (
                        <ChoiceCard
                          key={value}
                          name="navigation_policy"
                          value={value}
                          checked={navigation === value}
                          onChange={() => setNavigation(value)}
                          disabled={isPending || policyLocked}
                          title={t(value === "free" ? "format_navigation_free" : "format_navigation_sequential")}
                          description={t(value === "free" ? "exam_admin_nav_free_desc" : "exam_admin_nav_sequential_desc")}
                        />
                      ))}
                    </div>
                  </div>
                ) : (
                  <div className="grid gap-2">
                    <Label>{t("exam_packages_modal_mode")}</Label>
                    <div className="grid gap-2 sm:grid-cols-3">
                      {(
                        [
                          ["standard", t("exam_packages_modal_mode_standard")],
                          ["utbk", t("exam_packages_modal_mode_utbk")],
                          ["ielts", t("exam_packages_modal_mode_ielts")],
                        ] as const
                      ).map(([value, label]) => (
                        <label
                          key={value}
                          className={`${RADIO_CARD_BASE} ${mode === value ? RADIO_CARD_ON : RADIO_CARD_OFF}`}
                        >
                          <input
                            type="radio"
                            name="mode"
                            value={value}
                            checked={mode === value}
                            onChange={() => setMode(value)}
                            disabled={isPending}
                          />
                          <span>{label}</span>
                        </label>
                      ))}
                    </div>
                    {mode !== "standard" && (
                      <p className="text-xs text-ink-400">{t("exam_packages_modal_mode_hint")}</p>
                    )}
                  </div>
                )}

                <div className="grid gap-2">
                  <Label id="exam-exam_packages_modal_timer_mode">{t("exam_packages_modal_timer_mode")}</Label>
                  <div role="radiogroup" aria-labelledby="exam-exam_packages_modal_timer_mode" className="grid gap-2 sm:grid-cols-2">
                    {(["overall", "per_test"] as const).map((value) => (
                      <ChoiceCard
                        key={value}
                        name="timer_mode"
                        value={value}
                        checked={timerMode === value}
                        onChange={() => { setTimerMode(value); if (composable && value === "per_test") setNavigation("sequential"); }}
                        disabled={isPending || policyLocked}
                        title={t(value === "overall" ? "exam_packages_modal_timer_overall" : "exam_packages_modal_timer_per_test")}
                        description={t(value === "overall" ? "exam_admin_timer_overall_desc" : "exam_admin_timer_per_test_desc")}
                      />
                    ))}
                  </div>
                </div>
                {perTestNeedsSequential && <p role="alert" className="text-sm text-danger">{t("format_timer_navigation_hint")}</p>}
                {timerMode === "overall" && (
                  <div className="grid gap-2">
                    <Label htmlFor="exam-duration-minutes">
                      {t("exam_packages_modal_duration_minutes")}
                    </Label>
                    <Input
                      id="exam-duration-minutes"
                      type="number"
                      min={1}
                      value={duration}
                      onChange={(e) => setDuration(e.target.value)}
                      placeholder="60"
                      disabled={isPending || policyLocked}
                    />
                  </div>
                )}
                <div className="grid gap-1.5">
                  <label
                    className={`flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors ${
                      randomize ? RADIO_CARD_ON : RADIO_CARD_OFF
                    }`}
                  >
                    <input
                      type="checkbox"
                      checked={randomize}
                      onChange={(e) => setRandomize(e.target.checked)}
                      disabled={isPending || policyLocked}
                    />
                    <span>{t("exam_packages_modal_randomize")}</span>
                  </label>
                  <p className="text-xs text-ink-400">{t("exam_admin_randomize_help")}</p>
                </div>
              </TabsContent>

              <TabsContent value="participants" forceMount hidden={tab !== "participants"} className={PANEL_CLASS}>
                <div className="grid gap-2 sm:grid-cols-2">
                  {(
                    [
                      [isFree, setIsFree, t("exam_packages_modal_is_free")],
                      [requiresCheckin, setRequiresCheckin, t("exam_packages_modal_requires_checkin")],
                      [allowLeaderboard, setAllowLeaderboard, t("exam_packages_modal_allow_leaderboard")],
                    ] as const
                  ).map(([checked, setChecked, label]) => (
                    <label
                      key={label}
                      className={`flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors ${
                        checked ? RADIO_CARD_ON : RADIO_CARD_OFF
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={(e) => setChecked(e.target.checked)}
                        disabled={isPending}
                      />
                      <span>{label}</span>
                    </label>
                  ))}
                </div>

                <div className="grid gap-4 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="exam-check-in-window">{t("exam_packages_modal_check_in_window")}</Label>
                    <Input
                      id="exam-check-in-window"
                      type="number"
                      min={0}
                      value={checkInWindow}
                      onChange={(e) => setCheckInWindow(e.target.value)}
                      disabled={isPending}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="exam-grace-window">{t("exam_packages_modal_grace_window")}</Label>
                    <Input
                      id="exam-grace-window"
                      type="number"
                      min={0}
                      value={graceWindow}
                      onChange={(e) => setGraceWindow(e.target.value)}
                      disabled={isPending}
                    />
                  </div>
                </div>

                <div className="grid gap-2">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={allowMultipleAttempts}
                      onChange={(e) => setAllowMultipleAttempts(e.target.checked)}
                      disabled={isPending}
                    />
                    <span>{t("exam_admin_attempts_multiple")}</span>
                  </label>
                  {!allowMultipleAttempts && (
                    <p className="text-xs text-ink-500">{t("exam_admin_attempts_single_hint")}</p>
                  )}
                  {allowMultipleAttempts && (
                    <div className="grid gap-2 sm:grid-cols-2">
                      {!unlimitedAttempts && (
                        <div className="grid gap-2">
                          <Label htmlFor="exam-max-attempts">{t("exam_admin_attempts_count")}</Label>
                          <Input
                            id="exam-max-attempts"
                            type="number"
                            min={2}
                            step={1}
                            value={attemptsCount}
                            onChange={(e) => setAttemptsCount(e.target.value)}
                            disabled={isPending}
                          />
                          {!isAtLeastTwo(attemptsCount) && (
                            <p role="alert" className="text-xs text-danger">{t("exam_admin_attempts_required")}</p>
                          )}
                        </div>
                      )}
                      <label className="flex items-center gap-2 self-end pb-2 text-sm">
                        <input
                          type="checkbox"
                          checked={unlimitedAttempts}
                          onChange={(e) => setUnlimitedAttempts(e.target.checked)}
                          disabled={isPending}
                        />
                        <span>{t("exam_admin_attempts_unlimited")}</span>
                      </label>
                    </div>
                  )}
                </div>
              </TabsContent>

              <TabsContent value="results" forceMount hidden={tab !== "results"} className={PANEL_CLASS}>
                <div className="grid gap-2">
                  <Label id="exam-exam_packages_modal_result_config">{t("exam_packages_modal_result_config")}</Label>
                  <div role="radiogroup" aria-labelledby="exam-exam_packages_modal_result_config" className="grid gap-2">
                    {RESULT_CONFIGS.map(([value, titleKey, descKey]) => (
                      <ChoiceCard
                        key={value}
                        name="result_config"
                        value={value}
                        checked={resultConfig === value}
                        onChange={() => setResultConfig(value)}
                        disabled={isPending}
                        title={t(titleKey)}
                        description={t(descKey)}
                      />
                    ))}
                  </div>
                </div>
                <div className="grid gap-2 sm:max-w-xs">
                  <Label htmlFor="exam-result-release-at">{t("exam_packages_modal_result_release_at")}</Label>
                  <Input
                    id="exam-result-release-at"
                    type="datetime-local"
                    value={resultReleaseAt}
                    onChange={(e) => setResultReleaseAt(e.target.value)}
                    disabled={isPending}
                  />
                  {resultConfig !== "hidden" && <p className="text-xs text-ink-400">{t("exam_packages_modal_result_release_hint")}</p>}
                </div>
              </TabsContent>

              {isEdit && (
                <TabsContent value="display" forceMount hidden={tab !== "display"} className={PANEL_CLASS}>
                  <div className="grid gap-3">
                    <h4 className="text-sm font-semibold text-ink-900">{t("exam_packages_modal_section_card")}</h4>
                    <p className="text-xs text-ink-400">
                      {t("exam_packages_modal_card_notes_hint")}
                    </p>
                    {cardNotes.length === 0 ? (
                      <p className="text-xs text-ink-500">
                        {t("exam_packages_modal_card_notes_empty")}
                      </p>
                    ) : null}
                    {cardNotes.map((note, i) => (
                      <div key={i} className="flex items-center gap-2">
                        <Input
                          aria-label={t("exam_packages_modal_card_note_label") + " " + (i + 1)}
                          value={note}
                          maxLength={MAX_CARD_NOTE_LEN}
                          onChange={(e) =>
                            setCardNotes((prev) =>
                              prev.map((v, j) => (j === i ? e.target.value : v)),
                            )
                          }
                          disabled={isPending}
                        />
                        <Button
                          type="button"
                          variant="outline"
                          className="rounded-full px-3"
                          aria-label={t("exam_packages_modal_card_note_remove")}
                          onClick={() => setCardNotes((prev) => prev.filter((_, j) => j !== i))}
                          disabled={isPending}
                        >
                          ×
                        </Button>
                      </div>
                    ))}
                    <div>
                      <Button
                        type="button"
                        variant="outline"
                        className="rounded-full"
                        onClick={() => setCardNotes((prev) => [...prev, ""])}
                        disabled={isPending || cardNotes.length >= MAX_CARD_NOTES}
                      >
                        {t("exam_packages_modal_card_note_add")}
                      </Button>
                    </div>
                  </div>

                  <div className="grid gap-3 border-t border-line pt-5">
                    <h4 className="text-sm font-semibold text-ink-900">{t("exam_packages_modal_section_end_screen")}</h4>
                    <p className="text-xs text-ink-400">
                      {t("exam_packages_modal_end_screen_hint")}
                    </p>
                    <div className="grid gap-2">
                      <Label htmlFor="exam-end-screen-image">
                        {t("exam_packages_modal_end_screen_image")}
                      </Label>
                      <ImageUploadInput
                        id="exam-end-screen-image"
                        aria-label={t("exam_packages_modal_end_screen_image")}
                        value={endScreenImageUrl}
                        onChange={setEndScreenImageUrl}
                        disabled={isPending}
                      />
                    </div>
                    <div className="grid gap-2">
                      <Label htmlFor="exam-end-screen-promo">
                        {t("exam_packages_modal_end_screen_promo")}
                      </Label>
                      <textarea
                        id="exam-end-screen-promo"
                        data-slot="textarea"
                        value={endScreenPromoText}
                        onChange={(e) => setEndScreenPromoText(e.target.value)}
                        rows={3}
                        disabled={isPending}
                        className="flex w-full rounded-md border border-input bg-transparent px-3 py-2 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                      />
                    </div>
                  </div>
                </TabsContent>
              )}
            </div>
          </Tabs>

          <DialogFooter className="flex-row justify-end gap-2 border-t border-line px-6 py-4">
            <Button
              type="button"
              variant="outline"
              className="rounded-full"
              onClick={onClose}
              disabled={isPending}
            >
              {t("exam_packages_modal_cancel")}
            </Button>
            <Button type="submit" className="rounded-full" disabled={!canSubmit || isPending}>
              {isPending ? t("saving") : isEdit ? t("exam_admin_settings_save") : t("exam_packages_modal_save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
