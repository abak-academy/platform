"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { toast } from "sonner";
import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  ClipboardList,
  Eye,
  Library,
  List,
  Plus,
  Unlink,
} from "lucide-react";
import { AdminPageHeader } from "@/components/admin/AdminPageHeader";
import { Button } from "@/components/ui/button";
import { QuestionEditor } from "@/components/admin/QuestionEditor";
import { QuestionPickerModal } from "@/components/admin/QuestionPickerModal";
import { AudioUploadInput } from "@/components/admin/AudioUploadInput";
import { RichContent } from "@/components/admin/RichContent";
import { RichTextEditor } from "@/components/admin/RichTextEditor";
import { TestPreview } from "@/components/admin/TestPreview";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
import { useUnsavedChangesGuard } from "@/lib/hooks/use-unsaved-changes-guard";
import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { materialHTMLOrNull, stripHtmlToPlainText } from "@/lib/rich-text";
import { FORMAT_TONE, DIFFICULTY_TONE } from "@/lib/question-tone";
import type { AdminUpdateTestInput, QuestionWithOptions, QuestionFormat, Test } from "@/lib/types";

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

interface TestDraft {
  title: string;
  subject: string;
  topic: string;
  duration: string;
  audioUrl: string;
  audioPlayLimit: string;
  sectionType: string;
  material: string;
  instructions: string;
}

function toDraft(test: Test): TestDraft {
  return {
    title: test.title ?? "",
    subject: test.subject ?? "",
    topic: test.topic ?? "",
    duration: test.duration_minutes != null ? String(test.duration_minutes) : "",
    audioUrl: test.audio_url ?? "",
    audioPlayLimit: test.audio_play_limit != null ? String(test.audio_play_limit) : "",
    sectionType: test.section_type ?? "",
    material: test.shared_material_html ?? "",
    instructions: test.instructions_html ?? "",
  };
}

function isDirty(draft: TestDraft, baseline: TestDraft): boolean {
  const normalize = (d: TestDraft) =>
    JSON.stringify({
      ...d,
      material: materialHTMLOrNull(d.material) ?? "",
      instructions: materialHTMLOrNull(d.instructions) ?? "",
    });
  return normalize(draft) !== normalize(baseline);
}

