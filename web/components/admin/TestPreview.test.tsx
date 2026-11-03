import { describe, it, expect } from "vitest";
import { render, screen, fireEvent, within } from "@testing-library/react";
import { TestPreview } from "./TestPreview";
import type { QuestionWithOptions, Test } from "@/lib/types";

const test: Test = {
  id: "t1",
  title: "Tryout",
  subject: "Matematika",
  topic: "Aljabar",
  duration_minutes: 60,
  instructions_html: "<p>Kerjakan dengan teliti</p>",
  shared_material_html: "<p>Bacaan bersama</p><script>alert(1)</script>",
};

function q(id: string, format: QuestionWithOptions["question"]["format"], extra: Partial<QuestionWithOptions["question"]> = {}, rest: Partial<QuestionWithOptions> = {}): QuestionWithOptions {
  return {
    question: { id, format, body: `<p>Isi ${id}</p>`, sort_order: 1, point_correct: 1, point_wrong: 0, ...extra },
    options: [],
    ...rest,
  };
}

const mcq = q("q1", "mcq", {}, {
  options: [
    { question_id: "q1", key: "a", text: "Jakarta", is_correct: true, sort_order: 1 },
    { question_id: "q1", key: "b", text: "Bandung", is_correct: false, sort_order: 2 },
  ],
});
const essay = q("q2", "essay");
const trueFalse = q("q3", "true_false", { statements: [{ index: 1, body: "Bumi bulat", is_true: true }] });
const multiBlank = q("q4", "multi_blank", { body: "<p>Ibu kota {{1}}</p>" }, {
  blanks: [{ index: 1, correct_answer: "Jakarta" }],
});
const legacyShort = q("q5", "short", { correct_answer: "2" });

function renderPreview(questions: QuestionWithOptions[], props: { initialIndex?: number } = {}) {
  return render(<TestPreview open onOpenChange={() => {}} test={test} questions={questions} {...props} />);
}

describe("TestPreview", () => {
  it("is read-only: states that no session is created and shows no correct-answer markers", () => {
    renderPreview([mcq]);
    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/tidak ada sesi atau percobaan/i)).toBeInTheDocument();
    expect(within(dialog).queryByText(/jawaban benar/i)).not.toBeInTheDocument();
    for (const input of dialog.querySelectorAll("input, textarea")) {
      expect(input).toBeDisabled();
    }
  });

  it("renders instructions and sanitized shared material", () => {
    renderPreview([mcq]);
    expect(screen.getByText("Kerjakan dengan teliti")).toBeInTheDocument();
    expect(screen.getByText("Bacaan bersama")).toBeInTheDocument();
    expect(document.querySelector("script")).toBeNull();
  });

  it("shows one question at a time and steps through them", () => {
    renderPreview([mcq, essay]);
    expect(screen.getByText("Isi q1")).toBeInTheDocument();
    expect(screen.queryByText("Isi q2")).not.toBeInTheDocument();
    expect(screen.getByText("Soal 1 dari 2")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /berikutnya/i }));
    expect(screen.getByText("Isi q2")).toBeInTheDocument();
    expect(screen.queryByText("Isi q1")).not.toBeInTheDocument();
  });

  it("opens at the requested question", () => {
    renderPreview([mcq, essay], { initialIndex: 1 });
    expect(screen.getByText("Isi q2")).toBeInTheDocument();
  });

  it("shows every question in all-questions mode", () => {
    renderPreview([mcq, essay]);
    fireEvent.click(screen.getByRole("button", { name: /semua soal/i }));
    expect(screen.getByText("Isi q1")).toBeInTheDocument();
    expect(screen.getByText("Isi q2")).toBeInTheDocument();
  });

  it("blocks all-questions mode when a question carries its own material", () => {
    renderPreview([mcq, q("q6", "mcq", { material_html: "<p>Materi soal</p>" })]);
    expect(screen.getByRole("button", { name: /semua soal/i })).toBeDisabled();
    expect(screen.getByText(/materi sendiri/i)).toBeInTheDocument();
  });

  it("renders every answer format", () => {
    renderPreview([mcq, essay, trueFalse, multiBlank, legacyShort]);
    fireEvent.click(screen.getByRole("button", { name: /semua soal/i }));
    expect(screen.getByText("Jakarta")).toBeInTheDocument();
    expect(screen.getByText("Bandung")).toBeInTheDocument();
    expect(screen.getAllByRole("textbox").length).toBeGreaterThanOrEqual(2);
    expect(screen.getByText("Bumi bulat")).toBeInTheDocument();
    expect(screen.getAllByText("Benar").length).toBeGreaterThan(0);
    expect(screen.getByText(/ibu kota/i)).toBeInTheDocument();
  });

  it("explains an empty test", () => {
    renderPreview([]);
    expect(screen.getByText(/belum memiliki soal/i)).toBeInTheDocument();
  });
});
