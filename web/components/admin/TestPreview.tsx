"use client";

import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { useTranslation } from "@/lib/i18n";
import type { QuestionWithOptions, Test } from "@/lib/types";
import { MultiBlankPreviewBody } from "./QuestionPreview";
import { RichContent } from "./RichContent";

interface TestPreviewProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  test: Test;
  questions: QuestionWithOptions[];
  initialIndex?: number;
}

function hasContent(html?: string | null): html is string {
  return Boolean(html?.trim());
}

function PreviewQuestion({ item, number }: { item: QuestionWithOptions; number: number }) {
  const { t } = useTranslation();
  const { question, options } = item;
  const choice = question.format === "mcq" || question.format === "multi_answer";

  return (
    <section className="min-w-0 space-y-3 break-words [overflow-wrap:anywhere]">
      {hasContent(question.material_html) && (
        <RichContent html={question.material_html} className="rounded-lg bg-muted/40 p-3 text-sm" />
      )}
      <div className="flex gap-2 text-sm">
        <span className="font-semibold">{number}.</span>
        <div className="min-w-0 flex-1">
          {question.format === "multi_blank" ? (
            <MultiBlankPreviewBody html={question.body} />
          ) : (
            <RichContent html={question.body} />
          )}
          {question.image_url && (
            <img src={question.image_url} alt="" className="mt-3 max-h-48 max-w-full rounded-md object-contain" />
          )}
        </div>
      </div>
      {choice && (
        <ul className="space-y-2">
          {options.map((opt) => (
            <li key={opt.key} className="flex items-center gap-3 rounded-lg border p-3 text-sm">
              <input
                type={question.format === "mcq" ? "radio" : "checkbox"}
                disabled
                aria-label={opt.key.toUpperCase()}
              />
              <span className="w-5 font-mono font-medium uppercase">{opt.key}</span>
              <div className="min-w-0 flex-1">
                <RichContent html={opt.text} />
              </div>
            </li>
          ))}
        </ul>
      )}
      {question.format === "true_false" && (
        <ul className="space-y-2">
          {[...(question.statements ?? [])]
            .sort((a, b) => a.index - b.index)
            .map((statement) => (
              <li key={statement.index} className="flex flex-wrap items-center gap-3 rounded-lg border p-3 text-sm">
                <div className="min-w-0 flex-1">
                  <RichContent html={statement.body} />
                </div>
                {[true, false].map((value) => (
                  <label key={String(value)} className="flex items-center gap-1">
                    <input type="radio" disabled name={`${question.id}-${statement.index}`} />
                    {t(value ? "tests_field_statement_is_true" : "tests_field_statement_is_false")}
                  </label>
                ))}
              </li>
            ))}
        </ul>
      )}
      {question.format === "essay" && (
        <textarea disabled rows={4} aria-label={t("tests_format_essay")} className="w-full rounded-md border border-input bg-muted/30 p-2 text-sm" />
      )}
      {(question.format === "short" || question.format === "fill_blank") && (
        <input disabled aria-label={t("tests_format_short")} className="h-9 w-full rounded-md border border-input bg-muted/30 px-2 text-sm" />
      )}
    </section>
  );
}

export function TestPreview({ open, onOpenChange, test, questions, initialIndex = 0 }: TestPreviewProps) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"one" | "all">("one");
  const [index, setIndex] = useState(initialIndex);
  const allBlocked = questions.some((item) => hasContent(item.question.material_html));

  useEffect(() => {
    if (open) {
      setIndex(Math.min(initialIndex, Math.max(questions.length - 1, 0)));
      setMode("one");
    }
  }, [open, initialIndex, questions.length]);

  const visible = mode === "all" && !allBlocked ? questions : questions.slice(index, index + 1);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-3xl">
        <DialogHeader>
          <DialogTitle>{test.title}</DialogTitle>
          <DialogDescription>{t("tests_preview_readonly")}</DialogDescription>
        </DialogHeader>

        <div className="min-w-0 space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            {(["one", "all"] as const).map((value) => (
              <Button
                key={value}
                type="button"
                size="sm"
                variant={mode === value ? "default" : "outline"}
                aria-pressed={mode === value}
                disabled={value === "all" && allBlocked}
                onClick={() => setMode(value)}
              >
                {t(value === "one" ? "tests_preview_mode_one" : "tests_preview_mode_all")}
              </Button>
            ))}
            {allBlocked && <span className="text-xs text-ink-500">{t("tests_preview_all_blocked")}</span>}
          </div>

          {hasContent(test.instructions_html) && (
            <div className="rounded-lg border p-3 text-sm">
              <p className="mb-1 font-medium">{t("tests_preview_instructions")}</p>
              <RichContent html={test.instructions_html} />
            </div>
          )}
          {hasContent(test.shared_material_html) && (
            <RichContent html={test.shared_material_html} className="max-h-72 overflow-y-auto rounded-lg bg-muted/40 p-3 text-sm" />
          )}

          {questions.length === 0 ? (
            <p className="py-6 text-center text-sm text-ink-500">{t("tests_preview_no_questions")}</p>
          ) : (
            <div className="space-y-6">
              {visible.map((item) => (
                <PreviewQuestion
                  key={item.question.id}
                  item={item}
                  number={questions.indexOf(item) + 1}
                />
              ))}
            </div>
          )}

          {questions.length > 0 && mode === "one" && (
            <div className="flex items-center justify-between gap-2 border-t pt-3">
              <Button type="button" variant="outline" size="sm" disabled={index === 0} onClick={() => setIndex(index - 1)}>
                {t("tests_preview_prev")}
              </Button>
              <span className="text-sm text-ink-500">
                {t("tests_preview_question_of")
                  .replace("{n}", String(index + 1))
                  .replace("{total}", String(questions.length))}
              </span>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={index >= questions.length - 1}
                onClick={() => setIndex(index + 1)}
              >
                {t("tests_preview_next")}
              </Button>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
