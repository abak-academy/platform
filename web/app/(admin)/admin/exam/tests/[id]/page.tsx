"use client";

import { useEffect, useMemo, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  ClipboardList,
  Library,
  List,
  Plus,
  Trash2,
} from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Button } from "@/components/ui/button";
import { QuestionEditor } from "@/components/admin/QuestionEditor";
import { QuestionPickerModal } from "@/components/admin/QuestionPickerModal";
import { QuestionBundleControls } from "@/components/admin/QuestionBundleControls";
import { AudioUploadInput } from "@/components/admin/AudioUploadInput";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Badge } from "@/components/ui/badge";
import {
  useTestDetail,
  useTestQuestions,
  useUpdateTest,
  useDetachQuestion,
  useReorderTestQuestions,
  useAttachQuestions,
} from "@/lib/hooks/admin-tests";
import { useTopics } from "@/lib/hooks/admin-topics";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { stripHtmlToPlainText } from "@/lib/rich-text";
import { FORMAT_TONE, DIFFICULTY_TONE } from "@/lib/question-tone";
import type { QuestionWithOptions, QuestionFormat } from "@/lib/types";

const FORMAT_BADGE: Record<
  QuestionFormat,
  | "tests_format_mcq"
  | "tests_format_multi_answer"
  | "tests_format_short"
  | "tests_format_fill_blank"
  | "tests_format_essay"
  | "tests_format_multi_blank"
  | "tests_format_true_false"
> = {
  mcq: "tests_format_mcq",
  multi_answer: "tests_format_multi_answer",
  short: "tests_format_short",
  fill_blank: "tests_format_fill_blank",
  essay: "tests_format_essay",
  multi_blank: "tests_format_multi_blank",
  true_false: "tests_format_true_false",
};

const DIFFICULTY_LABELS: Record<string, "diff_easy" | "diff_medium" | "diff_hard"> = {
  easy: "diff_easy",
  medium: "diff_medium",
  hard: "diff_hard",
};

const SECTION_TYPES: Array<{ value: string; labelKey: string }> = [
  { value: "", labelKey: "tests_field_section_type_none" },
  { value: "listening", labelKey: "tests_field_section_type_listening" },
  { value: "reading", labelKey: "tests_field_section_type_reading" },
  { value: "writing", labelKey: "tests_field_section_type_writing" },
];

function errorMessage(err: unknown, fallback: string): string {
  if (err instanceof Error) return err.message;
  return fallback;
}

function useSyncedTestForm(test: { test: { title: string; subject: string; topic: string; duration_minutes: number; audio_url?: string | null; audio_play_limit?: number | null; section_type?: string | null } } | undefined) {
  const [title, setTitle] = useState("");
  const [subject, setSubject] = useState("");
  const [topic, setTopic] = useState("");
  const [duration, setDuration] = useState("");
  const [audioUrl, setAudioUrl] = useState("");
  const [audioPlayLimit, setAudioPlayLimit] = useState("");
  const [sectionType, setSectionType] = useState<string>("");

  useEffect(() => {
    if (!test) return;
    const t = test.test;
    setTitle(t.title ?? "");
    setSubject(t.subject ?? "");
    setTopic(t.topic ?? "");
    setDuration(t.duration_minutes != null ? String(t.duration_minutes) : "");
    setAudioUrl(t.audio_url ?? "");
    setAudioPlayLimit(t.audio_play_limit != null ? String(t.audio_play_limit) : "");
    setSectionType(t.section_type ?? "");
  }, [test]);

  return {
    title,
    setTitle,
    subject,
    setSubject,
    topic,
    setTopic,
    duration,
    setDuration,
    audioUrl,
    setAudioUrl,
    audioPlayLimit,
    setAudioPlayLimit,
    sectionType,
    setSectionType,
  };
}

