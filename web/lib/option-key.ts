import type { QuestionFormat } from "@/lib/types";

const CHOICE_FORMATS: ReadonlySet<QuestionFormat> = new Set([
  "mcq",
  "multi_answer",
]);

/** Display form of an option key (`a` → `A`). Identity stays the stored key. */
export function optionKeyLabel(key: string): string {
  return key.trim().toUpperCase();
}

/**
 * Pembahasan / report display for a stored answer. Choice formats are the
 * option keys (possibly comma-joined); everything else is shown as stored.
 */
export function formatChoiceAnswer(
  answer: string | null | undefined,
  format: QuestionFormat,
): string {
  if (answer == null) return "";
  if (!CHOICE_FORMATS.has(format)) return answer;
  return answer
    .split(",")
    .map((token) => optionKeyLabel(token))
    .filter(Boolean)
    .join(", ");
}
