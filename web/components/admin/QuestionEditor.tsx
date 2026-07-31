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
import { ApiError } from "@/lib/api";
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
  BankQuestionListItem,
  Question,
  QuestionFormat,
  QuestionWithOptions,
} from "@/lib/types";

interface QuestionEditorProps {
  testId?: string;
  question?: BankQuestionListItem;
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

interface BlankRow {
  index: number;
  accepted_answers: string[];
}

function AcceptedAnswerEditor({
  answers,
  onChange,
  disabled,
}: {
  answers: string[];
  onChange: (next: string[]) => void;
  disabled: boolean;
}) {
  const { t } = useTranslation();

  function update(index: number, value: string) {
    onChange(answers.map((a, i) => (i === index ? value : a)));
  }

  function remove(index: number) {
    if (answers.length <= 1) return;
    onChange(answers.filter((_, i) => i !== index));
  }

  function add() {
    onChange([...answers, ""]);
  }

  return (
    <div className="space-y-2">
      {answers.map((answer, index) => (
        <div key={index} className="flex items-center gap-2">
          <Input
            aria-label={t("tests_field_accepted_answers")}
            value={answer}
            onChange={(e) => update(index, e.target.value)}
            placeholder={t("tests_field_accepted_answers")}
            disabled={disabled}
            className="flex-1"
          />
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            onClick={() => remove(index)}
            disabled={disabled || answers.length <= 1}
            aria-label={t("tests_remove_accepted_answer")}
          >
            <Trash2 className="size-3" />
          </Button>
        </div>
      ))}
      <Button type="button" size="sm" variant="outline" onClick={add} disabled={disabled}>
        <Plus className="mr-1 size-4" />
        {t("tests_add_accepted_answer")}
      </Button>
    </div>
  );
}

interface BlankEditorProps {
  blanks: BlankRow[];
  onChange: (next: BlankRow[]) => void;
  onInsertToken: (index: number) => void;
  onRemoveToken: (index: number) => void;
  disabled: boolean;
}

function BlankEditor({ blanks, onChange, onInsertToken, onRemoveToken, disabled }: BlankEditorProps) {
  const { t } = useTranslation();

  function updateAnswers(index: number, answers: string[]) {
    onChange(blanks.map((b, i) => (i === index ? { ...b, accepted_answers: answers } : b)));
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
    onChange([...blanks, { index: nextIndex, accepted_answers: [""] }]);
  }

  return (
    <div className="space-y-3">
      {blanks.map((blank, index) => (
        <div key={index} className="flex items-start gap-2 rounded-lg border p-2">
          <div className="w-8 pt-2 text-sm font-mono text-muted-foreground">
            {`{{${blank.index}}}`}
          </div>
          <div className="flex-1">
            <AcceptedAnswerEditor
              answers={blank.accepted_answers}
              onChange={(next) => updateAnswers(index, next)}
              disabled={disabled}
            />
          </div>
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

interface StatementRow {
  index: number;
  body: string;
  is_true: boolean;
}

function buildStatementsFromQuestion(
  statements?: { index: number; body: string; is_true: boolean }[]
): StatementRow[] {
  if (!statements || statements.length === 0) {
    return [
      { index: 1, body: "", is_true: false },
      { index: 2, body: "", is_true: false },
    ];
  }
  return statements.map((s) => ({ index: s.index, body: s.body, is_true: s.is_true }));
}

function renumberStatements(rows: StatementRow[]): StatementRow[] {
  return rows.map((r, i) => ({ ...r, index: i + 1 }));
}

interface StatementEditorProps {
  statements: StatementRow[];
  onChange: (next: StatementRow[]) => void;
  disabled: boolean;
}

function StatementEditor({ statements, onChange, disabled }: StatementEditorProps) {
  const { t } = useTranslation();

  function update(index: number, patch: Partial<StatementRow>) {
    onChange(statements.map((s, i) => (i === index ? { ...s, ...patch } : s)));
  }

  function remove(index: number) {
    if (statements.length <= 1) return;
    onChange(renumberStatements(statements.filter((_, i) => i !== index)));
  }

  function add() {
    onChange([...statements, { index: statements.length + 1, body: "", is_true: false }]);
  }

  return (
    <div className="space-y-3">
      {statements.map((statement, index) => (
        <div key={index} className="flex items-start gap-2 rounded-lg border p-2">
          <div className="w-8 pt-2 text-sm font-mono text-muted-foreground">
            {statement.index}
          </div>
          <div className="flex-1 space-y-2">
            <Input
              aria-label={t("tests_field_statement_body")}
              value={statement.body}
              onChange={(e) => update(index, { body: e.target.value })}
              placeholder={t("tests_field_statement_body")}
              disabled={disabled}
            />
            {/* Segmented Benar/Salah rather than a lone checkbox — a checkbox
                labelled "Benar" reads as "checked = done", not "this statement
                is true", and the false state is invisible (user, 2026-07-31). */}
            <div
              role="group"
              aria-label={`${t("tests_field_statement_is_true")}/${t("tests_field_statement_is_false")}`}
              className="inline-flex rounded-md border p-0.5"
            >
              <button
                type="button"
                aria-pressed={statement.is_true}
                aria-label={t("tests_field_statement_is_true")}
                onClick={() => update(index, { is_true: true })}
                disabled={disabled}
                className={
                  statement.is_true
                    ? "rounded bg-primary px-3 py-1 text-xs font-medium text-primary-foreground"
                    : "rounded px-3 py-1 text-xs font-medium text-muted-foreground hover:text-foreground"
                }
              >
                {t("tests_field_statement_is_true")}
              </button>
              <button
                type="button"
                aria-pressed={!statement.is_true}
                aria-label={t("tests_field_statement_is_false")}
                onClick={() => update(index, { is_true: false })}
                disabled={disabled}
                className={
                  !statement.is_true
                    ? "rounded bg-destructive/90 px-3 py-1 text-xs font-medium text-white"
                    : "rounded px-3 py-1 text-xs font-medium text-muted-foreground hover:text-foreground"
                }
              >
                {t("tests_field_statement_is_false")}
              </button>
            </div>
          </div>
          <Button
            type="button"
            size="icon-xs"
            variant="ghost"
            onClick={() => remove(index)}
            disabled={disabled || statements.length <= 1}
            aria-label={t("tests_remove_statement")}
          >
            <Trash2 className="size-3" />
          </Button>
        </div>
      ))}
      <Button type="button" size="sm" variant="outline" onClick={add} disabled={disabled}>
        <Plus className="mr-1 size-4" />
        {t("tests_add_statement")}
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

const FORMAT_LABELS: Record<QuestionFormat, "tests_format_mcq" | "tests_format_multi_answer" | "tests_format_short" | "tests_format_fill_blank" | "tests_format_essay" | "tests_format_multi_blank" | "tests_format_true_false"> = {
  mcq: "tests_format_mcq",
  multi_answer: "tests_format_multi_answer",
  short: "tests_format_short",
  fill_blank: "tests_format_fill_blank",
  essay: "tests_format_essay",
  multi_blank: "tests_format_multi_blank",
  true_false: "tests_format_true_false",
};

// short and fill_blank are retired from authoring (user decision, 2026-07-31):
// they graded and rendered identically, and the inline-box use case both were
// reached for is multi_blank's — with one blank when one is enough. Existing
// questions in either format stay editable (see formatOptions below), gradeable
// and renderable; only NEW questions can no longer pick them.
const CREATABLE_FORMATS: QuestionFormat[] = ["mcq", "multi_answer", "essay", "multi_blank", "true_false"];


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

function buildAcceptedAnswersFromQuestion(q?: Question): string[] {
  if (!q) return [""];
  if (q.accepted_answers && q.accepted_answers.length > 0) return q.accepted_answers;
  if (q.correct_answer) return [q.correct_answer];
  return [""];
}

function buildBlanksFromQuestion(
  blanks?: Array<{ index: number; correct_answer: string; accepted_answers?: string[] }>
): BlankRow[] {
  if (!blanks) {
    return [
      { index: 1, accepted_answers: [""] },
      { index: 2, accepted_answers: [""] },
    ];
  }
  return blanks.map((b) => ({
    index: b.index,
    accepted_answers:
      b.accepted_answers && b.accepted_answers.length > 0
        ? b.accepted_answers
        : b.correct_answer
          ? [b.correct_answer]
          : [""],
  }));
}

function buildInput(
  format: QuestionFormat,
  body: string,
  difficulty: string,
  explanation: string,
  imageUrl: string,
  audioUrl: string,
  acceptedAnswers: string[],
  options: AdminQuestionOptionInput[],
  blanks: BlankRow[],
  statements: StatementRow[],
  pointCorrectRaw: string,
  pointWrongRaw: string,
  topicId: string
): AdminQuestionInput {
  const pointWrongNum = parsePointValue(pointWrongRaw);
  const base: AdminQuestionInput = {
    format,
    body: body.trim(),
    point_correct: parsePointValue(pointCorrectRaw),
    point_wrong: Number.isNaN(pointWrongNum) ? 0 : pointWrongNum,
  };
  if (topicId) base.topic_id = topicId;
  if (difficulty) base.difficulty = difficulty;
  if (explanation.trim()) base.explanation = explanation.trim();
  if (imageUrl.trim()) base.image_url = imageUrl.trim();
  if (audioUrl.trim()) base.audio_url = audioUrl.trim();
  if (format === "short" || format === "fill_blank") {
    const trimmed = acceptedAnswers.map((a) => a.trim());
    base.accepted_answers = trimmed;
    base.correct_answer = trimmed[0] ?? "";
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
    base.blanks = blanks.map((b) => {
      const trimmed = b.accepted_answers.map((a) => a.trim());
      return {
        index: b.index,
        correct_answer: trimmed[0] ?? "",
        accepted_answers: trimmed,
      };
    });
  }
  if (format === "true_false") {
    base.statements = statements.map((s) => ({
      index: s.index,
      body: s.body.trim(),
      is_true: s.is_true,
    }));
  }
  return base;
}

// Preserves decimals (`"2.5"` -> `2.5`), unlike `Number(raw) || fallback`
// which silently turns `0` into the fallback. An empty/invalid string
// parses to NaN so the caller's own bound check catches it.
function parsePointValue(raw: string): number {
  if (raw.trim() === "") return NaN;
  return Number(raw);
}

// FR-23's matching rule: trim, Unicode-lowercase, collapse internal
// whitespace runs to one space.
function normalizeAnswer(s: string): string {
  return s.trim().toLowerCase().replace(/\s+/g, " ");
}

function validateAcceptedAnswers(answers: string[]): { ok: true } | { ok: false; key: string } {
  if (answers.length === 0 || answers.some((a) => a.trim() === "")) {
    return { ok: false, key: "tests_validation_accepted_answers_required" };
  }
  const seen = new Set<string>();
  for (const answer of answers) {
    const normalized = normalizeAnswer(answer);
    if (seen.has(normalized)) {
      return { ok: false, key: "tests_validation_accepted_answers_duplicate" };
    }
    seen.add(normalized);
  }
  return { ok: true };
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
  blanks: Array<{ index: number }>
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

// Mirrors the server's FR-29/FR-30 checks: at least 2 statements, no empty body.
function validateStatements(statements: StatementRow[]): { ok: true } | { ok: false; key: string } {
  if (statements.length < 2) {
    return { ok: false, key: "tests_validation_statements_min" };
  }
  if (statements.some((s) => s.body.trim() === "")) {
    return { ok: false, key: "tests_validation_statement_body_required" };
  }
  return { ok: true };
}

function validate(
  format: QuestionFormat,
  body: string,
  acceptedAnswers: string[],
  options: AdminQuestionOptionInput[],
  blanks: BlankRow[],
  statements: StatementRow[],
  topicId: string,
  pointCorrectRaw: string
): { ok: true } | { ok: false; key: string } {
  if (!topicId) {
    return { ok: false, key: "tests_validation_topic_required" };
  }
  if (!body.trim()) {
    return { ok: false, key: "tests_validation_body_required" };
  }
  const pointCorrectNum = parsePointValue(pointCorrectRaw);
  if (!(pointCorrectNum > 0)) {
    return { ok: false, key: "tests_validation_point_positive" };
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
    const result = validateAcceptedAnswers(acceptedAnswers);
    if (!result.ok) return result;
  }
  if (format === "multi_blank") {
    const tokenResult = validateBlankTokens(body, blanks);
    if (!tokenResult.ok) return tokenResult;
    for (const blank of blanks) {
      const result = validateAcceptedAnswers(blank.accepted_answers);
      if (!result.ok) return result;
    }
  }
  if (format === "true_false") {
    const result = validateStatements(statements);
    if (!result.ok) return result;
  }
  return { ok: true };
}

export function QuestionEditor({ testId, question, onCancel, onSaved }: QuestionEditorProps) {
  const { t } = useTranslation();
  const isEdit = Boolean(question);
  const isTestScoped = Boolean(testId);
  const formatLocked = Boolean(question?.in_live_exam);
  const [format, setFormat] = useState<QuestionFormat>(question?.question.format ?? "mcq");
  // Editing a legacy short/fill_blank question must keep its format selectable —
  // a controlled <select> whose value has no matching <option> renders blank.
  const formatOptions: QuestionFormat[] = CREATABLE_FORMATS.includes(format)
    ? CREATABLE_FORMATS
    : [format, ...CREATABLE_FORMATS];
  const [body, setBody] = useState(question?.question.body ?? "");
  const [difficulty, setDifficulty] = useState<string>(question?.question.difficulty ?? "");
  const [explanation, setExplanation] = useState(question?.question.explanation ?? "");
  const [imageUrl, setImageUrl] = useState(question?.question.image_url ?? "");
  const [audioUrl, setAudioUrl] = useState(question?.question.audio_url ?? "");
  const [acceptedAnswers, setAcceptedAnswers] = useState<string[]>(
    buildAcceptedAnswersFromQuestion(question?.question)
  );
  const [pointCorrect, setPointCorrect] = useState(String(question?.question.point_correct ?? 1));
  const [pointWrong, setPointWrong] = useState(String(question?.question.point_wrong ?? 0));
  const [topicId, setTopicId] = useState(question?.question.topic_id ?? "");
  const [options, setOptions] = useState<AdminQuestionOptionInput[]>(
    question ? buildOptionsFromQuestion(question) : [
      { key: "a", text: "", is_correct: true, sort_order: 1 },
      { key: "b", text: "", is_correct: false, sort_order: 2 },
    ]
  );
  const [blanks, setBlanks] = useState<BlankRow[]>(buildBlanksFromQuestion(question?.blanks));
  const [statements, setStatements] = useState<StatementRow[]>(
    buildStatementsFromQuestion(question?.question.statements)
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
      setAcceptedAnswers([""]);
      setPointCorrect("1");
      setPointWrong("0");
      setTopicId("");
      setOptions([
        { key: "a", text: "", is_correct: true, sort_order: 1 },
        { key: "b", text: "", is_correct: false, sort_order: 2 },
      ]);
      setBlanks([
        { index: 1, accepted_answers: [""] },
        { index: 2, accepted_answers: [""] },
      ]);
      setStatements([
        { index: 1, body: "", is_true: false },
        { index: 2, body: "", is_true: false },
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
    const result = validate(format, body, acceptedAnswers, options, blanks, statements, topicId, pointCorrect);
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
      acceptedAnswers,
      options,
      blanks,
      statements,
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
      if (e instanceof ApiError && e.code === "question_format_locked") {
        toast.error(t("question_format_locked_reason"));
      } else {
        toast.error(e instanceof Error ? e.message : t("error_generic"));
      }
    }
  }

  const showOptions = format === "mcq" || format === "multi_answer";
  const showCorrectAnswer = format === "short" || format === "fill_blank";
  const showBlanks = format === "multi_blank";
  const showStatements = format === "true_false";
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
                  disabled={savePending || formatLocked}
                  className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-brand-300/50 disabled:pointer-events-none disabled:opacity-50"
                >
                  {formatOptions.map((f) => (
                    <option key={f} value={f}>
                      {t(FORMAT_LABELS[f])}
                    </option>
                  ))}
                </select>
                {formatLocked && (
                  <p className="text-xs text-muted-foreground">{t("tests_format_locked_hint")}</p>
                )}
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

            {(showOptions || showCorrectAnswer || showBlanks || showStatements) && (
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
                    <Label>{t("tests_field_accepted_answers")}</Label>
                    <AcceptedAnswerEditor
                      answers={acceptedAnswers}
                      onChange={setAcceptedAnswers}
                      disabled={savePending}
                    />
                  </div>
                )}

                {showBlanks && (
                  <div className="grid gap-2">
                    <Label>{t("tests_field_accepted_answers")}</Label>
                    <BlankEditor
                      blanks={blanks}
                      onChange={setBlanks}
                      onInsertToken={insertBlankToken}
                      onRemoveToken={removeBlankToken}
                      disabled={savePending}
                    />
                  </div>
                )}

                {showStatements && (
                  <div className="grid gap-2">
                    <Label>{t("tests_field_statements")}</Label>
                    <StatementEditor
                      statements={statements}
                      onChange={setStatements}
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