function QuestionRow({
  question,
  index,
  total,
  onReorder,
  onPreview,
  onDetach,
}: {
  question: QuestionWithOptions;
  index: number;
  total: number;
  onReorder: (questionId: string, direction: "up" | "down") => void;
  onPreview: (index: number) => void;
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
        <p className="line-clamp-2 text-sm text-ink-900 [overflow-wrap:anywhere]">{stripHtmlToPlainText(q.body)}</p>
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
      <div className="flex items-center gap-1">
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          onClick={() => onPreview(index)}
          aria-label={t("tests_preview_question")}
          title={t("tests_preview_question")}
        >
          <Eye className="size-3.5" />
        </Button>
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          onClick={() => onDetach(q.id)}
          aria-label={t("tests_detach_question")}
          title={t("tests_detach_question")}
          className="text-danger hover:text-danger"
        >
          <Unlink className="size-3.5" />
        </Button>
      </div>
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
  const [detachTarget, setDetachTarget] = useState<string | null>(null);
  const [previewIndex, setPreviewIndex] = useState<number | null>(null);
  const [materialOpen, setMaterialOpen] = useState(false);

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

  const [draft, setDraft] = useState<TestDraft | null>(null);
  const [baseline, setBaseline] = useState<TestDraft | null>(null);
  const hydratedId = useRef<string | null>(null);

  // Hydrate once per test: a refetch after attach/reorder must not overwrite the unsaved draft.
  useEffect(() => {
    if (!detail || hydratedId.current === detail.test.id) return;
    hydratedId.current = detail.test.id;
    const initial = toDraft(detail.test);
    setDraft(initial);
    setBaseline(initial);
  }, [detail]);

  const isLoading = detailLoading || questionsLoading;
  const isError = detailError || questionsError;
  const error = detailErr ?? questionsErr;
  const questions = questionsResp?.data ?? detail?.questions ?? [];

  const questionIds = useMemo(() => questions.map((q) => q.question.id), [questions]);
  const topicNames = useMemo(
    () => Array.from(new Set(topics.map((tp) => tp.name))).sort((a, b) => a.localeCompare(b)),
    [topics],
  );

  const dirty = draft !== null && baseline !== null && isDirty(draft, baseline);
  useUnsavedChangesGuard(dirty, t("tests_discard_changes_confirm"));

  const listeningRequiresAudio = draft?.sectionType === "listening" && draft.audioUrl.trim() === "";
  const canSaveTest =
    draft !== null &&
    dirty &&
    draft.title.trim() !== "" &&
    draft.subject.trim() !== "" &&
    draft.topic.trim() !== "" &&
    draft.duration !== "" &&
    Number(draft.duration) > 0 &&
    !listeningRequiresAudio &&
    !update.isPending;

  function patchDraft(patch: Partial<TestDraft>) {
    setDraft((prev) => (prev ? { ...prev, ...patch } : prev));
  }

  async function handleSaveTest() {
    if (!canSaveTest || !draft || !id) return;
    const submitted = draft;
    // Explicit null (not an omitted key) clears a field server-side; an absent key preserves it.
    const payload: AdminUpdateTestInput = {
      title: submitted.title.trim(),
      subject: submitted.subject.trim(),
      topic: submitted.topic.trim(),
      duration_minutes: Number(submitted.duration),
      audio_url: submitted.audioUrl.trim() || null,
      audio_play_limit: submitted.audioPlayLimit !== "" ? Number(submitted.audioPlayLimit) : null,
      section_type: submitted.sectionType || null,
      shared_material_html: materialHTMLOrNull(submitted.material),
      instructions_html: materialHTMLOrNull(submitted.instructions),
    };
    try {
      await update.mutateAsync(payload);
      setBaseline(submitted);
      toast.success(t("tests_update_success"));
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    }
  }

  function handleBack() {
    if (dirty && !window.confirm(t("tests_discard_changes_confirm"))) return;
    router.push("/admin/exam/tests");
  }

  function openPreview(index: number) {
    if (dirty) {
      toast.error(t("tests_preview_save_first"));
      return;
    }
    setPreviewIndex(index);
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

  async function handleDetach() {
    if (!detachTarget) return;
    try {
      await detach.mutateAsync(detachTarget);
      toast.success(t("tests_save_success"));
    } catch (e) {
      toast.error(errorMessage(e, t("error_generic")));
    } finally {
      setDetachTarget(null);
    }
  }

  async function handleAttach(selectedIds: string[]) {
    await attach.mutateAsync({ question_ids: selectedIds });
  }

  const savedMaterial = detail?.test.shared_material_html;
  const hasSavedMaterial = Boolean(savedMaterial?.trim());
  const ready = !isLoading && !isError && draft !== null;

  return (
    <div className="space-y-6 fade-in">
      <div>
        <button
          type="button"
          data-testid="tests-back-link"
          onClick={handleBack}
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
          ready ? (
            <>
              <Button type="button" variant="outline" size="lg" onClick={() => openPreview(0)}>
                <Eye className="mr-1 size-4" />
                {t("tests_preview_test")}
              </Button>
              <Button
                type="button"
                size="lg"
                onClick={handleSaveTest}
                disabled={!canSaveTest}
                className="shadow-md shadow-primary/30"
              >
                {update.isPending ? t("saving") : t("tests_save_settings")}
              </Button>
            </>
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

      {ready && draft && (
        <div className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
          <Tabs defaultValue="questions" className="min-w-0">
            <TabsList className="h-auto w-full flex-wrap justify-start sm:w-fit">
              <TabsTrigger value="questions" className="text-sm">
                {t("tests_tab_questions")}
              </TabsTrigger>
              <TabsTrigger value="material" className="text-sm">
                {t("tests_tab_material")}
              </TabsTrigger>
              <TabsTrigger value="info" className="text-sm">
                {t("tests_tab_info")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="questions" className="space-y-4 pt-2">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h3 className="text-[15px] font-semibold text-ink-900">
                  {t("tests_questions_in_this_test")}{" "}
                  <span className="font-normal text-ink-500">· {questions.length}</span>
                </h3>
                <div className="flex items-center gap-2">
                  <Button type="button" variant="outline" size="sm" onClick={() => setPickerOpen(true)}>
                    <Library className="mr-1 size-4" />
                    {t("tests_from_bank")}
                  </Button>
                  <Button type="button" size="sm" onClick={() => setCreating(true)} disabled={creating}>
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
                      <Button type="button" variant="outline" size="sm" onClick={() => setPickerOpen(true)}>
                        <Library className="mr-1 size-4" />
                        {t("tests_from_bank")}
                      </Button>
                      <Button type="button" size="sm" onClick={() => setCreating(true)} disabled={creating}>
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
                      onPreview={openPreview}
                      onDetach={setDetachTarget}
                    />
                  ))
                )}
              </div>
            </TabsContent>

            <TabsContent value="material" className="space-y-6 pt-2">
              <div className="grid gap-2">
                <Label htmlFor="test-shared-material">{t("format_shared_material")}</Label>
                <RichTextEditor
                  id="test-shared-material"
                  aria-label={t("format_shared_material")}
                  value={draft.material}
                  onChange={(html) => patchDraft({ material: html })}
                  disabled={update.isPending}
                />
                <p className="text-xs text-ink-500">{t("format_reusable_material_hint")}</p>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="test-instructions">{t("format_instructions")}</Label>
                <RichTextEditor
                  id="test-instructions"
                  aria-label={t("format_instructions")}
                  value={draft.instructions}
                  onChange={(html) => patchDraft({ instructions: html })}
                  disabled={update.isPending}
                />
              </div>
            </TabsContent>

            <TabsContent value="info" className="pt-2">
              <div className="space-y-6 rounded-2xl bg-white p-6 shadow-sm ring-1 ring-[var(--md-sys-color-outline-variant)]">
                <div className="grid gap-2">
                  <Label htmlFor="test-title">{t("tests_field_title")}</Label>
                  <Input
                    id="test-title"
                    value={draft.title}
                    onChange={(e) => patchDraft({ title: e.target.value })}
                    placeholder={t("tests_field_title")}
                    disabled={update.isPending}
                  />
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="test-subject">{t("tests_field_subject")}</Label>
                    <Input
                      id="test-subject"
                      value={draft.subject}
                      onChange={(e) => patchDraft({ subject: e.target.value })}
                      placeholder={t("tests_field_subject")}
                      disabled={update.isPending}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="test-topic">{t("tests_field_topic")}</Label>
                    <select
                      id="test-topic"
                      value={draft.topic}
                      onChange={(e) => patchDraft({ topic: e.target.value })}
                      disabled={update.isPending}
                      className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                    >
                      <option value="" disabled>
                        {t("tests_field_topic")}
                      </option>
                      {draft.topic && !topicNames.includes(draft.topic) && (
                        <option value={draft.topic}>{draft.topic}</option>
                      )}
                      {topicNames.map((name) => (
                        <option key={name} value={name}>
                          {name}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div className="grid gap-2">
                    <Label htmlFor="test-duration">{t("tests_field_duration")}</Label>
                    <Input
                      id="test-duration"
                      type="number"
                      min={1}
                      value={draft.duration}
                      onChange={(e) => patchDraft({ duration: e.target.value })}
                      placeholder="60"
                      disabled={update.isPending}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="test-audio-play-limit">{t("tests_field_audio_play_limit")}</Label>
                    <select
                      id="test-audio-play-limit"
                      value={draft.audioPlayLimit}
                      onChange={(e) => patchDraft({ audioPlayLimit: e.target.value })}
                      disabled={update.isPending}
                      className="h-9 w-full rounded-md border border-input bg-transparent px-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:opacity-50"
                    >
                      <option value="">{t("unlimited")}</option>
                      {!["", "1", "2", "3"].includes(draft.audioPlayLimit) && (
                        <option value={draft.audioPlayLimit}>{draft.audioPlayLimit}×</option>
                      )}
                      <option value="1">1×</option>
                      <option value="2">2×</option>
                      <option value="3">3×</option>
                    </select>
                  </div>
                </div>

                <div className="grid gap-2">
                  <Label>{t("tests_field_section_type")}</Label>
                  <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                    {SECTION_TYPES.map(({ value, labelKey }) => (
                      <label
                        key={value || "none"}
                        className="flex items-center gap-2 rounded-md border border-input px-3 py-2 text-sm"
                      >
                        <input
                          type="radio"
                          name="section_type"
                          value={value}
                          checked={draft.sectionType === value}
                          onChange={() => patchDraft({ sectionType: value })}
                          disabled={update.isPending}
                        />
                        <span>{t(labelKey as Parameters<typeof t>[0])}</span>
                      </label>
                    ))}
                  </div>
                  {draft.sectionType === "listening" && (
                    <p className="text-xs text-ink-500">
                      URL audio wajib diisi untuk sesi Listening.
                    </p>
                  )}
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="test-audio-url">{t("tests_field_audio_url")}</Label>
                  <AudioUploadInput
                    id="test-audio-url"
                    value={draft.audioUrl}
                    onChange={(url) => patchDraft({ audioUrl: url })}
                    placeholder="https://..."
                    disabled={update.isPending}
                  />
                </div>
              </div>
            </TabsContent>
          </Tabs>

          <aside
            data-testid="test-material-sidebar"
            className="min-w-0 space-y-3 rounded-2xl bg-white p-4 shadow-sm ring-1 ring-[var(--md-sys-color-outline-variant)] lg:sticky lg:top-6"
          >
            <div className="flex items-center justify-between gap-2">
              <h3 className="text-[15px] font-semibold text-ink-900">{t("tests_material_sidebar_title")}</h3>
              {hasSavedMaterial && (
                <Button type="button" size="sm" variant="ghost" onClick={() => setMaterialOpen(true)}>
                  {t("tests_material_view_full")}
                </Button>
              )}
            </div>
            {hasSavedMaterial ? (
              <RichContent
                html={savedMaterial ?? ""}
                className="max-h-72 overflow-y-auto text-sm [overflow-wrap:anywhere] [&_img]:h-auto [&_img]:max-w-full"
              />
            ) : (
              <p className="text-sm text-ink-500">{t("tests_material_empty")}</p>
            )}
          </aside>
        </div>
      )}

      <QuestionPickerModal
        open={pickerOpen}
        onOpenChange={setPickerOpen}
        testId={id}
        attached={questions}
        onAttach={handleAttach}
      />

      <Dialog open={detachTarget !== null} onOpenChange={(open) => !open && setDetachTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("tests_detach_confirm_title")}</DialogTitle>
            <DialogDescription>{t("tests_detach_confirm_body")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setDetachTarget(null)} disabled={detach.isPending}>
              {t("cancel")}
            </Button>
            <Button type="button" variant="destructive" onClick={handleDetach} disabled={detach.isPending}>
              {t("tests_detach_confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={materialOpen} onOpenChange={setMaterialOpen}>
        <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-3xl">
          <DialogHeader>
            <DialogTitle>{t("tests_material_sidebar_title")}</DialogTitle>
            <DialogDescription>{detail?.test.title}</DialogDescription>
          </DialogHeader>
          <RichContent
            html={savedMaterial ?? ""}
            className="text-sm [overflow-wrap:anywhere] [&_img]:h-auto [&_img]:max-w-full"
          />
        </DialogContent>
      </Dialog>

      {detail && (
        <TestPreview
          open={previewIndex !== null}
          onOpenChange={(open) => !open && setPreviewIndex(null)}
          test={detail.test}
          questions={questions}
          initialIndex={previewIndex ?? 0}
        />
      )}
    </div>
  );
}
