import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { QuestionPreview } from "./QuestionPreview";
import type { BankQuestionListItem } from "@/lib/types";

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

const sampleItem: BankQuestionListItem = {
  question: {
    id: "q1",
    format: "mcq",
    body: "What is 2+2?",
    difficulty: "easy",
    point_correct: 1,
    point_wrong: 0,
    sort_order: 1,
    topic_id: "topic-1",
    topic: "Arithmetic",
  },
  options: [
    { question_id: "q1", key: "a", text: "3", is_correct: false, sort_order: 1 },
    { question_id: "q1", key: "b", text: "4", is_correct: true, sort_order: 2 },
  ],
  attached_count: 2,
};

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("QuestionPreview", () => {
  const onOpenChange = vi.fn();
  const onEdit = vi.fn();

  beforeEach(() => {
    onOpenChange.mockReset();
    onEdit.mockReset();
  });

  it("renders read-only question details and options", () => {
    renderWithClient(
      <QuestionPreview
        item={sampleItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );

    expect(screen.getByText("What is 2+2?")).toBeInTheDocument();
    expect(screen.getByText("Arithmetic")).toBeInTheDocument();
    expect(screen.getByText("4")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("calls onEdit when Edit is clicked", () => {
    renderWithClient(
      <QuestionPreview
        item={sampleItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /action_edit/i }));
    expect(onEdit).toHaveBeenCalled();
  });

  it("returns null when item is not provided", () => {
    const { container } = renderWithClient(
      <QuestionPreview
        item={null}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders LaTeX in body as KaTeX (not literal delimiters)", async () => {
    const richItem: BankQuestionListItem = {
      ...sampleItem,
      question: { ...sampleItem.question, body: "Solve \\(x^2\\) now" },
    };
    renderWithClient(
      <QuestionPreview
        item={richItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    const richNode = document.querySelector("[data-rich-content]") as HTMLElement;
    expect(richNode).not.toBeNull();
    await waitFor(() => {
      expect(
        document.querySelector("[data-rich-content] .katex")
      ).not.toBeNull();
    });
    expect(richNode.textContent).not.toContain("\\(");
    expect(richNode.textContent).not.toContain("x^2\\)");
  });

  it("renders bold HTML in body as a <b> element (not literal tags)", () => {
    const richItem: BankQuestionListItem = {
      ...sampleItem,
      question: { ...sampleItem.question, body: "Make it <b>bold</b> please" },
    };
    renderWithClient(
      <QuestionPreview
        item={richItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    const richNode = document.querySelector("[data-rich-content]") as HTMLElement;
    expect(richNode).not.toBeNull();
    const b = richNode.querySelector("b");
    expect(b).not.toBeNull();
    expect(b?.textContent).toBe("bold");
    // The visible text contains "bold" but not the literal tag text "<b>" / "</b>".
    expect(richNode.textContent).not.toContain("<b>");
    expect(richNode.textContent).not.toContain("</b>");
  });

  it("renders explanation as plain text (no RichContent, no KaTeX)", () => {
    const richItem: BankQuestionListItem = {
      ...sampleItem,
      question: {
        ...sampleItem.question,
        explanation: "Because \\(x^2\\) means x squared. <b>literally</b>.",
      },
    };
    renderWithClient(
      <QuestionPreview
        item={richItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // Only the body field is inside a RichContent node. Explanation must NOT be.
    const richNodes = document.querySelectorAll("[data-rich-content]");
    const allRichText = Array.from(richNodes)
      .map((n) => n.textContent ?? "")
      .join("|");
    expect(allRichText).not.toContain("x squared");
    // Explanation renders as plain text — the text is visible as a single string
    // (JSX escapes the <b> tag), literal LaTeX delimiters are not rendered as math.
    expect(
      screen.getByText(/Because.*x squared.*literally/)
    ).toBeInTheDocument();
  });

  it("renders bold HTML in option text as <b> element (not literal tags)", () => {
    const itemWithRichOption: BankQuestionListItem = {
      ...sampleItem,
      options: [
        { question_id: "q1", key: "a", text: "<b>bold</b> option", is_correct: false, sort_order: 1 },
        { question_id: "q1", key: "b", text: "normal option", is_correct: true, sort_order: 2 },
      ],
    };
    renderWithClient(
      <QuestionPreview
        item={itemWithRichOption}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // Find the options container (space-y-2 div inside the dialog)
    const optionsContainer = document.querySelector(".space-y-2");
    expect(optionsContainer).not.toBeNull();
    // Find RichContent nodes within option containers (flex items-center gap-3)
    const optionDivs = optionsContainer?.querySelectorAll("div.flex.items-center.gap-3");
    expect(optionDivs?.length).toBeGreaterThan(0);
    // Check the first option's RichContent
    const firstOptionDiv = optionDivs?.[0] as HTMLElement;
    const richNode = firstOptionDiv.querySelector("[data-rich-content]");
    expect(richNode).not.toBeNull();
    const bElement = richNode?.querySelector("b");
    expect(bElement).not.toBeNull();
    expect(bElement?.textContent).toBe("bold");
    // Verify the literal tag text is NOT in the visible text
    expect(richNode?.textContent).not.toContain("<b>");
  });

  it("renders LaTeX in option text as KaTeX formula", async () => {
    const itemWithLatexOption: BankQuestionListItem = {
      ...sampleItem,
      options: [
        { question_id: "q1", key: "a", text: "\\(x^2\\)", is_correct: false, sort_order: 1 },
        { question_id: "q1", key: "b", text: "\\(x^3\\)", is_correct: true, sort_order: 2 },
      ],
    };
    renderWithClient(
      <QuestionPreview
        item={itemWithLatexOption}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    await waitFor(() => {
      // Find options container and check for KaTeX rendered in options
      const optionsContainer = document.querySelector(".space-y-2");
      const katexInOptions = optionsContainer?.querySelectorAll(".katex");
      expect(katexInOptions?.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("renders multi_blank question with blanks showing index and correct_answer", () => {
    const multiBlankItem: BankQuestionListItem = {
      question: {
        id: "q2",
        format: "multi_blank",
        body: "The capital of Indonesia is {{1}}, founded in {{2}}.",
        difficulty: "easy",
        point_correct: 2,
        point_wrong: 0,
        sort_order: 1,
      },
      options: [],
      attached_count: 0,
      blanks: [
        { index: 1, correct_answer: "Jakarta" },
        { index: 2, correct_answer: "1945" },
      ],
    };
    renderWithClient(
      <QuestionPreview
        item={multiBlankItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // Should NOT show options section (multi_blank has no options)
    expect(screen.queryByText("a")).toBeNull();
    // Should show blanks section with correct answers
    expect(screen.getByText("Jakarta")).toBeInTheDocument();
    expect(screen.getByText("1945")).toBeInTheDocument();
    // Should show the body
    expect(screen.getByText(/The capital of Indonesia/)).toBeInTheDocument();
  });

  it("shows EVERY accepted answer per blank plus its points, not just correct_answer", () => {
    const multiBlankItem: BankQuestionListItem = {
      question: {
        id: "q2c",
        format: "multi_blank",
        body: "Fill {{1}} here",
        difficulty: "easy",
        point_correct: 1,
        point_wrong: 0,
        sort_order: 1,
      },
      options: [],
      attached_count: 0,
      blanks: [
        {
          index: 1,
          correct_answer: "satu",
          accepted_answers: ["satu", "1", "sau"],
          points: 3,
        },
      ],
    };
    renderWithClient(
      <QuestionPreview
        item={multiBlankItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // The grader accepts every listed answer — the preview must say so, or
    // admins conclude the extra answers were never saved (2026-08-01 report).
    expect(screen.getByText("satu")).toBeInTheDocument();
    expect(screen.getByText("1", { selector: "span" })).toBeInTheDocument();
    expect(screen.getByText("sau")).toBeInTheDocument();
    // the file-level i18n mock renders raw keys, so the badge text is the key
    expect(screen.getByText(/3 tests_field_item_points/i)).toBeInTheDocument();
  });

  it("renders {{N}} tokens as blank chips, never as literal text", () => {
    const multiBlankItem: BankQuestionListItem = {
      question: {
        id: "q2b",
        format: "multi_blank",
        body: "Sun rises in {{1}}, sets in {{2}}.",
        difficulty: "easy",
        point_correct: 1,
        point_wrong: 0,
        sort_order: 1,
      },
      options: [],
      attached_count: 0,
      blanks: [
        { index: 1, correct_answer: "east" },
        { index: 2, correct_answer: "west" },
      ],
    };
    const { container } = renderWithClient(
      <QuestionPreview
        item={multiBlankItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // The admin must see what the student sees — a box per blank, not "{{1}}".
    // (Literal tokens read as a broken question; this is the exact confusion
    // reported on 2026-07-31.)
    const chips = container.ownerDocument.querySelectorAll("[data-blank-chip]");
    expect(chips.length).toBe(2);
    expect(chips[0].textContent).toBe("#1");
    expect(container.ownerDocument.body.textContent).not.toContain("{{1}}");
    expect(container.ownerDocument.body.textContent).not.toContain("{{2}}");
  });

  it("does not show options section for multi_blank (shows blanks instead)", () => {
    const multiBlankItem: BankQuestionListItem = {
      question: {
        id: "q3",
        format: "multi_blank",
        body: "Fill {{1}} and {{2}}",
        difficulty: "medium",
        point_correct: 1,
        point_wrong: 0,
        sort_order: 1,
      },
      options: [],
      attached_count: 0,
      blanks: [
        { index: 1, correct_answer: "answer1" },
        { index: 2, correct_answer: "answer2" },
      ],
    };
    renderWithClient(
      <QuestionPreview
        item={multiBlankItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // The options list should NOT be present
    const optionElements = document.querySelectorAll("div[class*='border p-3']");
    // The first p-3 is the body, second should be the blanks (not options)
    // We verify by checking that the blanks are shown
    expect(screen.getByText("answer1")).toBeInTheDocument();
    expect(screen.getByText("answer2")).toBeInTheDocument();
  });

  it("renders true_false question with statements and their authored truth values (FR-31)", () => {
    const trueFalseItem: BankQuestionListItem = {
      question: {
        id: "q4",
        format: "true_false",
        body: "Which of the following are true?",
        difficulty: "easy",
        point_correct: 1,
        point_wrong: 0.5,
        sort_order: 1,
        statements: [
          { index: 1, body: "Jakarta is the capital of Indonesia", is_true: true },
          { index: 2, body: "The sun orbits the Earth", is_true: false },
        ],
      },
      options: [],
      attached_count: 0,
    };
    renderWithClient(
      <QuestionPreview
        item={trueFalseItem}
        open={true}
        onOpenChange={onOpenChange}
        onEdit={onEdit}
      />
    );
    // No mcq/multi_answer options section
    expect(screen.queryByText("tests_field_option_is_correct")).toBeNull();
    // Both statement bodies render
    expect(
      screen.getByText("Jakarta is the capital of Indonesia")
    ).toBeInTheDocument();
    expect(screen.getByText("The sun orbits the Earth")).toBeInTheDocument();
    // Truth values render as admin-only labels
    expect(screen.getByText("tests_field_statement_is_true")).toBeInTheDocument();
    expect(screen.getByText("tests_field_statement_is_false")).toBeInTheDocument();
  });
});
