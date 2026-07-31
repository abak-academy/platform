import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { QuestionEditor } from "./QuestionEditor";
import { ApiError } from "@/lib/api";
import type { QuestionWithOptions, QuestionFormat, Question, QuestionOption, ExamTopic } from "@/lib/types";

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

const mockTestSaveAsync = vi.fn();
let testSaveState = { mutateAsync: mockTestSaveAsync, isPending: false };

const mockCreateBankAsync = vi.fn();
let createBankState = { mutateAsync: mockCreateBankAsync, isPending: false };

const mockUpdateBankAsync = vi.fn();
let updateBankState = { mutateAsync: mockUpdateBankAsync, isPending: false };

const mockTopics: ExamTopic[] = [
  { id: "topic-1", name: "Aljabar", subject: "Matematika" },
  { id: "topic-2", name: "Fisika Dasar", subject: "Fisika" },
];

vi.mock("@/lib/hooks/admin-tests", () => ({
  useSaveQuestion: () => testSaveState,
}));

vi.mock("@/lib/hooks/admin-bank-questions", () => ({
  useCreateBankQuestion: () => createBankState,
  useUpdateBankQuestion: () => updateBankState,
}));

vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: () => ({ data: { data: mockTopics }, isLoading: false }),
}));

const mockPresignAudioAsync = vi.fn();
let presignAudioState = { mutateAsync: mockPresignAudioAsync, isPending: false };

vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminAudioUpload: () => presignAudioState,
  usePresignAdminImageUpload: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
}));

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

function makeQuestion(overrides: Partial<Question> = {}): Question {
  return {
    id: "q1",
    format: "mcq" as QuestionFormat,
    body: "Apa ibu kota Indonesia?",
    sort_order: 1,
    point_correct: 1,
    point_wrong: 0,
    topic_id: "topic-1",
    topic: "Aljabar",
    ...overrides,
  };
}

function makeOption(overrides: Partial<QuestionOption> = {}): QuestionOption {
  return {
    question_id: "q1",
    key: "a",
    text: "Jakarta",
    is_correct: true,
    sort_order: 1,
    ...overrides,
  };
}

function makeQuestionWithOptions(
  q?: Partial<Question>,
  opts?: QuestionOption[]
): QuestionWithOptions {
  return {
    question: makeQuestion(q),
    options: opts ?? [
      makeOption({ key: "a", text: "Jakarta", is_correct: true, sort_order: 1 }),
      makeOption({ question_id: "q1", key: "b", text: "Bandung", is_correct: false, sort_order: 2 }),
    ],
  };
}

async function setBodyValue(text: string) {
  // Body field is TipTap's ProseMirror-managed contentEditable div (role
  // "textbox") with no `.value`. Mutating its innerHTML directly and firing
  // `input` still works — ProseMirror's own DOM observer reconciles an
  // external DOM mutation back into its model, same as it would for a
  // browser extension or spellcheck correction — but only if the mutated
  // DOM is schema-valid. A bare text node with no block wrapper isn't (the
  // doc requires block-level content at the top level), so it gets silently
  // discarded; wrapping in <p> gives the observer something it can actually
  // parse. Every caller here passes plain text, so this is a safe, direct
  // substitution for typing it. The reconciliation is async (a
  // MutationObserver callback, not synchronous with fireEvent.input), so
  // callers must await this before relying on the editor's React state.
  const body = screen.getByLabelText(/badan soal/i);
  body.innerHTML = `<p>${text}</p>`;
  fireEvent.input(body, { bubbles: true });
  await waitFor(() => expect(body.textContent).toBe(text));
}

async function fillRequiredFields() {
  await setBodyValue("Soal");
  fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });
}

