import { describe, it, expect } from "vitest";
import type { Question, AdminQuestionInput, BankQuestionListItem, QuestionWithOptions, QuestionFormat } from "@/lib/types";

describe("E2 shared frontend contract — widened question types", () => {
  it("QuestionFormat includes true_false", () => {
    const formats: QuestionFormat[] = [
      "mcq",
      "multi_answer",
      "short",
      "fill_blank",
      "essay",
      "multi_blank",
      "true_false",
    ];
    expect(formats).toContain("true_false");
  });

  it("Question carries question_number, accepted_answers, statements and in_live_exam", () => {
    const q: Question = {
      id: "q1",
      format: "true_false",
      body: "Statements below",
      sort_order: 1,
      point_correct: 1,
      point_wrong: 0.5,
      question_number: 42,
      accepted_answers: ["2", "dua"],
      statements: [
        { index: 1, body: "Jakarta is the capital", is_true: true },
        { index: 2, body: "The earth is flat", is_true: false },
      ],
      in_live_exam: true,
    };
    expect(q.question_number).toBe(42);
    expect(q.accepted_answers).toEqual(["2", "dua"]);
    expect(q.statements).toHaveLength(2);
    expect(q.statements?.[0].is_true).toBe(true);
    expect(q.in_live_exam).toBe(true);
  });

  it("Question still accepts a minimal literal without the new fields", () => {
    const q: Question = {
      id: "q1",
      format: "mcq",
      body: "Body",
      sort_order: 1,
      point_correct: 1,
      point_wrong: 0,
    };
    expect(q.question_number).toBeUndefined();
  });

  it("QuestionWithOptions/BankQuestionListItem blanks accept per-blank accepted_answers", () => {
    const qwo: QuestionWithOptions = {
      question: {
        id: "q1",
        format: "multi_blank",
        body: "{{1}} and {{2}}",
        sort_order: 1,
        point_correct: 2,
        point_wrong: 1,
      },
      options: [],
      blanks: [{ index: 1, correct_answer: "4", accepted_answers: ["4", "empat"] }],
    };
    expect(qwo.blanks?.[0].accepted_answers).toEqual(["4", "empat"]);

    const bqli: BankQuestionListItem = {
      question: qwo.question,
      options: [],
      attached_count: 0,
      blanks: [{ index: 1, correct_answer: "4", accepted_answers: ["4", "empat"] }],
    };
    expect(bqli.blanks?.[0].accepted_answers).toEqual(["4", "empat"]);
  });

  it("AdminQuestionInput accepts accepted_answers, statements and blanks[].accepted_answers", () => {
    const input: AdminQuestionInput = {
      format: "true_false",
      body: "Body",
      point_correct: 1,
      point_wrong: 0.5,
      statements: [
        { index: 1, body: "A", is_true: true },
        { index: 2, body: "B", is_true: false },
      ],
    };
    expect(input.statements).toHaveLength(2);

    const shortInput: AdminQuestionInput = {
      format: "short",
      body: "Body",
      point_correct: 1,
      point_wrong: 0,
      accepted_answers: ["2", "dua"],
    };
    expect(shortInput.accepted_answers).toEqual(["2", "dua"]);

    const multiBlankInput: AdminQuestionInput = {
      format: "multi_blank",
      body: "{{1}}",
      point_correct: 1,
      point_wrong: 0,
      blanks: [{ index: 1, correct_answer: "4", accepted_answers: ["4", "empat"] }],
    };
    expect(multiBlankInput.blanks?.[0].accepted_answers).toEqual(["4", "empat"]);
  });
});
