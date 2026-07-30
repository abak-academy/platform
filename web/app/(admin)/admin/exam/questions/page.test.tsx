import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import QuestionBankPage from "./page";
import type { BankQuestionListItem, BankQuestionListResponse, ExamTopic } from "@/lib/types";

const mockBankData: BankQuestionListResponse = {
  data: [
    {
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
    },
    {
      question: {
        id: "q2",
        format: "essay",
        body: "Explain photosynthesis.",
        difficulty: "hard",
        point_correct: 5,
        point_wrong: 0,
        sort_order: 1,
        topic_id: "topic-2",
        topic: "Biology",
      },
      options: [],
      attached_count: 0,
    },
  ],
  next_cursor: "next-page-cursor",
};

const sampleTopics: ExamTopic[] = [
  { id: "topic-1", name: "Arithmetic", subject: "Math", question_count: 1 },
  { id: "topic-2", name: "Biology", subject: "Science", question_count: 1 },
];

let bankState = {
  data: null as BankQuestionListResponse | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  isFetching: false,
  refetch: vi.fn(),
};

let topicsState = {
  data: null as { data: ExamTopic[]; next_cursor?: string } | null,
  isLoading: false,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

const mockDeleteMutateAsync = vi.fn();

vi.mock("@/lib/hooks/admin-bank-questions", () => ({
  useBankQuestions: () => bankState,
  useCreateBankQuestion: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBankQuestion: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteBankQuestion: () => ({ mutateAsync: mockDeleteMutateAsync, isPending: false }),
  useImportBankQuestions: () => ({ mutateAsync: vi.fn(), isPending: false }),
  adminBankQuestionsKeys: {
    all: ["admin", "bank-questions"],
    lists: () => ["admin", "bank-questions", "list"],
    list: (filters: unknown) => ["admin", "bank-questions", "list", filters ?? {}],
    detail: (id: string) => ["admin", "bank-questions", "detail", id],
  },
}));

vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: () => topicsState,
  useCreateTopic: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteTopic: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/components/admin/QuestionEditor", () => ({
  QuestionEditor: ({ question, onCancel, onSaved }: any) => (
    <div data-testid="question-editor">
      <span data-testid="editor-question-body">{question?.question.body || "new-question"}</span>
      <button onClick={onCancel}>Cancel</button>
      <button onClick={onSaved}>Save-mock</button>
    </div>
  ),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

vi.mock("sonner", () => {
  const success = vi.fn();
  const error = vi.fn();
  const info = vi.fn();
  return {
    toast: Object.assign((...args: unknown[]) => info(...args), {
      success,
      error,
      info,
    }),
  };
});

import { toast } from "sonner";
import { ApiError } from "@/lib/api";

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

describe("QuestionBankPage", () => {
  beforeEach(() => {
    bankState = {
      data: mockBankData,
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };
    topicsState = {
      data: { data: sampleTopics },
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
    (toast.info as ReturnType<typeof vi.fn>).mockReset();
    mockDeleteMutateAsync.mockReset();
  });

  it("renders title, subtitle, toolbar and table with used-in column", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => {
      expect(screen.getByText("question_bank_title")).toBeInTheDocument();
      expect(screen.getByText("question_bank_subtitle")).toBeInTheDocument();
    });

    expect(screen.getByText("What is 2+2?")).toBeInTheDocument();
    expect(screen.getByText("Explain photosynthesis.")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(screen.getByText("0")).toBeInTheDocument();
  });

  it("shows skeleton rows while loading", () => {
    bankState = {
      data: null,
      isLoading: true,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    expect(document.querySelectorAll("[data-slot=skeleton]").length).toBeGreaterThan(0);
  });

  it("surfaces an API error inline", async () => {
    bankState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat bank soal"),
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat bank soal/i)).toBeInTheDocument();
    });
  });

  it("shows empty state when no questions", async () => {
    bankState = {
      data: { data: [], next_cursor: "" },
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => {
      expect(screen.getByText("tests_picker_empty")).toBeInTheDocument();
    });
  });

  it("opens read-only preview on row click", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const row = screen.getByText("What is 2+2?").closest("tr");
    fireEvent.click(row!);

    await waitFor(() => {
      const dialog = screen.getByRole("dialog");
      expect(within(dialog).getByText("What is 2+2?")).toBeInTheDocument();
      expect(within(dialog).getByRole("button", { name: /action_edit/i })).toBeInTheDocument();
    });
  });

  it("opens the editor for a new question on Create click", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /create/i }));

    await waitFor(() => {
      expect(screen.getByTestId("question-editor")).toBeInTheDocument();
      expect(screen.getByTestId("editor-question-body")).toHaveTextContent("new-question");
    });
  });

  it("opens the topics modal on Manage Topics click", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /manage_topics/i }));

    await waitFor(() => {
      const dialog = screen.getByRole("dialog", { name: /manage_topics/i });
      expect(within(dialog).getByText("Arithmetic")).toBeInTheDocument();
      expect(within(dialog).getByLabelText(/topic_name/i)).toBeInTheDocument();
    });
  });

  it("opens the editor from preview with the selected question", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    fireEvent.click(screen.getByText("What is 2+2?").closest("tr")!);

    await waitFor(() =>
      expect(screen.getByRole("dialog")).toBeInTheDocument()
    );

    const dialog = screen.getByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /action_edit/i }));

    await waitFor(() => {
      expect(screen.getByTestId("question-editor")).toBeInTheDocument();
      expect(screen.getByTestId("editor-question-body")).toHaveTextContent("What is 2+2?");
    });
  });

  it("opens the import modal on CSV click", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const csvButton = screen.getByRole("button", { name: /csv/i });
    fireEvent.click(csvButton);

    await waitFor(() => {
      expect(screen.getByRole("dialog", { name: /import_questions_title/i })).toBeInTheDocument();
    });
  });

  it("changes the format filter via dropdown", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const [formatSelect] = screen.getAllByRole("combobox");
    expect(formatSelect).toHaveValue("all");

    fireEvent.change(formatSelect, { target: { value: "mcq" } });

    await waitFor(() => {
      expect(formatSelect).toHaveValue("mcq");
    });
  });

  it("lets the user select a topic filter", async () => {
    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const [, topicSelect] = screen.getAllByRole("combobox");
    fireEvent.change(topicSelect, { target: { value: "topic-2" } });

    await waitFor(() => {
      expect(topicSelect).toHaveValue("topic-2");
    });
  });

  it("strips HTML tags from question body in the bank list row", async () => {
    bankState = {
      data: {
        data: [
          {
            question: {
              id: "q3",
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
        ],
        next_cursor: "",
      },
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => {
      expect(screen.getByText("bold text")).toBeInTheDocument();
    });
    expect(screen.queryByText("<b>bold</b> text")).not.toBeInTheDocument();
    const bodyCell = screen.getByText("bold text").closest("td");
    expect(bodyCell?.innerHTML).not.toContain("<b>");
    expect(bodyCell?.textContent).toBe("bold text");
  });

  it("shows question_number instead of the id prefix", async () => {
    bankState = {
      data: {
        data: [
          {
            question: {
              id: "11111111-2222-3333-4444-555555555555",
              format: "mcq",
              body: "Numbered question",
              point_correct: 1,
              point_wrong: 0,
              sort_order: 1,
              question_number: 42,
            },
            options: [],
            attached_count: 0,
          },
        ],
        next_cursor: "",
      },
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => {
      expect(screen.getByText("42")).toBeInTheDocument();
    });
    expect(screen.queryByText("11111111")).not.toBeInTheDocument();
  });

  it("does not send a delete request until the confirm control is clicked", async () => {
    vi.stubGlobal("confirm", () => false);

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const row = screen.getByText("What is 2+2?").closest("tr") as HTMLElement;
    const deleteButton = within(row).getByRole("button", { name: /action_delete/i });
    fireEvent.click(deleteButton);

    expect(mockDeleteMutateAsync).not.toHaveBeenCalled();

    vi.unstubAllGlobals();
  });

  it("deletes a question after confirm", async () => {
    mockDeleteMutateAsync.mockResolvedValueOnce(undefined);
    vi.stubGlobal("confirm", () => true);

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const row = screen.getByText("What is 2+2?").closest("tr") as HTMLElement;
    const deleteButton = within(row).getByRole("button", { name: /action_delete/i });
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDeleteMutateAsync).toHaveBeenCalledWith("q1");
      expect(toast.success).toHaveBeenCalledWith("questions_deleted");
    });

    vi.unstubAllGlobals();
  });

  it("renders the server's reason on a 409 question_in_published_exam response", async () => {
    mockDeleteMutateAsync.mockRejectedValueOnce(
      new ApiError("question_in_published_exam", "This question is used in a live exam.", 409)
    );
    vi.stubGlobal("confirm", () => true);

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("What is 2+2?")).toBeInTheDocument());

    const row = screen.getByText("What is 2+2?").closest("tr") as HTMLElement;
    const deleteButton = within(row).getByRole("button", { name: /action_delete/i });
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith("question_in_published_exam_reason");
    });
    expect(toast.error).not.toHaveBeenCalledWith("error_generic");

    vi.unstubAllGlobals();
  });

  it("disables the delete control for a row with in_live_exam true", async () => {
    bankState = {
      data: {
        data: [
          {
            question: {
              id: "q4",
              format: "mcq",
              body: "Live exam question",
              point_correct: 1,
              point_wrong: 0,
              sort_order: 1,
              in_live_exam: true,
            },
            options: [],
            attached_count: 0,
          },
        ],
        next_cursor: "",
      },
      isLoading: false,
      isError: false,
      error: null,
      isFetching: false,
      refetch: vi.fn(),
    };

    renderWithClient(<QuestionBankPage />);

    await waitFor(() => expect(screen.getByText("Live exam question")).toBeInTheDocument());

    const row = screen.getByText("Live exam question").closest("tr") as HTMLElement;
    const deleteButton = within(row).getByRole("button", { name: /action_delete/i });

    expect(deleteButton).toBeDisabled();
    expect(deleteButton).toHaveAttribute("title", "question_in_published_exam_reason");
  });

});