function QuestionRow({
  question,
  index,
  total,
  onReorder,
  onDetach,
}: {
  question: QuestionWithOptions;
  index: number;
  total: number;
  onReorder: (questionId: string, direction: "up" | "down") => void;
  onDetach: (questionId: string) => void;
}) {
  const { t } = useTranslation();
  const isFirst = index === 0;
  const isLast = index === total - 1;
  const q = question.question;
  const difficultyKey = q.difficulty ? DIFFICULTY_LABELS[q.difficulty] : undefined;

  return (
    <div
      data-question-row
      className={`flex items-start gap-3 p-3 ${index < total - 1 ? "border-b" : ""}`}
    >
      <div className="flex flex-col items-center gap-0.5 pt-0.5 text-ink-500">
        <button
          type="button"
          onClick={() => onReorder(q.id, "up")}
          disabled={isFirst}
          aria-label={t("tests_reorder_up")}
          className="hover:text-ink-900 disabled:opacity-30"
        >
          <ChevronUp className="size-4" />
        </button>
        <span className="text-sm font-semibold text-ink-900">{index + 1}</span>
        <button
          type="button"
          onClick={() => onReorder(q.id, "down")}
          disabled={isLast}
          aria-label={t("tests_reorder_down")}
          className="hover:text-ink-900 disabled:opacity-30"
        >
          <ChevronDown className="size-4" />
        </button>
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm text-ink-900">{stripHtmlToPlainText(q.body)}</p>
        <div className="mt-2 flex flex-wrap items-center gap-1.5">
          <Badge variant="outline" className={cn("border-transparent", FORMAT_TONE[q.format])}>
            {t(FORMAT_BADGE[q.format])}
          </Badge>
          {q.topic && (
            <Badge variant="outline" className="border-transparent bg-brand-50 text-brand-700">
              {q.topic}
            </Badge>
          )}
          {difficultyKey && (
            <Badge
              variant="outline"
              className={cn("border-transparent", q.difficulty && DIFFICULTY_TONE[q.difficulty])}
            >
              {t(difficultyKey)}
            </Badge>
          )}
        </div>
      </div>
      <Button
        type="button"
        size="icon-xs"
        variant="ghost"
        onClick={() => onDetach(q.id)}
        aria-label={t("tests_detach_question")}
        className="text-danger hover:text-danger"
      >
        <Trash2 className="size-3.5" />
      </Button>
    </div>
  );
}

