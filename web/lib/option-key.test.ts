import { describe, it, expect } from "vitest";
import { optionKeyLabel, formatChoiceAnswer } from "@/lib/option-key";

describe("optionKeyLabel", () => {
  it("uppercases and trims the stored key", () => {
    expect(optionKeyLabel("a")).toBe("A");
    expect(optionKeyLabel(" B ")).toBe("B");
    expect(optionKeyLabel("e")).toBe("E");
  });
});

describe("formatChoiceAnswer", () => {
  it("uppercases a single mcq key", () => {
    expect(formatChoiceAnswer("b", "mcq")).toBe("B");
  });

  it("uppercases comma-separated multi_answer keys", () => {
    expect(formatChoiceAnswer("a,c", "multi_answer")).toBe("A, C");
    expect(formatChoiceAnswer(" a, c ", "multi_answer")).toBe("A, C");
  });

  it("passes non-choice formats through unchanged", () => {
    expect(formatChoiceAnswer("4", "short")).toBe("4");
    expect(formatChoiceAnswer('["true","false"]', "true_false")).toBe(
      '["true","false"]',
    );
    expect(formatChoiceAnswer("Jakarta", "fill_blank")).toBe("Jakarta");
  });

  it("returns empty string for null or undefined", () => {
    expect(formatChoiceAnswer(null, "mcq")).toBe("");
    expect(formatChoiceAnswer(undefined, "mcq")).toBe("");
  });
});
