import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { QuestionImportModal } from "./QuestionImportModal";

const mockImportAsync = vi.fn();
const mockDownloadAsync = vi.fn();
const mockOnSuccess = vi.fn();
const mockOnOpenChange = vi.fn();

let importState = {
  mutateAsync: mockImportAsync,
  isPending: false,
};

let downloadState = {
  mutateAsync: mockDownloadAsync,
  isPending: false,
};

vi.mock("@/lib/hooks/admin-bank-questions", () => ({
  useImportBankQuestions: () => importState,
  useDownloadQuestionImportTemplate: () => downloadState,
}));

const i18nTemplates: Record<string, string> = {
  import_questions_title: "Import Question CSV",
  import_choose_file: "Choose CSV file",
  import_submit: "Import",
  import_success: "{n} questions imported.",
  import_row_error: "Row {row}: {error}",
  import_errors_title: "Row errors",
  import_no_file: "Please select a file first.",
  saving: "Saving…",
  cancel: "Cancel",
  error_generic: "An error occurred.",
  questions_download_template: "Download Template",
};

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    t: (k: string) => i18nTemplates[k] ?? k,
    lang: "id",
  }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

import { toast } from "sonner";

function renderWithClient(ui: React.ReactNode) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

function makeFile() {
  return new File(
    ["format,body,subject,topic,point_correct,point_wrong\nmcq,Q,Math,Algebra,1,0\n"],
    "q.csv",
    { type: "text/csv" },
  );
}

describe("QuestionImportModal", () => {
  beforeEach(() => {
    importState = { mutateAsync: mockImportAsync, isPending: false };
    downloadState = { mutateAsync: mockDownloadAsync, isPending: false };
    mockImportAsync.mockReset();
    mockDownloadAsync.mockReset();
    mockOnSuccess.mockReset();
    mockOnOpenChange.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders file input and import button", () => {
    renderWithClient(
      <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
    );
    expect(screen.getByLabelText(/Choose CSV file/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Import/i })).toBeInTheDocument();
  });

  it("disables submit until a file is selected", () => {
    renderWithClient(
      <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
    );
    const button = screen.getByRole("button", { name: /Import/i });
    expect(button).toBeDisabled();

    const input = screen.getByLabelText(/Choose CSV file/i) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [makeFile()] } });
    expect(button).not.toBeDisabled();
    expect(mockImportAsync).not.toHaveBeenCalled();
  });

  it("submits the selected file and displays result summary", async () => {
    const file = makeFile();
    mockImportAsync.mockResolvedValueOnce({
      inserted: 2,
      rows: [
        { row_number: 1, status: "inserted", question_id: "q1" },
        { row_number: 2, status: "error", error: "invalid format" },
      ],
    });

    renderWithClient(
      <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
    );

    const input = screen.getByLabelText(/Choose CSV file/i) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [file] } });

    fireEvent.click(screen.getByRole("button", { name: /Import/i }));

    await waitFor(() => {
      expect(mockImportAsync).toHaveBeenCalledWith(file);
      expect(mockOnSuccess).toHaveBeenCalled();
      expect(toast.success).toHaveBeenCalledWith("2 questions imported.");
      expect(screen.getByText(/Row 2: invalid format/i)).toBeInTheDocument();
    });
  });

  it("disables submit while pending", () => {
    importState = { mutateAsync: mockImportAsync, isPending: true };
    renderWithClient(
      <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
    );
    const input = screen.getByLabelText(/Choose CSV file/i) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [makeFile()] } });
    expect(screen.getByRole("button", { name: /Saving/i })).toBeDisabled();
  });

  it("calls onOpenChange when cancel is clicked", () => {
    renderWithClient(
      <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Cancel/i }));
    expect(mockOnOpenChange).toHaveBeenCalledWith(false);
  });

  describe("download template", () => {
    let createObjectURL: ReturnType<typeof vi.fn>;
    let revokeObjectURL: ReturnType<typeof vi.fn>;
    let createElementSpy: ReturnType<typeof vi.spyOn>;
    let anchor: HTMLAnchorElement | undefined;

    beforeEach(() => {
      anchor = undefined;
      createObjectURL = vi.fn().mockReturnValue("blob:mock-url");
      revokeObjectURL = vi.fn();
      URL.createObjectURL = createObjectURL as unknown as typeof URL.createObjectURL;
      URL.revokeObjectURL = revokeObjectURL as unknown as typeof URL.revokeObjectURL;

      const originalCreateElement = document.createElement.bind(document);
      createElementSpy = vi.spyOn(document, "createElement").mockImplementation((tag: string) => {
        const el = originalCreateElement(tag);
        if (tag === "a") {
          anchor = el as HTMLAnchorElement;
          vi.spyOn(el, "click").mockImplementation(() => {});
        }
        return el;
      });
    });

    afterEach(() => {
      createElementSpy.mockRestore();
    });

    it("downloads the template CSV blob and names the file question_import_template.csv", async () => {
      const mockBlob = new Blob(["subject,topic\n"], { type: "text/csv" });
      mockDownloadAsync.mockResolvedValueOnce(mockBlob);

      renderWithClient(
        <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Download Template/i }));

      await waitFor(() => {
        expect(mockDownloadAsync).toHaveBeenCalled();
        expect(createObjectURL).toHaveBeenCalledWith(mockBlob);
        expect(anchor?.download).toBe("question_import_template.csv");
        expect(anchor?.click).toHaveBeenCalled();
        expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock-url");
      });
    });

    it("shows an error toast when the download fails", async () => {
      mockDownloadAsync.mockRejectedValueOnce(new Error("template download failed"));

      renderWithClient(
        <QuestionImportModal open={true} onOpenChange={mockOnOpenChange} onSuccess={mockOnSuccess} />,
      );

      fireEvent.click(screen.getByRole("button", { name: /Download Template/i }));

      await waitFor(() => {
        expect(toast.error).toHaveBeenCalledWith("template download failed");
        expect(createObjectURL).not.toHaveBeenCalled();
      });
    });
  });
});
