"use client";

import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { Check, Plus, Trash2, X } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { RichTextEditor, type RichTextEditorHandle } from "@/components/admin/RichTextEditor";
import { AudioUploadInput } from "@/components/admin/AudioUploadInput";
import { useSaveQuestion } from "@/lib/hooks/admin-tests";
import {
  useCreateBankQuestion,
  useUpdateBankQuestion,
} from "@/lib/hooks/admin-bank-questions";
import { useTopics } from "@/lib/hooks/admin-topics";
import { useTranslation } from "@/lib/i18n";
import type {
  AdminQuestionInput,
  AdminQuestionOptionInput,
  QuestionFormat,
  QuestionWithOptions,
} from "@/lib/types";

interface QuestionEditorProps {
  testId?: string;
  question?: QuestionWithOptions;
  onCancel: () => void;
  onSaved?: () => void;
}

const DEFAULT_KEYS: Array<"a" | "b" | "c" | "d"> = ["a", "b", "c", "d"];

function nextKey(existing: AdminQuestionOptionInput[]): string {
  const used = new Set(existing.map((o) => o.key.toLowerCase()));
  for (const k of DEFAULT_KEYS) {
    if (!used.has(k)) return k;
  }
  return "x";
}

interface BlankEditorProps {
  blanks: Array<{ index: number; correct_answer: string }>;
  onChange: (next: Array<{ index: number; correct_answer: string }>) => void;
  onInsertToken: (index: number) => void;
  onRemoveToken: (index: number) => void;
  disabled: boolean;
}

function BlankEditor({ blanks, onChange, onInsertToken, onRemoveToken, disabled }: BlankEditorProps) {
  const { t } = useTranslation();

  function update(index: number, patch: { correct_answer?: string }) {
    onChange(blanks.map((b, i) => (i === index ? { ...b, ...patch } : b)));
  }

  function remove(index: number) {
    if (blanks.length <= 1) return;
    const removedTokenIndex = blanks[index].index;
    onChange(
      blanks
        .filter((_, i) => i !== index)
        .map((b) => (b.index > removedTokenIndex ? { ...b, index: b.index - 1 } : b))
    );
    onRemoveToken(removedTokenIndex);
  }

  function add() {
    const nextIndex = blanks.length + 1;
    onInsertToken(nextIndex);
    onChange([
      ...blanks,
      { index: nextIndex, correct_answer: "" },
    ]);
  }

  return (
    <div className="space-y-3">
      <div className="grid gap-2 sm:grid-cols-2">
        {blanks.map((blank, index) => (
          <div key={index} className="flex items-center gap-2">
            <div className="w-8 text-sm font-mono text-muted-foreground">
              {`{{${blank.index}}}`}
            </div>
            <Input
              aria-label={t("tests_field_correct_answer")}
              value={blank.correct_answer}
              onChange={(e) => update(index, { correct_answer: e.target.value })}
              placeholder={t("tests_field_correct_answer")}
              disabled={disabled}
              className="flex-1"
            />
            <Button
              type="button"
              size="icon-xs"
              variant="ghost"
              onClick={() => remove(index)}
              disabled={disabled || blanks.length <= 1}
              aria-label={t("tests_remove_option")}
            >
              <Trash2 className="size-3" />
            </Button>
          </div>
        ))}
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={add}
        disabled={disabled}
      >
        <Plus className="mr-1 size-4" />
        {t("tests_add_option")}
      </Button>
    </div>
  );
}

