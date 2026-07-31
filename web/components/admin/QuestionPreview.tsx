"use client";

import { useEffect, useRef } from "react";
import { Pencil } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useTranslation } from "@/lib/i18n";
import { RichContent } from "./RichContent";
import type { BankQuestionListItem, QuestionFormat } from "@/lib/types";

const FORMAT_LABELS: Record<QuestionFormat, "fmt_mcq" | "fmt_multi_answer" | "fmt_short" | "fmt_fill_blank" | "fmt_essay" | "fmt_multi_blank" | "fmt_true_false"> = {
  mcq: "fmt_mcq",
  multi_answer: "fmt_multi_answer",
  short: "fmt_short",
  fill_blank: "fmt_fill_blank",
  essay: "fmt_essay",
  multi_blank: "fmt_multi_blank",
  true_false: "fmt_true_false",
};

const DIFFICULTY_LABELS: Record<string, "diff_easy" | "diff_medium" | "diff_hard"> = {
  easy: "diff_easy",
  medium: "diff_medium",
  hard: "diff_hard",
};

// The student session replaces {{N}} tokens with real inputs (MultiBlankInput);
// without the same treatment here the admin previews literal "{{1}}" text and
// reads it as a broken question. Renders each token as an inert box chip after
// RichContent has sanitised — the effect runs on the final DOM, so the chip
// markup never has to survive the sanitiser's allowlist.
function MultiBlankPreviewBody({ html }: { html: string }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const root = ref.current;
    if (!root) return;
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, null);
    const targets: Text[] = [];
    let node: Node | null;
    while ((node = walker.nextNode())) {
      if (/\{\{\d+\}\}/.test(node.textContent || "")) targets.push(node as Text);
    }
    for (const textNode of targets) {
      const fragment = document.createDocumentFragment();
      for (const part of (textNode.textContent || "").split(/(\{\{\d+\}\})/)) {
        const m = part.match(/^\{\{(\d+)\}\}$/);
        if (m) {
          const chip = document.createElement("span");
          chip.textContent = `#${m[1]}`;
          chip.setAttribute("data-blank-chip", m[1]);
          chip.className =
            "mx-1 inline-block min-w-16 rounded border border-dashed border-primary/60 bg-primary/5 px-2 text-center text-xs font-medium leading-6 text-primary align-baseline";
          fragment.appendChild(chip);
        } else if (part) {
          fragment.appendChild(document.createTextNode(part));
        }
      }
      textNode.parentNode?.replaceChild(fragment, textNode);
    }
  }, [html]);
  return (
    <div ref={ref}>
      <RichContent html={html} />
    </div>
  );
}

interface QuestionPreviewProps {
  item?: BankQuestionListItem | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onEdit: () => void;
}

