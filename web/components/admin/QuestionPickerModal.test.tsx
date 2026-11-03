import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
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

const refetch = vi.fn();

let bankState = {
  data: { data: bankQuestions, next_cursor: "" } as { data: BankQuestionListItem[]; next_cursor: string },
  isLoading: false,
  isError: false,
  refetch,
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
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  beforeEach(() => {
    bankState = {
      data: { data: bankQuestions, next_cursor: "" },
      isLoading: false,
      isError: false,
      refetch,
    };
    refetch.mockReset();
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
  });

  function bankItem(id: string, body: string): BankQuestionListItem {
    return {
      question: { id, format: "mcq", body, point_correct: 1, point_wrong: 0, sort_order: 1 },
      options: [],
      attached_count: 0,
    };
  }

  const longBody = "x".repeat(200);

  function mockOverflow(overflowing: boolean) {
    vi.spyOn(HTMLElement.prototype, "scrollHeight", "get").mockReturnValue(overflowing ? 100 : 40);
    vi.spyOn(HTMLElement.prototype, "clientHeight", "get").mockReturnValue(60);
  }

  it("clamps long bodies to three lines and expands inside the row without selecting it", () => {
    mockOverflow(true);
    bankState = { ...bankState, data: { data: [bankItem("long1", longBody)], next_cursor: "" } };
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={vi.fn()} />
    );

    const body = screen.getByText(longBody);
    expect(body.className).toContain("line-clamp-3");
    expect(body.className).toContain("[overflow-wrap:anywhere]");

    fireEvent.click(screen.getByRole("button", { name: "tests_picker_more" }));
    expect(screen.getByText(longBody).className).not.toContain("line-clamp-3");
    expect(screen.getByRole("button", { name: "tests_picker_confirm" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "tests_picker_less" }));
    expect(screen.getByText(longBody).className).toContain("line-clamp-3");
  });

  it("offers no expand toggle when the body fits in three lines, however many characters", () => {
    mockOverflow(false);
    bankState = { ...bankState, data: { data: [bankItem("long2", longBody)], next_cursor: "" } };
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={vi.fn()} />
    );
    expect(screen.queryByRole("button", { name: "tests_picker_more" })).not.toBeInTheDocument();
  });

  it("sends search, topic_id and format together and keeps the selection when filters change", async () => {
    const onAttach = vi.fn().mockResolvedValue(undefined);
    topicsState = { data: { data: [{ id: "topic-9", name: "Geometri", subject: "Matematika" }] }, isLoading: false };
    const { rerender } = renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={onAttach} />
    );
    const qc = new QueryClient();
    const ui = (
      <QueryClientProvider client={qc}>
        <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={onAttach} />
      </QueryClientProvider>
    );

    fireEvent.click(screen.getByText("bold text").closest("button") as HTMLElement);

    const [formatSelect, topicSelect] = screen.getAllByRole("combobox");
    fireEvent.change(formatSelect, { target: { value: "essay" } });
    fireEvent.change(topicSelect, { target: { value: "topic-9" } });
    fireEvent.change(screen.getByPlaceholderText("tests_picker_search"), { target: { value: "segitiga" } });
    await vi.waitFor(() =>
      expect(bankFilterCalls.at(-1)).toMatchObject({ search: "segitiga", topic_id: "topic-9", format: "essay" }),
    );

    bankState = { ...bankState, data: { data: [bankItem("other", "Soal lain")], next_cursor: "" } };
    rerender(ui);
    fireEvent.click(screen.getByText("Soal lain").closest("button") as HTMLElement);
    fireEvent.click(screen.getByRole("button", { name: "tests_picker_confirm" }));

    await vi.waitFor(() => expect(onAttach).toHaveBeenCalledWith(expect.arrayContaining(["bqr1", "other"])));
  });

  it("keeps the selection and stays open when the server rejects the attach", async () => {
    const onAttach = vi.fn().mockRejectedValue(new Error("sibling collision"));
    const onOpenChange = vi.fn();
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={onOpenChange} testId="t" attached={[]} onAttach={onAttach} />
    );

    fireEvent.click(screen.getByText("bold text").closest("button") as HTMLElement);
    fireEvent.click(screen.getByRole("button", { name: "tests_picker_confirm" }));

    await vi.waitFor(() => expect(onAttach).toHaveBeenCalled());
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(await screen.findByRole("button", { name: "tests_picker_confirm" })).toBeEnabled();
  });

  it("shows an error with retry when the bank fails to load", () => {
    bankState = { ...bankState, isError: true, data: undefined as never };
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={vi.fn()} />
    );

    expect(screen.getByText("tests_picker_error")).toBeInTheDocument();
    expect(screen.queryByText("tests_picker_empty")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "retry" }));
    expect(refetch).toHaveBeenCalled();
  });

  it("does not claim the whole bank is loaded when the API has another page", () => {
    bankState = { ...bankState, data: { data: bankQuestions, next_cursor: "more" } };
    renderWithClient(
      <QuestionPickerModal open={true} onOpenChange={vi.fn()} testId="t" attached={[]} onAttach={vi.fn()} />
    );
    expect(screen.getByText("tests_picker_partial")).toBeInTheDocument();
  });

  it("does not let an already attached question be selected", () => {
    renderWithClient(
      <QuestionPickerModal
        open={true}
        onOpenChange={vi.fn()}
        testId="t"
        attached={[{ question: bankQuestions[0].question, options: [] }]}
        onAttach={vi.fn()}
      />
    );
    expect(screen.getByText("bold text").closest("button")).toBeDisabled();
  });
});
