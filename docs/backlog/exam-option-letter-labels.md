# Exam overlay shows option letters A/B/C/D

| | |
|---|---|
| **Status** | Shipped — display `options[].key` as a letter on the live overlay and uppercase those keys on pembahasan |
| **Date** | 2026-08-20 |
| **Surface** | Student exam overlay (`mcq` / `multi_answer`); student session result pembahasan; admin school-report drill-down |
| **Not this change** | New API field; option shuffle; pembahasan showing option *text* (`B. Jakarta`); admin QuestionEditor / QuestionPreview (already labelled) |

Choice identity already lives on the bank row as `question_option.key` (usually `a`–`d`, composite PK with `question_id`). The student session JSON already sends `key`. The overlay posted that key as the answer but did not show the letter next to the choice. Pembahasan printed the raw stored string (`b`, `a,c`).

---

## What the student sees

**During the exam:** each MCQ / multi-answer row is letter, control, then option HTML:

```
A  ○  Jakarta
B  ○  Bandung
```

**After the exam** (`score_pembahasan`): Jawaban Anda / Jawaban Benar for those formats show `B` or `A, C`. Short, fill-blank, essay, and true/false stay as stored text or JSON.

The letter is CSS/display uppercase. The value posted and graded is still the stored `key` (typically lowercase). Grading already uses `EqualFold`.

---

## Why `key`, not array index

`key` is what import writes (`option_a` → `a`), what the session saves, and what pembahasan returns as `your_answer` / `correct_answer`. Labelling from position would drift if `sort_order` and key ever disagree, or if a fifth option is `e`.

Display helper: `web/lib/option-key.ts` (`optionKeyLabel`, `formatChoiceAnswer`). Overlay test ids are `option-key-${opt.key}` so they do not collide with option copy such as “Opt A”.

---

## Out of scope

Pembahasan JSON has no option list — only the key string. Showing `B. Jakarta` would need options (or text) on that payload.

---

## Acceptance

- MCQ and multi-answer rows on the live overlay show an uppercase letter from `opt.key` next to the option text.
- Selecting still persists the original `key`.
- Pembahasan (student + school report) shows uppercase letters for those formats only.
- Formats without options are unchanged.
- More than four options (`e`–`h`) still get their stored key, not a forced A–D.
