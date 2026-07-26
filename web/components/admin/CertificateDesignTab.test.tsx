import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import { CertificateDesignTab } from "./CertificateDesignTab";
import type { CertificateDesign, ExamDetail } from "@/lib/types";

const mockFetchCertificatePreview = vi.fn();
const mockUpdateDesignMutateAsync = vi.fn();
const mockPresignMutateAsync = vi.fn();

let certificateDesignState: {
  data: CertificateDesign | undefined;
  isLoading: boolean;
  isError: boolean;
} = { data: undefined, isLoading: true, isError: false };

const sampleLayout: CertificateDesign["layout"] = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [],
};

// layoutWithField carries one visible field so the drag/debounce tests below
// have a box to interact with; container is mocked to 1188x840px (see
// CertificateFieldEditor.test.tsx) so 0.25mm/px holds on both axes.
const layoutWithField: CertificateDesign["layout"] = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [
    {
      id: "student_name",
      x_mm: 48.5,
      y_mm: 100,
      w_mm: 200,
      align: "center",
      font: "source_serif_4",
      weight: "bold",
      size_pt: 26,
      color: "#1F2A44",
      visible: true,
    },
  ],
};

vi.mock("@/lib/hooks/admin-exams", () => ({
  useCertificateDesign: () => certificateDesignState,
  useUpdateCertificateDesign: () => ({
    mutateAsync: mockUpdateDesignMutateAsync,
    isPending: false,
  }),
  fetchCertificatePreview: (...args: unknown[]) => mockFetchCertificatePreview(...args),
}));

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({ mutateAsync: mockPresignMutateAsync, isPending: false }),
}));

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const sampleExam: ExamDetail = {
  id: "exam-1",
  title: "UTS Matematika",
  certificate_template: "custom",
  tests: [],
};

