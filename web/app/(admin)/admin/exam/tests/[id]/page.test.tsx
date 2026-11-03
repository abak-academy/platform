import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { toast } from "sonner";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import TestDetailPage from "./page";
import { useParams } from "next/navigation";
import type { TestDetail, QuestionWithOptions, Test, ExamTopic } from "@/lib/types";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useParams: vi.fn(),
  useRouter: () => ({ push }),
}));

const mockMutateAsync = vi.fn();

let testDetailState: {
  data: TestDetail | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = { data: undefined, isLoading: true, isError: false, error: null };

let questionsState: {
  data: { data: QuestionWithOptions[] } | undefined;
  isLoading: boolean;
  isError: boolean;
  error: Error | null;
} = { data: undefined, isLoading: true, isError: false, error: null };

let updateState = { mutateAsync: mockMutateAsync, isPending: false };
let detachState = { mutateAsync: mockMutateAsync, isPending: false };
let reorderState = { mutateAsync: mockMutateAsync, isPending: false };
let attachState = { mutateAsync: mockMutateAsync, isPending: false };
let saveQuestionState = { mutateAsync: mockMutateAsync, isPending: false };

const mockTopics: ExamTopic[] = [
  { id: "topic-1", name: "Aljabar", subject: "Matematika" },
  { id: "topic-2", name: "Geometri", subject: "Matematika" },
];
const mockBankQuestions: { data: QuestionWithOptions[] } = {
  data: [
    {
      question: {
        id: "bq1",
        format: "mcq",
        body: "Bank question one",
        sort_order: 1,
        point_correct: 1,
        point_wrong: 0,
      },
      options: [],
    },
  ],
};

vi.mock("@/lib/hooks/admin-tests", () => ({
  useTestDetail: () => testDetailState,
  useTestQuestions: () => questionsState,
  useUpdateTest: () => updateState,
  useDetachQuestion: () => detachState,
  useReorderTestQuestions: () => reorderState,
  useAttachQuestions: () => attachState,
  useSaveQuestion: () => saveQuestionState,
}));

vi.mock("@/lib/hooks/admin-topics", () => ({
  useTopics: () => ({ data: { data: mockTopics }, isLoading: false }),
}));

vi.mock("@/lib/hooks/admin-bank-questions", () => ({
  useBankQuestions: () => ({ data: mockBankQuestions, isLoading: false, isError: false }),
  useCreateBankQuestion: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateBankQuestion: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/components/admin/RichTextEditor", () => ({
  RichTextEditor: ({
    value,
    onChange,
    "aria-label": label,
  }: {
    value: string;
    onChange: (html: string) => void;
    "aria-label"?: string;
  }) => <textarea aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

async function openTab(name: RegExp) {
  await userEvent.click(await screen.findByRole("tab", { name }));
}

const sampleTest: Test = {
  id: "test-1",
  title: "Tryout UTBK Saintek",
  subject: "Matematika",
  topic: "Aljabar",
  duration_minutes: 90,
};

const sampleQuestions: QuestionWithOptions[] = [
  {
    question: {
      id: "q1",
      format: "mcq",
      body: "Apa ibu kota Indonesia?",
      sort_order: 1,
      point_correct: 1,
      point_wrong: 0,
    },
    options: [
      { question_id: "q1", key: "a", text: "Jakarta", is_correct: true, sort_order: 1 },
      { question_id: "q1", key: "b", text: "Bandung", is_correct: false, sort_order: 2 },
    ],
  },
  {
    question: {
      id: "q2",
      format: "short",
      body: "Sebutkan 1+1",
      sort_order: 2,
      correct_answer: "2",
      point_correct: 1,
      point_wrong: 0,
    },
    options: [],
  },
];

describe("TestDetailPage", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  beforeEach(() => {
    (useParams as ReturnType<typeof vi.fn>).mockReturnValue({ id: "test-1" });
    testDetailState = {
      data: { test: sampleTest, questions: sampleQuestions },
      isLoading: false,
      isError: false,
      error: null,
    };
    questionsState = {
      data: { data: sampleQuestions },
      isLoading: false,
      isError: false,
      error: null,
    };
    updateState = { mutateAsync: mockMutateAsync, isPending: false };
    detachState = { mutateAsync: mockMutateAsync, isPending: false };
    reorderState = { mutateAsync: mockMutateAsync, isPending: false };
    attachState = { mutateAsync: mockMutateAsync, isPending: false };
    saveQuestionState = { mutateAsync: mockMutateAsync, isPending: false };
    mockMutateAsync.mockReset();
    mockMutateAsync.mockResolvedValue(undefined);
    push.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders the test metadata via AdminPageHeader", async () => {
    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: /tryout utbk saintek/i })).toBeInTheDocument();
    });
    expect(screen.getByText("Matematika · Aljabar")).toBeInTheDocument();
  });

  it("shows a back-link to the tests list", async () => {
    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: /tryout utbk saintek/i })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByTestId("tests-back-link"));
    expect(push).toHaveBeenCalledWith("/admin/exam/tests");
  });

  it("separates questions, material/instructions and test information into tabs", async () => {
    renderWithClient(<TestDetailPage />);

    expect(await screen.findByRole("tab", { name: "Soal (2)", selected: true })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /materi & instruksi/i })).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: /informasi tes/i })).toBeInTheDocument();
    expect(screen.getByText("Apa ibu kota Indonesia?")).toBeInTheDocument();
    expect(screen.getByText("Sebutkan 1+1")).toBeInTheDocument();
    expect(screen.queryByLabelText(/judul/i)).not.toBeInTheDocument();

    await openTab(/informasi tes/i);
    expect(screen.getByLabelText(/judul/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/durasi/i)).toBeInTheDocument();
  });

  it("shows the question count, duration and save status in the meta row", async () => {
    renderWithClient(<TestDetailPage />);

    const meta = await screen.findByTestId("test-meta-row");
    expect(within(meta).getByText("2 soal")).toBeInTheDocument();
    expect(within(meta).getByText("90 menit")).toBeInTheDocument();
    expect(within(meta).getByRole("status")).toHaveTextContent("Pengaturan tersimpan");

    await openTab(/informasi tes/i);
    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul baru" } });
    expect(within(meta).getByRole("status")).toHaveTextContent("Ada pengaturan belum disimpan");
  });

  it("shows how many exams use the test when exam_count is positive", async () => {
    testDetailState = {
      ...testDetailState,
      data: { test: { ...sampleTest, exam_count: 3 }, questions: sampleQuestions },
    };
    renderWithClient(<TestDetailPage />);

    expect(within(await screen.findByTestId("test-meta-row")).getByText("Dipakai pada 3 ujian")).toBeInTheDocument();
  });

  it.each([
    ["is 0", 0],
    ["is absent", undefined],
  ])("hides the usage item when exam_count %s", async (_label, examCount) => {
    testDetailState = {
      ...testDetailState,
      data: { test: { ...sampleTest, exam_count: examCount }, questions: sampleQuestions },
    };
    renderWithClient(<TestDetailPage />);

    await screen.findByTestId("test-meta-row");
    expect(screen.queryByText(/dipakai pada/i)).not.toBeInTheDocument();
  });

  it("explains that one test can serve several exams", async () => {
    renderWithClient(<TestDetailPage />);

    expect(await screen.findByText(/satu tes dapat digunakan di beberapa ujian/i)).toBeInTheDocument();
  });

  it("marks questions with their own material and shows the exam-display warning", async () => {
    const withOwn: QuestionWithOptions[] = [
      sampleQuestions[0],
      { ...sampleQuestions[1], question: { ...sampleQuestions[1].question, material_html: "<p>Bacaan soal</p>" } },
    ];
    questionsState = { ...questionsState, data: { data: withOwn } };
    renderWithClient(<TestDetailPage />);

    const first = (await screen.findByText("Apa ibu kota Indonesia?")).closest("[data-question-row]") as HTMLElement;
    const second = screen.getByText("Sebutkan 1+1").closest("[data-question-row]") as HTMLElement;
    expect(within(first).getByText("Materi bersama")).toBeInTheDocument();
    expect(within(first).queryByText("Materi sendiri")).not.toBeInTheDocument();
    expect(within(second).getByText("Materi sendiri")).toBeInTheDocument();
    expect(within(second).queryByText("Materi bersama")).not.toBeInTheDocument();

    const display = screen.getByTestId("test-display-card");
    expect(within(display).getByText("1 soal memiliki materi sendiri")).toBeInTheDocument();
  });

  it("does not warn about per-question material when every question uses the shared one", async () => {
    renderWithClient(<TestDetailPage />);

    const display = await screen.findByTestId("test-display-card");
    expect(within(display).queryByText(/memiliki materi sendiri/i)).not.toBeInTheDocument();
    expect(within(display).getByText(/tidak ada materi per soal/i)).toBeInTheDocument();
  });

  it("Atur materi switches to the Materi & instruksi tab", async () => {
    renderWithClient(<TestDetailPage />);

    const sidebar = await screen.findByTestId("test-material-sidebar");
    fireEvent.click(within(sidebar).getByRole("button", { name: /atur materi/i }));

    expect(await screen.findByRole("tab", { name: /materi & instruksi/i, selected: true })).toBeInTheDocument();
    expect(screen.getByLabelText("Instruksi section")).toBeInTheDocument();
  });

  it("renders the shared material and instructions as two titled cards with an ownership notice", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/materi & instruksi/i);

    expect(screen.getByRole("heading", { name: "Materi bersama" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Instruksi pengerjaan" })).toBeInTheDocument();
    expect(screen.getByText(/materi dan instruksi milik sumber tes/i)).toBeInTheDocument();
    expect(screen.queryByTestId("test-material-sidebar")).not.toBeInTheDocument();
  });

  it("groups test information into identity and audio cards", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    expect(screen.getByRole("heading", { name: "Identitas & durasi" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Audio section" })).toBeInTheDocument();
  });

  it("saves test metadata via useUpdateTest", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toHaveValue("Tryout UTBK Saintek");
    });

    fireEvent.change(screen.getByLabelText(/topik/i), { target: { value: "Geometri" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan pengaturan/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ topic: "Geometri" })
      );
    });
  });

  it("sends explicit null (not an omitted key) for cleared audio/section fields", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toHaveValue("Tryout UTBK Saintek");
    });

    fireEvent.change(screen.getByLabelText(/durasi/i), { target: { value: "91" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan pengaturan/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          audio_url: null,
          audio_play_limit: null,
          section_type: null,
          shared_material_html: null,
          instructions_html: null,
        })
      );
    });
  });

  it("New question button opens an inline QuestionEditor", async () => {
    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Apa ibu kota Indonesia?")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /soal baru/i }));

    const body = screen.getByLabelText(/badan soal/i);
    expect(body).toBeInTheDocument();
    expect(body.textContent).toBe("");
  });

  it("detach asks for confirmation, then removes only the attachment", async () => {
    renderWithClient(<TestDetailPage />);

    const row = (await screen.findByText("Apa ibu kota Indonesia?")).closest("[data-question-row]") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /lepas/i }));

    const dialog = await screen.findByRole("dialog", { name: /lepas soal dari tes/i });
    expect(within(dialog).getByText(/tetap ada di bank soal/i)).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("button", { name: /batal/i }));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
    expect(mockMutateAsync).not.toHaveBeenCalled();

    fireEvent.click(within(row).getByRole("button", { name: /lepas/i }));
    const again = await screen.findByRole("dialog", { name: /lepas soal dari tes/i });
    fireEvent.click(within(again).getByRole("button", { name: /^lepas$/i }));

    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith("q1"));
  });

  it("keeps preview and detach icons side by side with accessible names", async () => {
    renderWithClient(<TestDetailPage />);

    const row = (await screen.findByText("Apa ibu kota Indonesia?")).closest("[data-question-row]") as HTMLElement;
    const preview = within(row).getByRole("button", { name: /pratinjau soal/i });
    const detach = within(row).getByRole("button", { name: /lepas/i });
    expect(preview.parentElement).toBe(detach.parentElement);
    expect(preview).toHaveAttribute("title");
    expect(detach).toHaveAttribute("title");
  });

  it("disables the reorder arrows while a reorder is pending", async () => {
    reorderState = { mutateAsync: mockMutateAsync, isPending: true };
    renderWithClient(<TestDetailPage />);

    const row = (await screen.findByText("Apa ibu kota Indonesia?")).closest("[data-question-row]") as HTMLElement;
    expect(within(row).getByRole("button", { name: /turun/i })).toBeDisabled();
    const second = screen.getByText("Sebutkan 1+1").closest("[data-question-row]") as HTMLElement;
    expect(within(second).getByRole("button", { name: /naik/i })).toBeDisabled();
  });

  it("shows the server error when reorder is rejected as stale", async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error("urutan tidak sesuai"));
    renderWithClient(<TestDetailPage />);

    const row = (await screen.findByText("Apa ibu kota Indonesia?")).closest("[data-question-row]") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /turun/i }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("urutan tidak sesuai"));
    expect(toast.success).not.toHaveBeenCalled();
  });

  it("saves material and instructions together with the other settings", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/materi & instruksi/i);

    fireEvent.change(screen.getByLabelText("Materi bersama"), { target: { value: "<p>Bacaan</p>" } });
    fireEvent.change(screen.getByLabelText("Instruksi section"), { target: { value: "<p>Kerjakan</p>" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan pengaturan/i }));

    await waitFor(() =>
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Tryout UTBK Saintek",
          shared_material_html: "<p>Bacaan</p>",
          instructions_html: "<p>Kerjakan</p>",
        }),
      ),
    );
  });

  it("sends null to clear material that was emptied", async () => {
    testDetailState = {
      ...testDetailState,
      data: { test: { ...sampleTest, shared_material_html: "<p>Lama</p>" }, questions: sampleQuestions },
    };
    renderWithClient(<TestDetailPage />);
    await openTab(/materi & instruksi/i);

    fireEvent.change(screen.getByLabelText("Materi bersama"), { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan pengaturan/i }));

    await waitFor(() =>
      expect(mockMutateAsync).toHaveBeenCalledWith(expect.objectContaining({ shared_material_html: null })),
    );
  });

  it("keeps unsaved drafts across tab switches and server refetches", async () => {
    const { rerender } = renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);
    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul draft" } });

    await openTab(/materi & instruksi/i);
    fireEvent.change(screen.getByLabelText("Materi bersama"), { target: { value: "<p>Draft materi</p>" } });

    testDetailState = {
      ...testDetailState,
      data: { test: { ...sampleTest }, questions: [...sampleQuestions] },
    };
    questionsState = { ...questionsState, data: { data: [...sampleQuestions] } };
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <TestDetailPage />
      </QueryClientProvider>,
    );

    expect(screen.getByLabelText("Materi bersama")).toHaveValue("<p>Draft materi</p>");
    await openTab(/informasi tes/i);
    expect(screen.getByLabelText(/judul/i)).toHaveValue("Judul draft");
  });

  it("blocks saving a Listening test without audio", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    fireEvent.click(screen.getByLabelText("Listening"));

    expect(screen.getByRole("button", { name: /simpan pengaturan/i })).toBeDisabled();
  });

  it("blocks saving a non-positive duration and has nothing to save while clean", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    expect(screen.getByRole("button", { name: /simpan pengaturan/i })).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/durasi/i), { target: { value: "0" } });
    expect(screen.getByRole("button", { name: /simpan pengaturan/i })).toBeDisabled();
  });

  it("preserves an existing audio play limit outside the 1-3 options", async () => {
    testDetailState = {
      ...testDetailState,
      data: { test: { ...sampleTest, audio_play_limit: 5 }, questions: sampleQuestions },
    };
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    expect(screen.getByLabelText(/batas pemutaran audio/i)).toHaveValue("5");
  });

  it("does not report success when the save fails and keeps the draft", async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error("gagal simpan"));
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul baru" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan pengaturan/i }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith("gagal simpan"));
    expect(toast.success).not.toHaveBeenCalled();
    expect(screen.getByLabelText(/judul/i)).toHaveValue("Judul baru");
    expect(screen.getByRole("button", { name: /simpan pengaturan/i })).toBeEnabled();
  });

  it("asks before leaving through the back link only when there is an unsaved draft", async () => {
    const confirmFn = vi.fn(() => false);
    vi.stubGlobal("confirm", confirmFn);
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    fireEvent.click(screen.getByTestId("tests-back-link"));
    expect(confirmFn).not.toHaveBeenCalled();
    expect(push).toHaveBeenCalledWith("/admin/exam/tests");
    push.mockReset();

    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul baru" } });
    fireEvent.click(screen.getByTestId("tests-back-link"));
    expect(confirmFn).toHaveBeenCalledWith("Buang perubahan yang belum disimpan?");
    expect(push).not.toHaveBeenCalled();

    confirmFn.mockReturnValue(true);
    fireEvent.click(screen.getByTestId("tests-back-link"));
    expect(push).toHaveBeenCalledWith("/admin/exam/tests");
  });

  it("warns on reload or tab close only while the draft is dirty", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);

    const clean = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(clean);
    expect(clean.defaultPrevented).toBe(false);

    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul baru" } });
    const dirty = new Event("beforeunload", { cancelable: true });
    window.dispatchEvent(dirty);
    expect(dirty.defaultPrevented).toBe(true);
  });

  it("shows the saved shared material in the sidebar with rich content and a full view", async () => {
    testDetailState = {
      ...testDetailState,
      data: {
        test: {
          ...sampleTest,
          shared_material_html: '<p>Bacaan utama</p><table><tr><td>sel</td></tr></table><img src="https://cdn.test/a.png" alt="gambar"><script>alert(1)</script>',
        },
        questions: sampleQuestions,
      },
    };
    renderWithClient(<TestDetailPage />);

    const sidebar = await screen.findByTestId("test-material-sidebar");
    await waitFor(() => expect(within(sidebar).getByText("Bacaan utama")).toBeInTheDocument());
    expect(within(sidebar).getByText("sel").closest("table")).toBeTruthy();
    expect(within(sidebar).getByRole("img", { name: "gambar" })).toBeInTheDocument();
    expect(sidebar.querySelector("script")).toBeNull();

    fireEvent.click(within(sidebar).getByRole("button", { name: /lihat lengkap/i }));
    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Bacaan utama")).toBeInTheDocument();
  });

  it("shows an empty state when there is no shared material", async () => {
    renderWithClient(<TestDetailPage />);

    const sidebar = await screen.findByTestId("test-material-sidebar");
    expect(within(sidebar).getByText(/belum ada materi bersama/i)).toBeInTheDocument();
    expect(within(sidebar).queryByRole("button", { name: /lihat lengkap/i })).not.toBeInTheDocument();
  });

  it("asks to save first instead of previewing unsaved changes", async () => {
    renderWithClient(<TestDetailPage />);
    await openTab(/informasi tes/i);
    fireEvent.change(screen.getByLabelText(/judul/i), { target: { value: "Judul baru" } });

    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));

    expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/simpan pengaturan terlebih dahulu/i));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("previews the saved test read-only without calling any mutation", async () => {
    renderWithClient(<TestDetailPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Pratinjau" }));

    const dialog = await screen.findByRole("dialog", { name: /tryout utbk saintek/i });
    expect(within(dialog).getByText(/tidak ada sesi atau percobaan/i)).toBeInTheDocument();
    expect(within(dialog).getByText("Apa ibu kota Indonesia?")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("previews a single question from its row icon", async () => {
    renderWithClient(<TestDetailPage />);

    const row = (await screen.findByText("Sebutkan 1+1")).closest("[data-question-row]") as HTMLElement;
    fireEvent.click(within(row).getByRole("button", { name: /pratinjau soal/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Soal 2 dari 2")).toBeInTheDocument();
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  it("reorder down persists the new full question_ids order", async () => {
    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Apa ibu kota Indonesia?")).toBeInTheDocument();
    });

    const row = screen.getByText("Apa ibu kota Indonesia?").closest("[data-question-row]");
    const downButton = within(row as HTMLElement).getByRole("button", { name: /turun/i });
    fireEvent.click(downButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        question_ids: ["q2", "q1"],
      });
    });
  });

  it("From bank opens the picker and attaches selected questions", async () => {
    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Apa ibu kota Indonesia?")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /dari bank/i }));

    await waitFor(() => {
      expect(screen.getByRole("dialog", { name: /pilih dari bank soal/i })).toBeInTheDocument();
    });

    const row = screen.getByText("Bank question one").closest("button");
    expect(row).toBeTruthy();
    fireEvent.click(row as HTMLElement);

    fireEvent.click(screen.getByRole("button", { name: /tambahkan 1 soal/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ question_ids: ["bq1"] });
    });
  });

  it("strips HTML tags from question body in the QuestionRow", async () => {
    const richQuestions: QuestionWithOptions[] = [
      {
        question: {
          id: "qr1",
          format: "mcq",
          body: "<b>bold</b> text",
          sort_order: 1,
          point_correct: 1,
          point_wrong: 0,
        },
        options: [],
      },
    ];
    testDetailState = {
      data: {
        test: sampleTest,
        questions: richQuestions,
      },
      isLoading: false,
      isError: false,
      error: null,
    };
    questionsState = {
      data: { data: richQuestions },
      isLoading: false,
      isError: false,
      error: null,
    };

    renderWithClient(<TestDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("bold text")).toBeInTheDocument();
    });
    expect(screen.queryByText("<b>bold</b> text")).not.toBeInTheDocument();
    const row = screen.getByText("bold text").closest("[data-question-row]") as HTMLElement;
    expect(row).toBeTruthy();
    expect(row.innerHTML).not.toContain("<b>");
  });
});
