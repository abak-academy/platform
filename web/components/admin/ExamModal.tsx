"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useTranslation } from "@/lib/i18n";
import { toast } from "sonner";
import { useCreateExam, useUpdateExam } from "@/lib/hooks/admin-exams";
import type { ExamListItem, CreateExamPayload, UpdateExamPayload, ExamResultConfig } from "@/lib/types";

interface ExamModalProps {
  open: boolean;
  onClose: () => void;
  exam?: ExamListItem | null;
  onSaved?: (exam: ExamListItem) => void;
}

type TimerMode = "overall" | "per_test";

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

function inputValueFromNumber(n: number | null | undefined): string {
  return n != null ? String(n) : "";
}

const RADIO_CARD_BASE =
  "flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors";
const RADIO_CARD_ON = "border-brand-400 bg-brand-50 text-brand-800";
const RADIO_CARD_OFF = "border-line text-ink-700 hover:border-ink-300";

function SectionHeading({ children }: { children: React.ReactNode }) {
  return (
    <h3 className="border-b border-line pb-1.5 text-xs font-semibold tracking-wide text-ink-500 uppercase">
      {children}
    </h3>
  );
}

export function ExamModal({ open, onClose, exam, onSaved }: ExamModalProps) {
  const { t } = useTranslation();
  const isEdit = Boolean(exam);
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
  const [mode, setMode] = useState("standard");
  const [resultConfig, setResultConfig] = useState<ExamResultConfig>("hidden");
  const [resultReleaseAt, setResultReleaseAt] = useState("");
  const [checkInWindow, setCheckInWindow] = useState("");
  const [graceWindow, setGraceWindow] = useState("");
  const [maxAttempts, setMaxAttempts] = useState("");

  useEffect(() => {
    if (!open) return;
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
      setResultConfig((exam.result_config as ExamResultConfig) ?? "hidden");
      setResultReleaseAt(scheduledAtInputValue(exam.result_release_at));
      setCheckInWindow(inputValueFromNumber(exam.check_in_window_minutes));
      setGraceWindow(inputValueFromNumber(exam.grace_window_minutes));
      setMaxAttempts(inputValueFromNumber(exam.max_attempts));
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
      setResultConfig("hidden");
      setResultReleaseAt("");
      setCheckInWindow("");
      setGraceWindow("");
      setMaxAttempts("");
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
  const canSubmit = useMemo(
    () =>
      title.trim() !== "" &&
      (!durationRequired || (duration !== "" && Number(duration) > 0)) &&
      !endBeforeStart &&
      !isPending,
    [title, duration, durationRequired, endBeforeStart, isPending],
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
      mode,
      result_config: resultConfig,
      result_release_at: scheduledAtIso(resultReleaseAt),
      check_in_window_minutes: nonNegativeInt(checkInWindow),
      grace_window_minutes: nonNegativeInt(graceWindow),
      max_attempts: nonNegativeInt(maxAttempts),
    };

    try {
      if (isEdit) {
        const payload: UpdateExamPayload = {
          ...base,
          duration_minutes:
            timerMode === "overall" && duration !== "" ? Number(duration) : null,
        };
        const saved = await update.mutateAsync(payload);
        onSaved?.(saved);
      } else {
        const payload: CreateExamPayload = {
          ...base,
          duration_minutes:
            timerMode === "overall" && duration !== "" ? Number(duration) : null,
        };
        const result = await create.mutateAsync(payload);
        onSaved?.(result as ExamListItem);
      }
      onClose();
    } catch {
      toast.error(t("error_generic"));
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <DialogContent className="sm:max-w-2xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle className="font-serif">
              {isEdit
                ? t("exam_packages_modal_edit_title")
                : t("exam_packages_modal_create_title")}
            </DialogTitle>
          </DialogHeader>

          <div className="grid max-h-[70vh] gap-6 overflow-y-auto py-4 pr-1">
            <div className="grid gap-3">
              <SectionHeading>{t("exam_packages_modal_section_details")}</SectionHeading>
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

              <div className="grid grid-cols-2 gap-4">
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
            </div>

            <div className="grid gap-3">
              <SectionHeading>{t("exam_packages_modal_section_format")}</SectionHeading>
              <div className="grid gap-2">
                <Label>{t("exam_packages_modal_mode")}</Label>
                <div className="grid grid-cols-3 gap-2">
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

              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label>{t("exam_packages_modal_timer_mode")}</Label>
                  <div className="grid grid-cols-2 gap-2">
                    {(
                      [
                        ["overall", t("exam_packages_modal_timer_overall")],
                        ["per_test", t("exam_packages_modal_timer_per_test")],
                      ] as const
                    ).map(([value, label]) => (
                      <label
                        key={value}
                        className={`${RADIO_CARD_BASE} ${timerMode === value ? RADIO_CARD_ON : RADIO_CARD_OFF}`}
                      >
                        <input
                          type="radio"
                          name="timer_mode"
                          value={value}
                          checked={timerMode === value}
                          onChange={() => setTimerMode(value)}
                          disabled={isPending}
                        />
                        <span>{label}</span>
                      </label>
                    ))}
                  </div>
                </div>
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
                      disabled={isPending}
                    />
                  </div>
                )}
              </div>
            </div>

            <div className="grid gap-3">
              <SectionHeading>{t("exam_packages_modal_section_access")}</SectionHeading>
              <div className="grid grid-cols-2 gap-2">
                {(
                  [
                    [isFree, setIsFree, t("exam_packages_modal_is_free")],
                    [requiresCheckin, setRequiresCheckin, t("exam_packages_modal_requires_checkin")],
                    [allowLeaderboard, setAllowLeaderboard, t("exam_packages_modal_allow_leaderboard")],
                    [randomize, setRandomize, t("exam_packages_modal_randomize")],
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
            </div>

            <div className="grid gap-3">
              <SectionHeading>{t("exam_packages_modal_section_results")}</SectionHeading>
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="exam-result-config">{t("exam_packages_modal_result_config")}</Label>
                  <select
                    id="exam-result-config"
                    value={resultConfig}
                    onChange={(e) => setResultConfig(e.target.value as ExamResultConfig)}
                    disabled={isPending}
                    className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                  >
                    <option value="hidden">{t("exam_packages_modal_result_config_hidden")}</option>
                    <option value="score_only">{t("exam_packages_modal_result_config_score_only")}</option>
                    <option value="score_pembahasan">{t("exam_packages_modal_result_config_score_pembahasan")}</option>
                  </select>
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="exam-result-release-at">{t("exam_packages_modal_result_release_at")}</Label>
                  <Input
                    id="exam-result-release-at"
                    type="datetime-local"
                    value={resultReleaseAt}
                    onChange={(e) => setResultReleaseAt(e.target.value)}
                    disabled={isPending}
                  />
                </div>
              </div>

              <div className="grid grid-cols-3 gap-4">
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
                <div className="grid gap-2">
                  <Label htmlFor="exam-max-attempts">{t("exam_packages_modal_max_attempts")}</Label>
                  <Input
                    id="exam-max-attempts"
                    type="number"
                    min={0}
                    value={maxAttempts}
                    onChange={(e) => setMaxAttempts(e.target.value)}
                    disabled={isPending}
                  />
                </div>
              </div>
            </div>
          </div>

          <DialogFooter>
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
              {isPending ? t("saving") : t("exam_packages_modal_save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
