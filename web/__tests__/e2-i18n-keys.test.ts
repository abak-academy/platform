import { describe, it, expect } from "vitest";
import { DICT } from "@/lib/i18n";

type Dict = Record<string, string>;
const idDict = DICT.id as Dict;
const enDict = DICT.en as Dict;

describe("E2 shared frontend contract — i18n key parity", () => {
  it("id and en dictionaries expose the exact same key set", () => {
    const idKeys = Object.keys(idDict).sort();
    const enKeys = Object.keys(enDict).sort();
    expect(idKeys).toEqual(enKeys);
  });

  const NEW_KEYS = [
    "fmt_true_false",
    "tests_format_true_false",
    "tests_field_statements",
    "tests_field_statement_body",
    "tests_field_statement_is_true",
    "tests_add_statement",
    "tests_remove_statement",
    "tests_validation_statements_min",
    "tests_field_accepted_answers",
    "tests_add_accepted_answer",
    "tests_remove_accepted_answer",
    "tests_validation_accepted_answers_required",
    "tests_validation_accepted_answers_duplicate",
    "tests_validation_accepted_answers_not_allowed",
    "questions_confirm_delete",
    "questions_deleted",
    "questions_delete_failed",
    "question_in_published_exam_reason",
    "question_format_locked_reason",
    "questions_download_template",
    "tests_format_locked_hint",
  ];

  it.each(NEW_KEYS)("DICT has key '%s' in both id and en, non-empty", (key) => {
    expect(idDict[key]).toBeTruthy();
    expect(enDict[key]).toBeTruthy();
  });
});