export default function TestDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params?.id ?? "";
  const { t } = useTranslation();
  const [creating, setCreating] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);

  const {
    data: detail,
    isLoading: detailLoading,
    isError: detailError,
    error: detailErr,
  } = useTestDetail(id);
  const {
    data: questionsResp,
    isLoading: questionsLoading,
    isError: questionsError,
    error: questionsErr,
  } = useTestQuestions(id);
  const { data: topicsResp } = useTopics();
  const topics = topicsResp?.data ?? [];

  const update = useUpdateTest(id);
  const detach = useDetachQuestion(id);
  const reorder = useReorderTestQuestions(id);
  const attach = useAttachQuestions(id);

  const form = useSyncedTestForm(detail);

  const isLoading = detailLoading || questionsLoading;
  const isError = detailError || questionsError;
  const error = detailErr ?? questionsErr;
  const questions = questionsResp?.data ?? detail?.questions ?? [];

  const questionIds = useMemo(() => questions.map((q) => q.question.id), [questions]);
  const topicNames = useMemo(
    () => Array.from(new Set(topics.map((tp) => tp.name))).sort((a, b) => a.localeCompare(b)),
    [topics],
  );

  const listeningRequiresAudio = form.sectionType === "listening" && form.audioUrl.trim() === "";
  const canSaveTest =
    form.title.trim() !== "" &&
    form.subject.trim() !== "" &&
    form.topic.trim() !== "" &&
    form.duration !== "" &&
    Number(form.duration) > 0 &&
    !listeningRequiresAudio &&
    !update.isPending;

  async function handleSaveTest(e: React.FormEvent) {
    e.preventDefault();
    if (!canSaveTest || !id) return;
    const payload: Record<string, unknown> = {
      title: form.title.trim(),
      subject: form.subject.trim(),
      topic: form.topic.trim(),
      duration_minutes: Number(form.duration),
    };
    // Explicit null (not an omitted key) when cleared, so the backend can tell
    // "clear this field" apart from "field not sent" — the form is synced from
    // the existing test on load, so an empty value here means the user cleared it.
    payload.audio_url = form.audioUrl.trim() || null;
    payload.audio_play_limit = form.audioPlayLimit !== "" ? Number(form.audioPlayLimit) : null;
    payload.section_type = form.sectionType || null;
    try {
      await update.mutateAsync(payload);
      toast.success(t("tests_update_success"));
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    }
  }

  async function handleReorder(questionId: string, direction: "up" | "down") {
    const idx = questionIds.indexOf(questionId);
    if (idx < 0) return;
    const targetIdx = direction === "up" ? idx - 1 : idx + 1;
    if (targetIdx < 0 || targetIdx >= questionIds.length) return;
    const next = [...questionIds];
    [next[idx], next[targetIdx]] = [next[targetIdx], next[idx]];
    try {
      await reorder.mutateAsync({ question_ids: next });
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    }
  }

  async function handleDetach(questionId: string) {
    try {
      await detach.mutateAsync(questionId);
      toast.success(t("tests_save_success"));
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    }
  }

  async function handleAttach(selectedIds: string[]) {
    await attach.mutateAsync({ question_ids: selectedIds });
  }

  return (
    <div className="space-y-6 fade-in">
      <div>
        <button
          type="button"
          data-testid="tests-back-link"
          onClick={() => router.push("/admin/exam/tests")}
          aria-label={t("tests_page_title")}
          className="inline-flex items-center gap-1 text-sm text-ink-500 hover:text-ink-900"
        >
          <ArrowLeft className="size-4" />
          {t("tests_page_title")}
        </button>
      </div>

      <AdminPageHeader
        icon={ClipboardList}
        title={detail ? detail.test.title : t("sys_loading")}
        description={
          detail
            ? `${detail.test.subject} · ${detail.test.topic} · ${detail.test.duration_minutes} min`
            : undefined
        }
        actionsAlign="end"
        actions={
          !isLoading && !isError ? (
            <Button
              type="submit"
              form="test-detail-form"
              size="lg"
              disabled={!canSaveTest}
              className="shadow-md shadow-primary/30"
            >
              {update.isPending ? t("saving") : t("save")}
            </Button>
          ) : undefined
        }
      />

      {isLoading && (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      )}

      {isError && (
        <div className="rounded-lg border border-danger/20 bg-danger/10 p-4 text-danger">
          {errorMessage(error, t("error_generic"))}
        </div>
      )}

      {!isLoading && !isError && (
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[360px_1fr]">
          {/* Left: sticky test details form */}
          <form
            id="test-detail-form"
            onSubmit={handleSaveTest}
            className="space-y-6 rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[var(--md-sys-color-outline-variant)] lg:sticky lg:top-6 lg:self-start"
          >
            <div className="grid gap-2">
              <Label htmlFor="test-title">{t("tests_field_title")}</Label>
              <Input
                id="test-title"
                value={form.title}
                onChange={(e) => form.setTitle(e.target.value)}
                placeholder={t("tests_field_title")}
                disabled={update.isPending}
              />
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="test-subject">{t("tests_field_subject")}</Label>
                <Input
                  id="test-subject"
                  value={form.subject}
                  onChange={(e) => form.setSubject(e.target.value)}
                  placeholder={t("tests_field_subject")}
                  disabled={update.isPending}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="test-topic">{t("tests_field_topic")}</Label>
                <select
                  id="test-topic"
                  value={form.topic}
                  onChange={(e) => form.setTopic(e.target.value)}
                  disabled={update.isPending}
                  className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                >
                  <option value="" disabled>
                    {t("tests_field_topic")}
                  </option>
                  {form.topic && !topicNames.includes(form.topic) && (
                    <option value={form.topic}>{form.topic}</option>
                  )}
                  {topicNames.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4">
              <div className="grid gap-2">
                <Label htmlFor="test-duration">{t("tests_field_duration")}</Label>
                <Input
                  id="test-duration"
                  type="number"
                  min={1}
                  value={form.duration}
                  onChange={(e) => form.setDuration(e.target.value)}
                  placeholder="60"
                  disabled={update.isPending}
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="test-audio-play-limit">{t("tests_field_audio_play_limit")}</Label>
                <select
                  id="test-audio-play-limit"
                  value={form.audioPlayLimit}
                  onChange={(e) => form.setAudioPlayLimit(e.target.value)}
                  disabled={update.isPending}
                  className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                >
                  <option value="">{t("unlimited")}</option>
                  {!["", "1", "2", "3"].includes(form.audioPlayLimit) && (
                    <option value={form.audioPlayLimit}>{form.audioPlayLimit}×</option>
                  )}
                  <option value="1">1×</option>
                  <option value="2">2×</option>
                  <option value="3">3×</option>
                </select>
              </div>
            </div>

            <div className="grid gap-2">
              <Label>{t("tests_field_section_type")}</Label>
              <div className="grid grid-cols-2 gap-2">
                {SECTION_TYPES.map(({ value, labelKey }) => (
                  <label
                    key={value || "none"}
                    className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm"
                  >
                    <input
                      type="radio"
                      name="section_type"
                      value={value}
                      checked={form.sectionType === value}
                      onChange={() => form.setSectionType(value)}
                      disabled={update.isPending}
                    />
                    <span>{t(labelKey as Parameters<typeof t>[0])}</span>
                  </label>
                ))}
              </div>
              {form.sectionType === "listening" && (
                <p className="text-xs text-ink-500">
                  URL audio wajib diisi untuk sesi Listening.
                </p>
              )}
            </div>

            <div className="grid gap-2">
              <Label htmlFor="test-audio-url">{t("tests_field_audio_url")}</Label>
              <AudioUploadInput
                id="test-audio-url"
                value={form.audioUrl}
                onChange={form.setAudioUrl}
                placeholder="https://..."
                disabled={update.isPending}
              />
            </div>
          </form>

          {/* Right: questions panel */}
          <div className="space-y-4">
            <QuestionBundleControls scope="test" scopeId={id} disabled={questions.length === 0} />

            <div className="flex items-center justify-between gap-2">
              <h3 className="text-[15px] font-semibold text-ink-900">
                {t("tests_questions_in_this_test")}{" "}
                <span className="font-normal text-ink-500">· {questions.length}</span>
              </h3>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setPickerOpen(true)}
                >
                  <Library className="mr-1 size-4" />
                  {t("tests_from_bank")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  onClick={() => setCreating(true)}
                  disabled={creating}
                >
                  <Plus className="mr-1 size-4" />
                  {t("tests_new_question")}
                </Button>
              </div>
            </div>

            {creating && (
              <QuestionEditor
                testId={id}
                onCancel={() => setCreating(false)}
                onSaved={() => setCreating(false)}
              />
            )}

            <div className="rounded-2xl bg-white shadow-sm ring-1 ring-[var(--md-sys-color-outline-variant)]">
              {questions.length === 0 ? (
                <div className="px-4 py-10 text-center text-ink-500">
                  <List className="mx-auto size-8" strokeWidth={1.4} />
                  <p className="mt-3 text-sm">{t("tests_questions_empty")}</p>
                  <div className="mt-5 flex items-center justify-center gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => setPickerOpen(true)}
                    >
                      <Library className="mr-1 size-4" />
                      {t("tests_from_bank")}
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => setCreating(true)}
                      disabled={creating}
                    >
                      <Plus className="mr-1 size-4" />
                      {t("tests_new_question")}
                    </Button>
                  </div>
                </div>
              ) : (
                questions.map((q, idx) => (
                  <QuestionRow
                    key={q.question.id}
                    question={q}
                    index={idx}
                    total={questions.length}
                    onReorder={handleReorder}
                    onDetach={handleDetach}
                  />
                ))
              )}
            </div>
          </div>
        </div>
      )}

      <QuestionPickerModal
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        testId={id}
        attached={questions}
        onAttach={handleAttach}
      />
    </div>
  );
}