describe("CertificateDesignTab", () => {
  beforeEach(() => {
    mockFetchCertificatePreview
      .mockReset()
      .mockResolvedValue(new Blob(["pdf"], { type: "application/pdf" }));
    mockUpdateDesignMutateAsync.mockReset();
    mockPresignMutateAsync.mockReset();
    certificateDesignState = {
      data: {
        template: "classic",
        background_key: null,
        background_url: null,
        signature_url: null,
        layout: sampleLayout,
      },
      isLoading: false,
      isError: false,
    };
    vi.mocked(toast.success).mockClear();
    vi.mocked(toast.error).mockClear();
    URL.createObjectURL = vi.fn().mockReturnValue("blob:test-url");
    URL.revokeObjectURL = vi.fn();
  });

  it("does not auto-generate the PDF preview on mount (FR-17)", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-generate-pdf-button");
    expect(mockFetchCertificatePreview).not.toHaveBeenCalled();
  });

  it("generates the PDF preview only when the Generate PDF button is clicked, carrying the current layout", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    const button = await screen.findByTestId("certificate-generate-pdf-button");
    fireEvent.click(button);

    await waitFor(() => {
      expect(mockFetchCertificatePreview).toHaveBeenCalledWith("exam-1", "classic", sampleLayout);
    });
    expect(mockFetchCertificatePreview).toHaveBeenCalledTimes(1);
  });

  it("does not refresh the preview when the template is switched until Generate PDF is clicked", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);
    await screen.findByTestId("certificate-generate-pdf-button");

    fireEvent.click(screen.getByLabelText("Modern"));
    expect(mockFetchCertificatePreview).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("certificate-generate-pdf-button"));

    await waitFor(() => {
      expect(mockFetchCertificatePreview).toHaveBeenCalledWith("exam-1", "modern", sampleLayout);
    });
    expect(mockFetchCertificatePreview).toHaveBeenCalledTimes(1);
  });

  it("reflects a drag in the layout without any auto-render, only on the next Generate PDF click", async () => {
    certificateDesignState = {
      data: {
        template: "classic",
        background_key: null,
        background_url: "https://cdn.example.com/bg.png",
        signature_url: null,
        layout: layoutWithField,
      },
      isLoading: false,
      isError: false,
    };
    vi.spyOn(HTMLDivElement.prototype, "getBoundingClientRect").mockReturnValue({
      width: 1188,
      height: 840,
      left: 0,
      top: 0,
      right: 1188,
      bottom: 840,
      x: 0,
      y: 0,
      toJSON: () => {},
    } as DOMRect);

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);
    await screen.findByTestId("certificate-generate-pdf-button");

    const box = screen.getByTestId("certificate-field-box-student_name");
    // Grab exactly at the box's top-left (48.5mm,100mm) -> (194px,400px), drop
    // at (20mm,150mm) — the same drop CertificateFieldEditor.test.tsx uses.
    fireEvent.pointerDown(box, { pointerId: 1, clientX: 194, clientY: 400 });
    fireEvent.pointerMove(box, { pointerId: 1, clientX: 80, clientY: 600 });
    fireEvent.pointerUp(box, { pointerId: 1 });

    expect(mockFetchCertificatePreview).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("certificate-generate-pdf-button"));

    await waitFor(() => {
      expect(mockFetchCertificatePreview).toHaveBeenCalledTimes(1);
    });

    const lastCall =
      mockFetchCertificatePreview.mock.calls[mockFetchCertificatePreview.mock.calls.length - 1];
    expect(lastCall[0]).toBe("exam-1");
    expect(lastCall[1]).toBe("classic");
    const dragged = lastCall[2].fields.find((f: { id: string }) => f.id === "student_name");
    expect(dragged.x_mm).toBeCloseTo(20, 5);
    expect(dragged.y_mm).toBeCloseTo(150, 5);
  });

  it("carries rapid consecutive layout edits into the single Generate PDF request that follows", async () => {
    certificateDesignState = {
      data: {
        template: "classic",
        background_key: null,
        background_url: "https://cdn.example.com/bg.png",
        signature_url: null,
        layout: layoutWithField,
      },
      isLoading: false,
      isError: false,
    };

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);
    await screen.findByTestId("certificate-generate-pdf-button");

    const xInput = screen.getByLabelText("x_mm student_name");
    fireEvent.change(xInput, { target: { value: "10" } });
    fireEvent.change(xInput, { target: { value: "20" } });

    expect(mockFetchCertificatePreview).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("certificate-generate-pdf-button"));

    await waitFor(() => {
      expect(mockFetchCertificatePreview).toHaveBeenCalledTimes(1);
    });

    const lastCall =
      mockFetchCertificatePreview.mock.calls[mockFetchCertificatePreview.mock.calls.length - 1];
    const dragged = lastCall[2].fields.find((f: { id: string }) => f.id === "student_name");
    expect(dragged.x_mm).toBe(20);
  });

  it("uploads a background then saves with the returned object key", async () => {
    mockPresignMutateAsync.mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "certificates/exam-1/new-bg.png",
    });
    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);
    mockUpdateDesignMutateAsync.mockResolvedValue({
      template: "custom",
      background_url: "https://signed.example.com/new-bg.png",
      layout: sampleLayout,
    });

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-background-upload-input");

    const fileInput = screen.getByTestId("certificate-background-upload-input");
    const file = new File(["bg"], "bg.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(mockPresignMutateAsync).toHaveBeenCalledWith({
        filename: "bg.png",
        content_type: "image/png",
      });
    });

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith(
        "https://upload.example.com/put-here",
        expect.objectContaining({ method: "PUT", body: file }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockUpdateDesignMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          template: "classic",
          background_key: "certificates/exam-1/new-bg.png",
          layout: sampleLayout,
        }),
      );
    });

    vi.unstubAllGlobals();
  });

  it("uploads a signature and saves it inside the layout as a visible signature field", async () => {
    mockPresignMutateAsync.mockResolvedValue({
      url: "https://upload.example.com/put-sig",
      method: "PUT",
      key: "certificates/exam-1/sig.png",
    });
    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);
    mockUpdateDesignMutateAsync.mockResolvedValue({
      template: "classic",
      background_url: null,
      signature_url: null,
      layout: sampleLayout,
    });

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-background-upload-input");

    const sigInput = screen.getByTestId("certificate-signature-upload-input");
    const file = new File(["sig"], "sig.png", { type: "image/png" });
    fireEvent.change(sigInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith(
        "https://upload.example.com/put-sig",
        expect.objectContaining({ method: "PUT", body: file }),
      );
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      const call = mockUpdateDesignMutateAsync.mock.calls.at(-1)?.[0];
      expect(call?.layout?.signature_key).toBe("certificates/exam-1/sig.png");
      const sig = call?.layout?.fields?.find((f: { id: string }) => f.id === "signature");
      expect(sig?.visible).toBe(true);
    });

    vi.unstubAllGlobals();
  });

  // Regression: the key used to be read off exam.certificate_background_key, a
  // column migration 0042 dropped, so saving without touching the background
  // sent background_key:null and erased the persisted upload.
  it("round-trips the saved background key when the background isn't touched", async () => {
    certificateDesignState = {
      data: {
        template: "custom",
        background_key: "certificates/exam-1/old-bg.png",
        background_url: "https://signed.example.com/old-bg.png",
        signature_url: null,
        layout: sampleLayout,
      },
      isLoading: false,
      isError: false,
    };
    mockUpdateDesignMutateAsync.mockResolvedValue({
      template: "custom",
      background_key: "certificates/exam-1/old-bg.png",
      background_url: null,
      layout: sampleLayout,
    });

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-background-upload-input");

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockUpdateDesignMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          background_key: "certificates/exam-1/old-bg.png",
        }),
      );
    });
  });

  it("surfaces a toast when saving fails instead of failing silently", async () => {
    mockUpdateDesignMutateAsync.mockRejectedValue(new Error("boom"));

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-background-upload-input");

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });
  });

  it("surfaces a toast when the upload PUT fails instead of failing silently", async () => {
    mockPresignMutateAsync.mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "certificates/exam-1/new-bg.png",
    });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500 }));

    render(<CertificateDesignTab examId="exam-1" exam={sampleExam} />);

    await screen.findByTestId("certificate-background-upload-input");

    const fileInput = screen.getByTestId("certificate-background-upload-input");
    const file = new File(["bg"], "bg.png", { type: "image/png" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled();
    });

    vi.unstubAllGlobals();
  });
});