function OptionEditor({
  format,
  options,
  onChange,
  disabled,
}: {
  format: "mcq" | "multi_answer";
  options: AdminQuestionOptionInput[];
  onChange: (next: AdminQuestionOptionInput[]) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();
  const isSingle = format === "mcq";

  function update(index: number, patch: Partial<AdminQuestionOptionInput>) {
    onChange(options.map((o, i) => (i === index ? { ...o, ...patch } : o)));
  }

  function setCorrect(index: number, value: boolean) {
    if (isSingle) {
      onChange(options.map((o, i) => ({ ...o, is_correct: i === index })));
      return;
    }
    onChange(options.map((o, i) => (i === index ? { ...o, is_correct: value } : o)));
  }

  function remove(index: number) {
    if (options.length <= 2) return;
    onChange(options.filter((_, i) => i !== index));
  }

  function add() {
    const key = nextKey(options);
    onChange([
      ...options,
      {
        key,
        text: "",
        is_correct: false,
        sort_order: options.length + 1,
      },
    ]);
  }

  return (
    <div className="space-y-3">
      <div className="grid gap-3 sm:grid-cols-2">
        {options.map((opt, index) => (
          <div key={index} className="space-y-2 rounded-lg border p-2">
            <div className="flex items-center gap-2">
              <div className="text-sm font-mono uppercase text-muted-foreground">
                {opt.key}
              </div>
              <label className="ml-auto flex items-center gap-1 text-sm">
                {isSingle ? (
                  <input
                    type="radio"
                    name={`question-correct-${format}`}
                    checked={opt.is_correct}
                    onChange={() => setCorrect(index, true)}
                    disabled={disabled}
                    aria-label={t("tests_field_option_is_correct")}
                  />
                ) : (
                  <input
                    type="checkbox"
                    checked={opt.is_correct}
                    onChange={(e) => setCorrect(index, e.target.checked)}
                    disabled={disabled}
                    aria-label={t("tests_field_option_is_correct")}
                  />
                )}
                <span>{t("tests_field_option_is_correct")}</span>
              </label>
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                onClick={() => remove(index)}
                disabled={disabled || options.length <= 2}
                aria-label={t("tests_remove_option")}
              >
                <Trash2 className="size-3" />
              </Button>
            </div>
            <RichTextEditor
              id={`option-text-${index}`}
              aria-label={`${t("tests_field_option_text")} ${opt.key}`}
              value={opt.text}
              onChange={(html) => update(index, { text: html })}
              placeholder={t("tests_field_option_text")}
              disabled={disabled}
              minHeightClassName="min-h-[60px]"
              compact
            />
          </div>
        ))}
      </div>
      <Button
        type="button"
        size="sm"
        variant="outline"
        onClick={add}
        disabled={disabled || options.length >= 4}
      >
        <Plus className="mr-1 size-4" />
        {t("tests_add_option")}
      </Button>
    </div>
  );
}

const FORMAT_LABELS: Record<QuestionFormat, "tests_format_mcq" | "tests_format_multi_answer" | "tests_format_short" | "tests_format_fill_blank" | "tests_format_essay" | "tests_format_multi_blank"> = {
  mcq: "tests_format_mcq",
  multi_answer: "tests_format_multi_answer",
  short: "tests_format_short",
  fill_blank: "tests_format_fill_blank",
  essay: "tests_format_essay",
  multi_blank: "tests_format_multi_blank",
};

const ALL_FORMATS: QuestionFormat[] = ["mcq", "multi_answer", "short", "fill_blank", "essay", "multi_blank"];


function buildOptionsFromQuestion(q: QuestionWithOptions): AdminQuestionOptionInput[] {
  // options is null (not []) for optionless formats coming from the bank API.
  if (!q.options || q.options.length === 0) {
    return [
      { key: "a", text: "", is_correct: true, sort_order: 1 },
      { key: "b", text: "", is_correct: false, sort_order: 2 },
    ];
  }
  return q.options.map((o) => ({
    key: o.key,
    text: o.text,
    image_url: o.image_url ?? undefined,
    is_correct: o.is_correct,
    sort_order: o.sort_order,
  }));
}

function buildInput(
  format: QuestionFormat,
  body: string,
  difficulty: string,
  explanation: string,
  imageUrl: string,
  audioUrl: string,
  correctAnswer: string,
  options: AdminQuestionOptionInput[],
  blanks: Array<{ index: number; correct_answer: string }>,
  pointCorrect: string,
  pointWrong: string,
  topicId: string
): AdminQuestionInput {
  const base: AdminQuestionInput = {
    format,
    body: body.trim(),
    point_correct: Number(pointCorrect) || 1,
    point_wrong: Number(pointWrong) || 0,
  };
  if (topicId) base.topic_id = topicId;
  if (difficulty) base.difficulty = difficulty;
  if (explanation.trim()) base.explanation = explanation.trim();
  if (imageUrl.trim()) base.image_url = imageUrl.trim();
  if (audioUrl.trim()) base.audio_url = audioUrl.trim();
  if (format === "short" || format === "fill_blank") {
    base.correct_answer = correctAnswer.trim();
  }
  if (format === "mcq" || format === "multi_answer") {
    base.options = options.map((o, i) => ({
      key: o.key,
      text: o.text,
      image_url: o.image_url,
      is_correct: o.is_correct,
      sort_order: i + 1,
    }));
  }
  if (format === "multi_blank") {
    base.blanks = blanks.map((b) => ({
      index: b.index,
      correct_answer: b.correct_answer.trim(),
    }));
  }
  return base;
}

// Mirrors backend/internal/service/exam.go's extractTokensFromStem: finds
// every {{N}} token in the body, in document order.
function extractBlankTokens(body: string): number[] {
  return Array.from(body.matchAll(/\{\{(\d+)\}\}/g), (m) => Number(m[1]));
}

// Mirrors validateTokenSequence (exam.go:157-181) plus the blanks/tokens
// count check from validateQuestion's multi_blank case (exam.go:275-277), so
// the admin sees the same rule the server enforces before they submit.
function validateBlankTokens(
  body: string,
  blanks: Array<{ index: number; correct_answer: string }>
): { ok: true } | { ok: false; key: string } {
  const tokens = extractBlankTokens(body);
  if (tokens.length === 0) {
    return { ok: false, key: "tests_validation_blank_token_required" };
  }
  const seen = new Set<number>();
  for (const token of tokens) {
    if (seen.has(token)) {
      return { ok: false, key: "tests_validation_blank_token_duplicate" };
    }
    seen.add(token);
  }
  for (let i = 1; i <= tokens.length; i++) {
    if (!seen.has(i)) {
      return { ok: false, key: "tests_validation_blank_token_gap" };
    }
  }
  if (blanks.length !== tokens.length) {
    return { ok: false, key: "tests_validation_blank_row_mismatch" };
  }
  return { ok: true };
}

// Removes the token numbered `removedIndex` from the body and shifts every
// token above it down by one, so the remaining tokens stay a contiguous
// 1..N set — pairs with BlankEditor's remove(), which does the same
// renumbering to the blank rows.
function renumberTokensInBody(body: string, removedIndex: number): string {
  return body.replace(/\{\{(\d+)\}\}/g, (match, digits: string) => {
    const n = Number(digits);
    if (n === removedIndex) return "";
    if (n > removedIndex) return `{{${n - 1}}}`;
    return match;
  });
}

function validate(
  format: QuestionFormat,
  body: string,
  correctAnswer: string,
  options: AdminQuestionOptionInput[],
  blanks: Array<{ index: number; correct_answer: string }>,
  topicId: string
): { ok: true } | { ok: false; key: string } {
  if (!topicId) {
    return { ok: false, key: "tests_validation_topic_required" };
  }
  if (!body.trim()) {
    return { ok: false, key: "tests_validation_body_required" };
  }
  if (format === "mcq") {
    const correct = options.filter((o) => o.is_correct).length;
    if (correct !== 1) return { ok: false, key: "tests_validation_mcq_one_correct" };
  }
  if (format === "multi_answer") {
    const correct = options.filter((o) => o.is_correct).length;
    if (correct < 1) return { ok: false, key: "tests_validation_multi_answer_one_correct" };
  }
  if (format === "short" || format === "fill_blank") {
    if (!correctAnswer.trim()) {
      return { ok: false, key: "tests_validation_correct_answer_required" };
    }
  }
  if (format === "multi_blank") {
    const tokenResult = validateBlankTokens(body, blanks);
    if (!tokenResult.ok) return tokenResult;
    for (const blank of blanks) {
      if (!blank.correct_answer.trim()) {
        return { ok: false, key: "tests_validation_correct_answer_required" };
      }
    }
  }
  return { ok: true };
}

export function QuestionEditor({ testId, question, onCancel, onSaved }: QuestionEditorProps) {
  const { t } = useTranslation();
  const isEdit = Boolean(question);
  const isTestScoped = Boolean(testId);
  const [format, setFormat] = useState<QuestionFormat>(question?.question.format ?? "mcq");
  const [body, setBody] = useState(question?.question.body ?? "");
  const [difficulty, setDifficulty] = useState<string>(question?.question.difficulty ?? "");
  const [explanation, setExplanation] = useState(question?.question.explanation ?? "");
  const [imageUrl, setImageUrl] = useState(question?.question.image_url ?? "");
  const [audioUrl, setAudioUrl] = useState(question?.question.audio_url ?? "");
  const [correctAnswer, setCorrectAnswer] = useState(question?.question.correct_answer ?? "");
  const [pointCorrect, setPointCorrect] = useState(String(question?.question.point_correct ?? 1));
  const [pointWrong, setPointWrong] = useState(String(question?.question.point_wrong ?? 0));
  const [topicId, setTopicId] = useState(question?.question.topic_id ?? "");
  const [options, setOptions] = useState<AdminQuestionOptionInput[]>(
    question ? buildOptionsFromQuestion(question) : [
      { key: "a", text: "", is_correct: true, sort_order: 1 },
      { key: "b", text: "", is_correct: false, sort_order: 2 },
    ]
  );
  const [blanks, setBlanks] = useState<Array<{ index: number; correct_answer: string }>>(
    question?.blanks ?? [
      { index: 1, correct_answer: "" },
      { index: 2, correct_answer: "" },
    ]
  );
  const [errorKey, setErrorKey] = useState<string | null>(null);
  const bodyEditorRef = useRef<RichTextEditorHandle>(null);

  const topics = useTopics();
  const createBankQuestion = useCreateBankQuestion();
  const updateBankQuestion = useUpdateBankQuestion(question?.question.id ?? "");
  const testSave = useSaveQuestion(testId ?? "");

  useEffect(() => {
    if (!question) {
      setFormat("mcq");
      setBody("");
      setDifficulty("");
      setExplanation("");
      setImageUrl("");
      setAudioUrl("");
      setCorrectAnswer("");
      setPointCorrect("1");
      setPointWrong("0");
      setTopicId("");
      setOptions([
        { key: "a", text: "", is_correct: true, sort_order: 1 },
        { key: "b", text: "", is_correct: false, sort_order: 2 },
      ]);
      setBlanks([
        { index: 1, correct_answer: "" },
        { index: 2, correct_answer: "" },
      ]);
    }
  }, [question]);

  function handleDifficultyChange(value: string) {
    setDifficulty(value === "none" ? "" : value);
  }

  function insertBlankToken(index: number) {
    bodyEditorRef.current?.insertTextAtCaret(`{{${index}}}`);
  }

  function removeBlankToken(removedIndex: number) {
    bodyEditorRef.current?.setContent(renumberTokensInBody(body, removedIndex));
  }

  async function handleSave() {
    const result = validate(format, body, correctAnswer, options, blanks, topicId);
    if (!result.ok) {
      setErrorKey(result.key);
      return;
    }
    setErrorKey(null);
    const input = buildInput(
      format,
      body,
      difficulty,
      explanation,
      imageUrl,
      audioUrl,
      correctAnswer,
      options,
      blanks,
      pointCorrect,
      pointWrong,
      topicId
    );
    try {
      if (isTestScoped) {
        await testSave.mutateAsync({ question: question?.question.id, input });
      } else if (isEdit) {
        await updateBankQuestion.mutateAsync(input);
      } else {
        await createBankQuestion.mutateAsync(input);
      }
      toast.success(t("tests_save_success"));
      onSaved?.();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : t("error_generic"));
    }
  }

  const showOptions = format === "mcq" || format === "multi_answer";
  const showCorrectAnswer = format === "short" || format === "fill_blank";
  const showBlanks = format === "multi_blank";
  const errorMessage = errorKey ? t(errorKey as Parameters<typeof t>[0]) : null;
  const savePending = testSave.isPending || createBankQuestion.isPending || updateBankQuestion.isPending;

  const topicOptions = topics.data?.data ?? [];

  return (
    <Dialog open onOpenChange={(o) => { if (!o) onCancel(); }}>
      <DialogContent className="flex flex-col gap-0 p-0 sm:max-w-6xl max-h-[90vh] overflow-hidden">
        <DialogHeader className="shrink-0 border-b px-6 py-5">
          <DialogTitle>{isEdit ? t("tests_edit_question") : t("tests_new_question")}</DialogTitle>
          <DialogDescription>{t("tests_editor_hint")}</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-0 flex-1 flex-col sm:flex-row">
          <div className="flex-1 space-y-5 overflow-y-auto p-6 sm:w-3/4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor="question-format">{t("format")}</Label>
                <select
                  id="question-format"
                  data-slot="select"
                  value={format}
                  onChange={(e) => setFormat(e.target.value as QuestionFormat)}
                  disabled={savePending}
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                >
                  {ALL_FORMATS.map((f) => (
                    <option key={f} value={f}>
                      {t(FORMAT_LABELS[f])}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="question-topic">{t("topic")}</Label>
                <select
                  id="question-topic"
                  data-slot="select"
                  value={topicId}
                  onChange={(e) => setTopicId(e.target.value)}
                  disabled={savePending || topics.isLoading}
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                >
                  <option value="">{t("select_topic")}</option>
                  {topicOptions.map((topic) => (
                    <option key={topic.id} value={topic.id}>
                      {topic.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="question-body">{t("tests_field_body")}</Label>
              <RichTextEditor
                ref={bodyEditorRef}
                id="question-body"
                aria-label={t("tests_field_body")}
                value={body}
                onChange={setBody}
                placeholder={t("tests_field_body")}
                disabled={savePending}
                minHeightClassName="min-h-[220px]"
              />
            </div>

            {(showOptions || showCorrectAnswer || showBlanks) && (
              <>
                <Separator />

                {showOptions && (
                  <div className="grid gap-2">
                    <Label>{t("tests_field_option_text")}</Label>
                    <OptionEditor
                      format={format as "mcq" | "multi_answer"}
                      options={options}
                      onChange={setOptions}
                      disabled={savePending}
                    />
                  </div>
                )}

                {showCorrectAnswer && (
                  <div className="grid gap-2">
                    <Label htmlFor="question-correct-answer">{t("tests_field_correct_answer")}</Label>
                    <Input
                      id="question-correct-answer"
                      value={correctAnswer}
                      onChange={(e) => setCorrectAnswer(e.target.value)}
                      disabled={savePending}
                    />
                  </div>
                )}

                {showBlanks && (
                  <div className="grid gap-2">
                    <Label>{t("tests_field_correct_answer")}</Label>
                    <BlankEditor
                      blanks={blanks}
                      onChange={setBlanks}
                      onInsertToken={insertBlankToken}
                      onRemoveToken={removeBlankToken}
                      disabled={savePending}
                    />
                  </div>
                )}
              </>
            )}
          </div>

          <div className="shrink-0 space-y-5 overflow-y-auto border-t bg-muted/40 p-6 sm:w-1/4 sm:border-t-0 sm:border-l">
            <div className="grid gap-2">
              <Label htmlFor="question-difficulty">{t("difficulty")}</Label>
              <select
                id="question-difficulty"
                value={difficulty || "none"}
                onChange={(e) => handleDifficultyChange(e.target.value)}
                disabled={savePending}
                className="flex h-9 w-full rounded-md border border-input bg-background px-3 py-1 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
              >
                <option value="none">—</option>
                <option value="easy">{t("tests_field_difficulty_easy")}</option>
                <option value="medium">{t("tests_field_difficulty_medium")}</option>
                <option value="hard">{t("tests_field_difficulty_hard")}</option>
              </select>
            </div>

            <div className="space-y-3">
              <Label className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                {t("tests_points_panel_title")}
              </Label>
              <div className="grid grid-cols-2 gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="question-point-correct" className="flex items-center gap-1">
                    <Check className="size-3.5 text-success" />
                    {t("tests_field_point_correct")}
                  </Label>
                  <Input
                    id="question-point-correct"
                    type="number"
                    min={1}
                    step={1}
                    value={pointCorrect}
                    onChange={(e) => setPointCorrect(e.target.value)}
                    disabled={savePending}
                    className="bg-background"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="question-point-wrong" className="flex items-center gap-1">
                    <X className="size-3.5 text-destructive" />
                    {t("tests_field_point_wrong")}
                  </Label>
                  <Input
                    id="question-point-wrong"
                    type="number"
                    min={0}
                    step={1}
                    value={pointWrong}
                    onChange={(e) => setPointWrong(e.target.value)}
                    disabled={savePending}
                    className="bg-background"
                  />
                </div>
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="question-audio-url">{t("tests_field_audio_url")}</Label>
              <AudioUploadInput
                id="question-audio-url"
                value={audioUrl}
                onChange={setAudioUrl}
                placeholder="https://..."
                disabled={savePending}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="question-explanation">{t("tests_field_explanation")}</Label>
              <textarea
                id="question-explanation"
                value={explanation}
                onChange={(e) => setExplanation(e.target.value)}
                rows={4}
                disabled={savePending}
                className="flex w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
              />
            </div>
          </div>
        </div>

        <div className="flex shrink-0 items-center justify-between gap-2 border-t px-6 py-4">
          {errorMessage ? (
            <p role="alert" className="text-sm text-destructive">
              {errorMessage}
            </p>
          ) : <span />}
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={onCancel}
              disabled={savePending}
            >
              {t("cancel")}
            </Button>
            <Button type="button" onClick={handleSave} disabled={savePending}>
              {savePending ? t("saving") : t("tests_save_question")}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
