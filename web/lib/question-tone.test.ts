import { describe, it, expect } from "vitest";
import { FORMAT_TONE } from "@/lib/question-tone";
import type { QuestionFormat } from "@/lib/types";

describe("FORMAT_TONE", () => {
  it("has an entry for every QuestionFormat, including true_false", () => {
    const formats: QuestionFormat[] = [
      "mcq",
      "multi_answer",
      "short",
      "fill_blank",
      "essay",
      "multi_blank",
      "true_false",
    ];
    for (const format of formats) {
      expect(FORMAT_TONE[format]).toBeTruthy();
    }
  });
});
