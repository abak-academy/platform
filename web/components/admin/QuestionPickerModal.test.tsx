import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within, fireEvent, act } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { QuestionPickerModal } from "./QuestionPickerModal";
import type { BankQuestionListItem, QuestionWithOptions, ExamTopic } from "@/lib/types";

const bankQuestions: BankQuestionListItem[] = [
  {
    question: {
      id: "bqr1",
      format: "mcq",
      body: "<b>bold</b> text",
      difficulty: "easy",
      point_correct: 1,
      point_wrong: 0,
      sort_order: 1,
      topic_id: "topic-1",
      topic: "Arithmetic",
    },
    options: [],
    attached_count: 0,
  },
];

let bankState = {
  data: { data: bankQuestions, next_cursor: "" } as { data: BankQuestionListItem[]; next_cursor: string },
  isLoading: false,
  isError: false,
};

let topicsState = {
  data: { data: [] as ExamTopic[] },
  isLoading: false,
};

const bankFilterCalls: Array<Record<string, unknown> | undefined> = [];

vi.mock("@/lib/hooks/admin-bank-questions", () => ({
  useBankQuestions: (filters?: Record<string, unknown>) => {
    bankFilterCalls.push(filters);
    return bankState;
  },
}));

vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: () => topicsState,
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("QuestionPickerModal", () => {
  beforeEach(() => {
    bankState = {
      data: { data: bankQuestions, next_cursor: "" },
      isLoading: false,
      isError: false,
    };
    topicsState = {
      data: { data: [] },
      isLoading: false,
    };
    bankFilterCalls.length = 0;
  });

  it("strips HTML tags from question body in the picker row", async () => {
    const onAttach = vi.fn().mockResolvedValue(undefined);
    const onOpenChange = vi.fn();
    const attached: QuestionWithOptions[] = [];

    renderWithClient(
      <QuestionPickerModal
        open={true}
        onOpenChange={onOpenChange}
        testId="test-1"
        attached={attached}
        onAttach={onAttach}
      />
    );

    expect(await screen.findByText("bold text")).toBeInTheDocument();
    expect(screen.queryByText("<b>bold</b> text")).not.toBeInTheDocument();

    const row = screen.getByText("bold text").closest("button") as HTMLElement;
    expect(row).toBeTruthy();
    expect(row.innerHTML).not.toContain("<b>");
  });

  it("debounces the search box before it reaches the bank query", () => {
    vi.useFakeTimers();
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="test-1" attached={[]} onAttach={vi.fn()} />
    );

    const search = screen.getByPlaceholderText("tests_picker_search");
    fireEvent.change(search, { target: { value: "bold" } });

    expect(bankFilterCalls.some((filters) => filters?.search !== undefined)).toBe(false);

    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(bankFilterCalls.at(-1)?.search).toBe("bold");

    vi.useRealTimers();
  });
});
