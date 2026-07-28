import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { CertificateDesignTab } from "./CertificateDesignTab";
import type { CertificateDesign, ExamDetail } from "@/lib/types";

const update = vi.fn();
const presign = vi.fn();
let design: CertificateDesign;

vi.mock("@/lib/hooks/admin-exams", () => ({
  useCertificateDesign: () => ({ data: design, isLoading: false, isError: false }),
  useUpdateCertificateDesign: () => ({ mutateAsync: update, isPending: false }),
  usePresignCertificateAsset: () => ({ mutateAsync: presign }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const classic = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [{
    id: "student_name", kind: "text" as const, name: "Student name",
    content: "{{student_name}}", x_mm: 48.5, y_mm: 100, w_mm: 200,
    align: "center", font: "public_sans", weight: "regular",
    size_pt: 26, color: "#17213B", visible: true,
  }],
};

const exam: ExamDetail = { id: "exam-1", title: "Ujian Matematika", tests: [] };

describe("CertificateDesignTab", () => {
  beforeEach(() => {
    update.mockReset().mockResolvedValue({});
    presign.mockReset();
    vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
    URL.createObjectURL = vi.fn(() => "blob:asset");
    design = {
      template: "classic",
      background_key: null,
      background_url: "data:image/png;base64,classic",
      signature_url: null,
      layout: classic,
      presets: [
        { template: "classic", background_url: "data:image/png;base64,classic", layout: classic },
        { template: "modern", background_url: "data:image/png;base64,modern", layout: { ...classic, background: { kind: "builtin", ref: "modern" }, fields: [{ ...classic.fields[0], x_mm: 60 }] } },
      ],
      asset_urls: {},
    };
  });

  it("removes PDF preview and manual coordinates", () => {
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    expect(screen.queryByText(/Generate PDF/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/x_mm/i)).not.toBeInTheDocument();
    expect(screen.getByText("Pratinjau langsung")).toBeInTheDocument();
  });

  it("uses the default UI language", () => {
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    expect(screen.getByText("Editor Sertifikat")).toBeInTheDocument();
    expect(screen.getByText("Tambah elemen")).toBeInTheDocument();
  });

  it("edits content and typography in the saved payload", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    fireEvent.change(screen.getByLabelText("Konten"), { target: { value: "Awarded to {{student_name}}" } });
    fireEvent.click(screen.getByLabelText("Tebal"));
    fireEvent.click(screen.getByRole("button", { name: /Simpan perubahan/i }));
    await waitFor(() => expect(update).toHaveBeenCalled());
    const field = update.mock.calls[0][0].layout.fields[0];
    expect(field).toMatchObject({ content: "Awarded to {{student_name}}", weight: "bold" });
  });

  it("selecting a preset replaces its layout and background", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    fireEvent.click(screen.getByRole("button", { name: /modern/i }));
    const background = screen.getByTestId("certificate-field-editor-background") as HTMLImageElement;
    expect(background.src).toContain("data:image/png;base64,modern");
    fireEvent.click(screen.getByRole("button", { name: /Simpan perubahan/i }));
    await waitFor(() => expect(update).toHaveBeenCalledWith(expect.objectContaining({
      template: "modern",
      layout: expect.objectContaining({ background: { kind: "builtin", ref: "modern" } }),
    })));
  });

  it("uploads a custom background without replacing layers", async () => {
    presign.mockResolvedValue({ url: "https://upload", key: "certificates/exam-1/bg.png" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    const input = screen.getByTestId("certificate-background-upload-input");
    fireEvent.change(input, { target: { files: [new File(["bg"], "bg.png", { type: "image/png" })] } });
    await waitFor(() => expect(presign).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: /Simpan perubahan/i }));
    await waitFor(() => expect(update).toHaveBeenCalledWith(expect.objectContaining({
      template: "custom",
      background_key: "certificates/exam-1/bg.png",
      layout: expect.objectContaining({ fields: expect.arrayContaining([expect.objectContaining({ id: "student_name" })]) }),
    })));
  });

  it("adds an optional score token and saves the new layer", async () => {
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);
    const catalog = screen.getByText("Tambah elemen").closest("section");
    if (!catalog) throw new Error("element catalog not found");
    fireEvent.click(within(catalog).getByRole("button", { name: "Skor" }));
    expect(screen.getByText("86")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Simpan perubahan/i }));
    await waitFor(() => expect(update).toHaveBeenCalledWith(expect.objectContaining({
      layout: expect.objectContaining({
        fields: expect.arrayContaining([expect.objectContaining({
          id: "score",
          content: "{{score}}",
        })]),
      }),
    })));
  });

  it("uploads and saves a new image layer", async () => {
    presign.mockResolvedValue({ url: "https://upload", key: "certificates/exam-1/logo.png" });
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));
    render(<CertificateDesignTab examId="exam-1" exam={exam} />);

    fireEvent.change(screen.getByTestId("certificate-image-upload-input"), {
      target: { files: [new File(["logo"], "logo.png", { type: "image/png" })] },
    });

    await waitFor(() => expect(presign).toHaveBeenCalled());
    fireEvent.click(screen.getByRole("button", { name: /Simpan perubahan/i }));
    await waitFor(() => expect(update).toHaveBeenCalledWith(expect.objectContaining({
      layout: expect.objectContaining({
        fields: expect.arrayContaining([expect.objectContaining({
          kind: "image",
          asset_key: "certificates/exam-1/logo.png",
          name: "logo.png",
        })]),
      }),
    })));
  });
});
