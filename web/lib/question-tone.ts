import type { QuestionFormat } from "@/lib/types";

export const FORMAT_TONE: Record<QuestionFormat, string> = {
  mcq: "bg-info-bg text-info",
  multi_answer: "bg-violet-bg text-violet",
  short: "bg-success-bg text-success",
  fill_blank: "bg-line-2 text-ink-600",
  essay: "bg-gold-bg text-gold",
  multi_blank: "bg-line-2 text-ink-600",
  true_false: "bg-violet-bg text-violet",
};

export const DIFFICULTY_TONE: Record<string, string> = {
  easy: "bg-success-bg text-success",
  medium: "bg-warn-bg text-warn",
  hard: "bg-danger-bg text-danger",
};