describe("QuestionEditor", () => {
  beforeEach(() => {
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();

    mockTestSaveAsync.mockReset();
    mockTestSaveAsync.mockResolvedValue({ question: makeQuestion(), options: [] });
    testSaveState = { mutateAsync: mockTestSaveAsync, isPending: false };

    mockCreateBankAsync.mockReset();
    mockCreateBankAsync.mockResolvedValue({ question: makeQuestion(), options: [] });
    createBankState = { mutateAsync: mockCreateBankAsync, isPending: false };

    mockUpdateBankAsync.mockReset();
    mockUpdateBankAsync.mockResolvedValue({ question: makeQuestion(), options: [] });
    updateBankState = { mutateAsync: mockUpdateBankAsync, isPending: false };

    mockPresignAudioAsync.mockReset();
    mockPresignAudioAsync.mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "questions/uuid/audio.mp3",
    });
    presignAudioState = { mutateAsync: mockPresignAudioAsync, isPending: false };
  });

  it("renders create mode with format defaulting to mcq", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByLabelText(/badan soal/i).textContent).toBe("");
    const radios = screen.getAllByRole("radio");
    expect(radios.length).toBe(2);
  });

  it("renders edit mode with existing mcq options prefilled and correct radio set", () => {
    const qwo = makeQuestionWithOptions();
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByLabelText(/badan soal/i).textContent).toContain("Apa ibu kota Indonesia?");
    // Option text now uses RichTextEditor (contentEditable), check for text content
    const optionTextElements = screen.getAllByLabelText(/teks opsi/i);
    expect(optionTextElements.some((el) => el.textContent?.includes("Jakarta"))).toBe(true);
    expect(optionTextElements.some((el) => el.textContent?.includes("Bandung"))).toBe(true);

    const radios = screen.getAllByRole("radio");
    const checked = radios.filter((r) => (r as HTMLInputElement).checked);
    expect(checked.length).toBe(1);
  });

  it("edit mode does not crash when an optionless question has null options", () => {
    // The bank list API returns options: null for fill_blank / short / essay
    // (a nil Go slice). The editor must tolerate it, not read null.length.
    const qwo = {
      question: makeQuestion({ format: "fill_blank", correct_answer: "Jakarta" }),
      options: null as unknown as QuestionOption[],
    };
    expect(() =>
      renderWithClient(
        <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
      )
    ).not.toThrow();

    expect(screen.getByLabelText(/jawaban yang diterima/i)).toBeInTheDocument();
    expect(screen.queryAllByRole("radio").length).toBe(0);
  });

  it("switching format to essay hides option editor and accepted-answer input", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getAllByRole("radio").length).toBeGreaterThan(0);

    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "essay" } });

    expect(screen.queryAllByRole("radio").length).toBe(0);
    expect(screen.queryByLabelText(/jawaban yang diterima/i)).not.toBeInTheDocument();
  });

  it("switching format to short shows accepted-answer input and hides option editor", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "short" } });

    expect(screen.getByLabelText(/jawaban yang diterima/i)).toBeInTheDocument();
    expect(screen.queryAllByRole("radio").length).toBe(0);
  });

  it("switching format to fill_blank shows accepted-answer input and hides option editor", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "fill_blank" } });

    expect(screen.getByLabelText(/jawaban yang diterima/i)).toBeInTheDocument();
    expect(screen.queryAllByRole("radio").length).toBe(0);
  });

  it("switching format to multi_answer shows checkboxes instead of radios", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "multi_answer" } });

    expect(screen.queryAllByRole("radio").length).toBe(0);
    expect(screen.getAllByRole("checkbox").length).toBeGreaterThan(0);
  });

  it("submit calls save mutation with correct input shape (mcq)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await setBodyValue("Soal baru");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            format: "mcq",
            // TipTap always wraps content in a block-level node — bare text
            // becomes <p>text</p>, never bare text, since the schema
            // requires block content at the doc's top level.
            body: "<p>Soal baru</p>",
            topic_id: "topic-1",
            options: expect.any(Array),
          }),
        })
      );
    });
  });

  it("does not render a sort-order input — ordering is managed by the dedicated reorder endpoint", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );
    expect(screen.queryByLabelText(/urutan/i)).not.toBeInTheDocument();
  });

  it("mcq submit with default 1-correct option passes validation", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalled();
    });
  });

  it("mcq submit with all options moved to a different one still passes (1 correct)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();

    const radios = screen.getAllByRole("radio");
    fireEvent.change(radios[1], { target: { checked: true } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalled();
    });
  });

  it("multi_answer validation: 0 correct blocks submit", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "multi_answer" } });

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[0]);

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/minimal satu opsi benar/i)
      ).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("multi_answer validation: 1 correct allowed", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "multi_answer" } });

    const checkboxes = screen.getAllByRole("checkbox");
    fireEvent.click(checkboxes[1]);

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalled();
    });
  });

  it("short validation: empty accepted answer blocks submit", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "short" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/minimal satu jawaban yang diterima wajib diisi/i)
      ).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("empty body blocks submit with validation error", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(
        screen.getByText(/badan soal wajib diisi/i)
      ).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("edit mode includes question id in save payload", async () => {
    const qwo = makeQuestionWithOptions();
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({ question: "q1" })
      );
    });
  });

  // ── Penilaian panel (FR-S5-03, FR-S5-29) ─────────────────────────────────

  it("renders the Penilaian panel with correct min/step attributes", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByText(/^penilaian$/i)).toBeInTheDocument();

    const pointCorrect = screen.getByLabelText(/poin benar/i);
    expect(pointCorrect).toHaveAttribute("min", "1");
    expect(pointCorrect).toHaveAttribute("step", "1");
    expect(pointCorrect).toHaveValue(1);

    const pointWrong = screen.getByLabelText(/poin salah/i);
    expect(pointWrong).toHaveAttribute("min", "0");
    expect(pointWrong).toHaveAttribute("step", "1");
    expect(pointWrong).toHaveValue(0);
  });

  it("save payload carries both point_correct and point_wrong", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.input(screen.getByLabelText(/poin benar/i), { target: { value: "4" } });
    fireEvent.input(screen.getByLabelText(/poin salah/i), { target: { value: "2" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({ point_correct: 4, point_wrong: 2 }),
        })
      );
    });
  });

  it("edit mode initializes points from question.point_correct/point_wrong, not the difficulty default", () => {
    const qwo = makeQuestionWithOptions({ difficulty: "easy", point_correct: 7, point_wrong: 3 });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByLabelText(/poin benar/i)).toHaveValue(7);
    expect(screen.getByLabelText(/poin salah/i)).toHaveValue(3);
  });

  it("fractional point_correct (2.5) round-trips into the save payload — not 2, not 1 (FR-16)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.input(screen.getByLabelText(/poin benar/i), { target: { value: "2.5" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({ point_correct: 2.5 }),
        })
      );
    });
  });

  it("point_correct of 0 is blocked client-side with the > 0 validation message (FR-17)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.input(screen.getByLabelText(/poin benar/i), { target: { value: "0" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(screen.getByText(/lebih besar dari 0/i)).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  // ── Topic select (FR-34..FR-36) ─────────────────────────────────────────

  it("renders topic select populated from useTopics", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const topicSelect = screen.getByLabelText(/topik/i);
    expect(topicSelect).toBeInTheDocument();
    expect(within(topicSelect).getByText("Aljabar")).toBeInTheDocument();
    expect(within(topicSelect).getByText("Fisika Dasar")).toBeInTheDocument();
  });

  it("topic is required and blocks submit when empty", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await setBodyValue("Soal");
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(screen.getByText(/topik wajib dipilih/i)).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("bank standalone create uses useCreateBankQuestion", async () => {
    renderWithClient(
      <QuestionEditor onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockCreateBankAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          format: "mcq",
          body: "<p>Soal</p>",
          topic_id: "topic-1",
          options: expect.any(Array),
        })
      );
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
    expect(mockUpdateBankAsync).not.toHaveBeenCalled();
  });

  it("bank standalone edit uses useUpdateBankQuestion", async () => {
    const qwo = makeQuestionWithOptions();
    renderWithClient(
      <QuestionEditor question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockUpdateBankAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          format: "mcq",
          body: "Apa ibu kota Indonesia?",
          topic_id: "topic-1",
        })
      );
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
    expect(mockCreateBankAsync).not.toHaveBeenCalled();
  });

  it("bank standalone create omits sort_order in payload", async () => {
    renderWithClient(
      <QuestionEditor onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockCreateBankAsync).toHaveBeenCalledWith(
        expect.not.objectContaining({ sort_order: expect.any(Number) })
      );
    });
  });

  it("test scoped new question hits create-and-attach via useSaveQuestion", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          question: undefined,
          input: expect.objectContaining({ topic_id: "topic-1" }),
        })
      );
    });
  });

  // ── Multi-blank format (Task 7) ─────────────────────────────────────────

  it("switching format to multi_blank shows blank editor instead of option editor", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getAllByRole("radio").length).toBeGreaterThan(0);

    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "multi_blank" } });

    // Should hide option editor (radios/checkboxes)
    expect(screen.queryAllByRole("radio").length).toBe(0);
    expect(screen.queryAllByRole("checkbox").length).toBe(0);
  });

  it("multi_blank question can be created with 2 blanks and saves with blanks array", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // Set format to multi_blank
    const formatSelect = screen.getByLabelText(/format/i);
    fireEvent.change(formatSelect, { target: { value: "multi_blank" } });

    // Fill required fields
    await setBodyValue("Ibu kota Indonesia adalah {{1}}, didirikan tahun {{2}}.");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    // Fill in the blank accepted answers
    const blankInputs = screen.getAllByLabelText(/jawaban yang diterima/i);
    fireEvent.change(blankInputs[0], { target: { value: "Jakarta" } });
    fireEvent.change(blankInputs[1], { target: { value: "1945" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            format: "multi_blank",
            body: "<p>Ibu kota Indonesia adalah {{1}}, didirikan tahun {{2}}.</p>",
            topic_id: "topic-1",
            blanks: expect.arrayContaining([
              expect.objectContaining({ index: 1, correct_answer: "Jakarta", accepted_answers: ["Jakarta"] }),
              expect.objectContaining({ index: 2, correct_answer: "1945", accepted_answers: ["1945"] }),
            ]),
          }),
        })
      );
    });
  });

  // ── Insert-blank token lockstep (Task 14, FB-25) ────────────────────────

  function makeMultiBlankQuestion(
    blanks: Array<{ index: number; correct_answer: string }>,
    body = "Isi soal: "
  ): QuestionWithOptions {
    return {
      question: makeQuestion({ format: "multi_blank" as QuestionFormat, body }),
      options: [],
      blanks,
    };
  }

  function mockInsertTextExecCommand() {
    return vi.spyOn(document, "execCommand").mockImplementation((cmd, _ui, arg) => {
      if (cmd === "insertText" && typeof arg === "string") {
        const editable = document.querySelector('[contenteditable="true"]');
        editable?.appendChild(document.createTextNode(arg));
      }
      return true;
    });
  }

  it("clicking Tambah opsi on an empty blank list writes {{1}} into the body and appends one row; a second click writes {{2}}", () => {
    const execSpy = mockInsertTextExecCommand();
    const qwo = makeMultiBlankQuestion([]);
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.queryAllByLabelText(/jawaban yang diterima/i).length).toBe(0);

    fireEvent.click(screen.getByRole("button", { name: /tambah opsi/i }));

    expect(screen.getByLabelText(/badan soal/i).textContent).toContain("{{1}}");
    expect(screen.getAllByLabelText(/jawaban yang diterima/i).length).toBe(1);

    fireEvent.click(screen.getByRole("button", { name: /tambah opsi/i }));

    expect(screen.getByLabelText(/badan soal/i).textContent).toContain("{{2}}");
    expect(screen.getAllByLabelText(/jawaban yang diterima/i).length).toBe(2);

    execSpy.mockRestore();
  });

  it("removing a blank row keeps the remaining {{N}} tokens in the body contiguous", () => {
    const execSpy = mockInsertTextExecCommand();
    const qwo = makeMultiBlankQuestion([], "");
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const addButton = screen.getByRole("button", { name: /tambah opsi/i });
    fireEvent.click(addButton); // {{1}}
    fireEvent.click(addButton); // {{2}}
    fireEvent.click(addButton); // {{3}}

    expect(screen.getByLabelText(/badan soal/i).textContent).toBe("{{1}}{{2}}{{3}}");

    // Remove the middle row ({{2}}) — {{3}} must renumber down to {{2}}.
    const removeButtons = screen.getAllByRole("button", { name: /hapus opsi/i });
    fireEvent.click(removeButtons[1]);

    const bodyText = screen.getByLabelText(/badan soal/i).textContent;
    expect(bodyText).toContain("{{1}}");
    expect(bodyText).toContain("{{2}}");
    expect(bodyText).not.toContain("{{3}}");
    expect(screen.getAllByLabelText(/jawaban yang diterima/i).length).toBe(2);

    execSpy.mockRestore();
  });

  it("multi_blank validation: a body with {{1}} and {{3}} (a gap) is rejected client-side", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "multi_blank" } });
    await setBodyValue("Soal {{1}} dan {{3}}");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });
    // Fill both default blank rows so the (pre-existing) empty-answer check
    // cannot be the thing rejecting this submission — only the token gap can.
    const blankInputs = screen.getAllByLabelText(/jawaban yang diterima/i);
    fireEvent.change(blankInputs[0], { target: { value: "A" } });
    fireEvent.change(blankInputs[1], { target: { value: "B" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("multi_blank validation: two blank rows but only one {{N}} token in the body is rejected client-side", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "multi_blank" } });
    // Default multi_blank seeds 2 blank rows; the body carries only one token.
    await setBodyValue("Soal {{1}}");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });
    const blankInputs = screen.getAllByLabelText(/jawaban yang diterima/i);
    fireEvent.change(blankInputs[0], { target: { value: "A" } });
    fireEvent.change(blankInputs[1], { target: { value: "B" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  // ── true_false statement authoring (Task 11, FR-29/FR-30) ──────────────

  it("selecting true_false shows the statement editor and hides options, correct-answer and blanks editors", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getAllByRole("radio").length).toBeGreaterThan(0);

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "true_false" } });

    expect(screen.queryAllByRole("radio").length).toBe(0);
    expect(screen.queryAllByLabelText(/teks opsi/i).length).toBe(0);
    expect(screen.queryByLabelText(/jawaban yang diterima/i)).not.toBeInTheDocument();
    expect(screen.getAllByLabelText(/isi pernyataan/i).length).toBe(2);
  });

  it("adding statements to 4, marking two true, saves exactly 4 statements with indices 1..4 (FR-29)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "true_false" } });
    await setBodyValue("Soal benar/salah");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    const addButton = screen.getByRole("button", { name: /tambah pernyataan/i });
    fireEvent.click(addButton); // 3 rows
    fireEvent.click(addButton); // 4 rows

    const bodies = screen.getAllByLabelText(/isi pernyataan/i);
    expect(bodies.length).toBe(4);
    fireEvent.change(bodies[0], { target: { value: "Statement 1" } });
    fireEvent.change(bodies[1], { target: { value: "Statement 2" } });
    fireEvent.change(bodies[2], { target: { value: "Statement 3" } });
    fireEvent.change(bodies[3], { target: { value: "Statement 4" } });

    const trueToggles = screen.getAllByLabelText(/^benar$/i);
    fireEvent.click(trueToggles[0]);
    fireEvent.click(trueToggles[2]);

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            format: "true_false",
            statements: [
              { index: 1, body: "Statement 1", is_true: true },
              { index: 2, body: "Statement 2", is_true: false },
              { index: 3, body: "Statement 3", is_true: true },
              { index: 4, body: "Statement 4", is_true: false },
            ],
          }),
        })
      );
    });
  });

  it("removing statement rows down to 1 blocks save with the minimum-2 message (FR-30)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "true_false" } });
    await setBodyValue("Soal benar/salah");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    const bodies = screen.getAllByLabelText(/isi pernyataan/i);
    fireEvent.change(bodies[0], { target: { value: "Statement 1" } });
    fireEvent.change(bodies[1], { target: { value: "Statement 2" } });

    fireEvent.click(screen.getAllByRole("button", { name: /hapus pernyataan/i })[1]);
    expect(screen.getAllByLabelText(/isi pernyataan/i).length).toBe(1);

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/minimal 2 pernyataan/i);
    });
    expect(mockTestSaveAsync).not.toHaveBeenCalled();
  });

  it("removing the middle statement of 3 renumbers the remaining rows to 1,2 in the save payload", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "true_false" } });
    await setBodyValue("Soal benar/salah");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    fireEvent.click(screen.getByRole("button", { name: /tambah pernyataan/i })); // 3 rows

    const bodies = screen.getAllByLabelText(/isi pernyataan/i);
    fireEvent.change(bodies[0], { target: { value: "Statement A" } });
    fireEvent.change(bodies[1], { target: { value: "Statement B" } });
    fireEvent.change(bodies[2], { target: { value: "Statement C" } });

    // Remove the middle row (originally index 2) — the last row (index 3)
    // must renumber down to 2, leaving no gap.
    fireEvent.click(screen.getAllByRole("button", { name: /hapus pernyataan/i })[1]);

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            statements: [
              { index: 1, body: "Statement A", is_true: false },
              { index: 2, body: "Statement C", is_true: false },
            ],
          }),
        })
      );
    });
  });

  // ── Rich-text option authoring (Task 7, FR-11) ─────────────────────────

  it("mcq option text field is present in render (before rich-text swap)", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // Default format is mcq, should see option text fields (multiple inputs)
    expect(screen.getAllByLabelText(/teks opsi/i).length).toBeGreaterThan(0);
  });

  // ── Per-question audio URL (Task 7, FR-25) ─────────────────────────────

  it("audio_url field is present for every format", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // For mcq (default)
    expect(screen.getByLabelText(/url audio/i)).toBeInTheDocument();
  });

  it("audio_url field is preserved and round-trips in save payload", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    const audioInput = screen.getByLabelText(/url audio/i);
    fireEvent.change(audioInput, { target: { value: "https://example.com/audio.mp3" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            audio_url: "https://example.com/audio.mp3",
          }),
        })
      );
    });
  });

  it("audio_url field is empty/omitted when not filled", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    // Don't fill audio_url

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.not.objectContaining({
            audio_url: expect.any(String),
          }),
        })
      );
    });
  });

  it("edit mode pre-fills audio_url if question has it", () => {
    const qwo = makeQuestionWithOptions({
      audio_url: "https://example.com/existing-audio.mp3",
    });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    const audioInput = screen.getByLabelText(/url audio/i) as HTMLInputElement;
    expect(audioInput.value).toBe("https://example.com/existing-audio.mp3");
  });

  it("audio_url field uses AudioUploadInput with upload capability", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // AudioUploadInput renders both a text input and an upload button
    const audioInput = screen.getByLabelText(/url audio/i) as HTMLInputElement;
    expect(audioInput).toBeInTheDocument();

    const uploadButton = screen.getByRole("button", { name: /upload audio/i });
    expect(uploadButton).toBeInTheDocument();
  });

  it("selecting an audio file triggers presign and upload flow", async () => {
    const onChange = vi.fn();
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={onChange} />
    );

    await fillRequiredFields();

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);

    // Find the hidden file input for audio upload
    const fileInput = document.querySelector('input[data-testid="audio-upload-input-question-audio-url"]') as HTMLInputElement;
    expect(fileInput).toBeInTheDocument();

    const file = new File(["audio data"], "test.mp3", { type: "audio/mpeg" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    // Wait for presign call
    await waitFor(() => {
      expect(mockPresignAudioAsync).toHaveBeenCalledWith({
        filename: "test.mp3",
        content_type: "audio/mpeg",
      });
    });

    // Verify fetch was called with PUT
    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith(
        "https://upload.example.com/put-here",
        expect.objectContaining({
          method: "PUT",
          body: file,
        })
      );
    });

    // Verify the audio_url field was populated with the result
    await waitFor(() => {
      const audioInput = screen.getByLabelText(/url audio/i) as HTMLInputElement;
      expect(audioInput.value).toContain("audio.mp3");
    });

    vi.unstubAllGlobals();
  });

  it("pre-existing audio_url with AudioUploadInput still loads correctly", () => {
    const qwo = makeQuestionWithOptions({
      audio_url: "https://example.com/existing-audio.mp3",
    });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // AudioUploadInput should display the pre-existing URL
    const audioInput = screen.getByDisplayValue("https://example.com/existing-audio.mp3");
    expect(audioInput).toBeInTheDocument();
  });

  it("pre-existing audio_url saves correctly without forced re-upload", async () => {
    const qwo = makeQuestionWithOptions({
      audio_url: "https://example.com/existing-audio.mp3",
    });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    // Don't upload a new file, just save
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            audio_url: "https://example.com/existing-audio.mp3",
          }),
        })
      );
    });
  });

  // ── Accepted-answer sets (Task 10, FR-22, FR-24, FR-26) ─────────────────

  it("short question: adding two accepted answers saves accepted_answers: [\"2\",\"dua\"]", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "short" } });
    await fillRequiredFields();

    const firstAnswer = screen.getByLabelText(/jawaban yang diterima/i);
    fireEvent.change(firstAnswer, { target: { value: "2" } });

    fireEvent.click(screen.getByRole("button", { name: /tambah jawaban/i }));
    const answers = screen.getAllByLabelText(/jawaban yang diterima/i);
    expect(answers.length).toBe(2);
    fireEvent.change(answers[1], { target: { value: "dua" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            accepted_answers: ["2", "dua"],
          }),
        })
      );
    });
  });

  it("short question: removing accepted-answer rows down to zero is prevented — the last row's remove control is disabled", () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "short" } });

    expect(screen.getAllByLabelText(/jawaban yang diterima/i).length).toBe(1);
    const removeButton = screen.getByRole("button", { name: /hapus jawaban/i });
    expect(removeButton).toBeDisabled();

    fireEvent.click(removeButton);
    expect(screen.getAllByLabelText(/jawaban yang diterima/i).length).toBe(1);
  });

  it("multi_blank: adding a second accepted answer to blank 2 lands under blanks[1].accepted_answers (FR-24)", async () => {
    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    fireEvent.change(screen.getByLabelText(/format/i), { target: { value: "multi_blank" } });
    await setBodyValue("Soal {{1}} dan {{2}}");
    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "topic-1" } });

    const answerInputs = screen.getAllByLabelText(/jawaban yang diterima/i);
    fireEvent.change(answerInputs[0], { target: { value: "4" } });
    fireEvent.change(answerInputs[1], { target: { value: "empat" } });

    const addAnswerButtons = screen.getAllByRole("button", { name: /tambah jawaban/i });
    fireEvent.click(addAnswerButtons[1]);
    const updatedInputs = screen.getAllByLabelText(/jawaban yang diterima/i);
    // blank 1 still has 1 input, blank 2 now has 2
    expect(updatedInputs.length).toBe(3);
    fireEvent.change(updatedInputs[2], { target: { value: "four" } });

    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(mockTestSaveAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          input: expect.objectContaining({
            blanks: [
              expect.objectContaining({ index: 1, accepted_answers: ["4"] }),
              expect.objectContaining({ index: 2, accepted_answers: ["empat", "four"] }),
            ],
          }),
        })
      );
    });
  });

  // ── Format lock (Task 10, FR-14, FR-15) ─────────────────────────────────

  it("format select is disabled and shows the locked reason when in_live_exam is true", () => {
    const qwo = makeQuestionWithOptions({ in_live_exam: true });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByLabelText(/format/i)).toBeDisabled();
    expect(
      screen.getByText(/format tidak dapat diubah karena soal ini digunakan dalam ujian aktif/i)
    ).toBeInTheDocument();
  });

  it("format select is enabled and shows no locked reason when in_live_exam is false", () => {
    const qwo = makeQuestionWithOptions({ in_live_exam: false });
    renderWithClient(
      <QuestionEditor testId="test-1" question={qwo} onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    expect(screen.getByLabelText(/format/i)).not.toBeDisabled();
    expect(
      screen.queryByText(/format tidak dapat diubah karena soal ini digunakan dalam ujian aktif/i)
    ).not.toBeInTheDocument();
  });

  it("a 409 question_format_locked from the server surfaces the locked reason, not a generic error toast", async () => {
    mockTestSaveAsync.mockRejectedValueOnce(
      new ApiError("question_format_locked", "format is locked", 409)
    );

    renderWithClient(
      <QuestionEditor testId="test-1" onCancel={vi.fn()} onSaved={vi.fn()} />
    );

    await fillRequiredFields();
    fireEvent.click(screen.getByRole("button", { name: /simpan soal/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(
        "Format soal tidak dapat diubah karena soal ini digunakan pada ujian yang aktif."
      );
    });
  });
});
