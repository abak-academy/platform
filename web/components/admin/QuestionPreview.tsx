"use client";

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
            <RichContent html={question.body} />
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
                  {opt.is_correct && <Badge variant="default">{t("tests_field_option_is_correct")}</Badge>}
                </div>
              ))}
            </div>
          )}

          {showBlanks && blanks && blanks.length > 0 && (
            <div className="space-y-2">
              {blanks.map((blank) => (
                <div
                  key={blank.index}
                  className="rounded-lg border p-3 text-sm border-primary/50 bg-primary/5"
                >
                  <div className="flex items-center gap-2 mb-2">
                    <span className="text-xs font-medium text-muted-foreground">
                      {t("tests_format_multi_blank")} #{blank.index}
                    </span>
                  </div>
                  <p className="font-medium">{blank.correct_answer}</p>
                </div>
              ))}
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
              <span className="text-muted-foreground">{t("tests_field_correct_answer")}</span>
              <p className="font-medium">{question.correct_answer || "—"}</p>
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