export function QuestionPreview({ item, open, onOpenChange, onEdit }: QuestionPreviewProps) {
  const { t } = useTranslation();
  if (!item) return null;

  const { question, options, blanks } = item;
  const showOptions = question.format === "mcq" || question.format === "multi_answer";
  const showBlanks = question.format === "multi_blank";
  const showStatements = question.format === "true_false";
  const formatKey = FORMAT_LABELS[question.format];
  const difficultyKey = question.difficulty ? DIFFICULTY_LABELS[question.difficulty] : undefined;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("question")}</DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="flex flex-wrap items-center gap-2 text-sm">
            <Badge variant="outline">{t(formatKey)}</Badge>
            {difficultyKey && <Badge variant="secondary">{t(difficultyKey)}</Badge>}
            {question.topic && <span className="text-muted-foreground">{question.topic}</span>}
          </div>

          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <span className="text-muted-foreground">{t("tests_field_point_correct")}</span>
              <p className="font-medium">{question.point_correct}</p>
            </div>
            <div>
              <span className="text-muted-foreground">{t("tests_field_point_wrong")}</span>
              <p className="font-medium">{question.point_wrong}</p>
            </div>
          </div>

          <div className="rounded-lg border p-3 text-sm">
            {showBlanks ? (
              <MultiBlankPreviewBody html={question.body} />
            ) : (
              <RichContent html={question.body} />
            )}
            {question.image_url && (
              <img
                src={question.image_url}
                alt=""
                className="mt-3 max-h-48 rounded-md object-contain"
              />
            )}
          </div>

          {showOptions && (
            <div className="space-y-2">
              {options.map((opt) => (
                <div
                  key={opt.key}
                  className={`flex items-center gap-3 rounded-lg border p-3 text-sm ${
                    opt.is_correct ? "border-primary/50 bg-primary/5" : ""
                  }`}
                >
                  <span className="w-6 text-center font-mono uppercase font-medium">
                    {opt.key}
                  </span>
                  <div className="flex-1">
                    <RichContent html={opt.text} />
                  </div>
                  {opt.is_correct && opt.points !== undefined && (
                    <Badge variant="outline" className="border-transparent bg-primary/10 text-primary">
                      {opt.points} {t("tests_field_item_points").toLowerCase()}
                    </Badge>
                  )}
                  {opt.is_correct && <Badge variant="default">{t("tests_field_option_is_correct")}</Badge>}
                </div>
              ))}
            </div>
          )}

          {showBlanks && blanks && blanks.length > 0 && (
            <div className="space-y-2">
              {blanks.map((blank) => {
                // The grader accepts EVERY entry in accepted_answers (all
                // case-insensitive) — showing only correct_answer here made
                // admins believe the extra answers were never saved.
                const accepted =
                  blank.accepted_answers && blank.accepted_answers.length > 0
                    ? blank.accepted_answers
                    : [blank.correct_answer];
                return (
                  <div
                    key={blank.index}
                    className="rounded-lg border p-3 text-sm border-primary/50 bg-primary/5"
                  >
                    <div className="flex items-center gap-2 mb-2">
                      <span className="text-xs font-medium text-muted-foreground">
                        {t("tests_format_multi_blank")} #{blank.index}
                      </span>
                      {blank.points !== undefined && (
                        <Badge variant="outline" className="border-transparent bg-primary/10 text-primary">
                          {blank.points} {t("tests_field_item_points").toLowerCase()}
                        </Badge>
                      )}
                    </div>
                    <div className="flex flex-wrap gap-1.5">
                      {accepted.map((answer, i) => (
                        <span
                          key={i}
                          className={
                            i === 0
                              ? "rounded bg-primary/15 px-2 py-0.5 font-medium"
                              : "rounded bg-muted px-2 py-0.5"
                          }
                        >
                          {answer}
                        </span>
                      ))}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {showStatements && question.statements && question.statements.length > 0 && (
            <div className="space-y-2">
              {[...question.statements]
                .sort((a, b) => a.index - b.index)
                .map((statement) => (
                  <div
                    key={statement.index}
                    className="flex items-center gap-3 rounded-lg border p-3 text-sm"
                  >
                    <span className="w-6 text-center font-mono font-medium">
                      {statement.index}
                    </span>
                    <div className="flex-1">
                      <RichContent html={statement.body} />
                    </div>
                    {statement.points !== undefined && (
                      <Badge variant="outline" className="border-transparent bg-primary/10 text-primary">
                        {statement.points} {t("tests_field_item_points").toLowerCase()}
                      </Badge>
                    )}
                    <Badge variant={statement.is_true ? "default" : "outline"}>
                      {statement.is_true
                        ? t("tests_field_statement_is_true")
                        : t("tests_field_statement_is_false")}
                    </Badge>
                  </div>
                ))}
            </div>
          )}

          {(question.format === "short" || question.format === "fill_blank") && (
            <div className="rounded-lg border p-3 text-sm">
              <span className="text-muted-foreground">{t("tests_field_accepted_answers")}</span>
              <div className="mt-1 flex flex-wrap gap-1.5">
                {(question.accepted_answers && question.accepted_answers.length > 0
                  ? question.accepted_answers
                  : [question.correct_answer ?? "—"]
                ).map((answer, i) => (
                  <span
                    key={i}
                    className={
                      i === 0
                        ? "rounded bg-primary/15 px-2 py-0.5 font-medium"
                        : "rounded bg-muted px-2 py-0.5"
                    }
                  >
                    {answer}
                  </span>
                ))}
              </div>
            </div>
          )}

          {question.explanation && (
            <div className="rounded-lg border p-3 text-sm">
              <span className="text-muted-foreground">{t("tests_field_explanation")}</span>
              <p className="mt-1 whitespace-pre-wrap">{question.explanation}</p>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("cancel")}
          </Button>
          <Button type="button" onClick={onEdit}>
            <Pencil className="mr-1 size-4" />
            {t("action_edit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
