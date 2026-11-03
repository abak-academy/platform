import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { ExamAdditionalRulesCard, ExamWhatsAppCard } from "./ExamEventInfo";
import type { ExamDetail } from "@/lib/types";

const mockUpdateExam = vi.fn();
const { toastSuccess, toastError } = vi.hoisted(() => ({ toastSuccess: vi.fn(), toastError: vi.fn() }));

vi.mock("sonner", () => ({ toast: { success: toastSuccess, error: toastError } }));
vi.mock("@/lib/hooks/admin-exams", () => ({
  useUpdateExam: () => ({ mutateAsync: mockUpdateExam, isPending: false }),
}));

const baseExam: ExamDetail = { id: "exam-1", title: "Tryout", tests: [] };
const withRules: ExamDetail = {
  ...baseExam,
  additional_rules_note: "Baca panduan teknis.\nJangan buka tab lain.",
  additional_rules_url: "https://example.com/panduan",
};

function toggle() {
  return screen.getByRole("checkbox", { name: /tambahkan aturan tambahan/i });
}

describe("ExamWhatsAppCard", () => {
  it("shows the stored link as a new-tab anchor when present", () => {
    render(<ExamWhatsAppCard url="https://chat.whatsapp.com/AbC" />);
    expect(screen.getByText("Link tersedia")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /chat\.whatsapp\.com\/AbC/ });
    expect(link).toHaveAttribute("href", "https://chat.whatsapp.com/AbC");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.queryByText("Tidak dicantumkan")).not.toBeInTheDocument();
  });

  it("points to Pengaturan only for roles that can edit", () => {
    const { rerender } = render(<ExamWhatsAppCard url={null} />);
    expect(screen.getByText(/kelola link melalui pengaturan/i)).toBeInTheDocument();
    rerender(<ExamWhatsAppCard url={null} canManage={false} />);
    expect(screen.queryByText(/kelola link melalui pengaturan/i)).not.toBeInTheDocument();
  });

  it.each([undefined, null, ""])("says it is not provided, without a reason, for %s", (url) => {
    render(<ExamWhatsAppCard url={url} />);
    expect(screen.getByText("Tidak dicantumkan")).toBeInTheDocument();
    expect(screen.getByText(/tidak ditampilkan kepada siswa/i)).toBeInTheDocument();
    expect(screen.queryByText("Link tersedia")).not.toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});

describe("ExamAdditionalRulesCard", () => {
  beforeEach(() => {
    mockUpdateExam.mockReset();
    mockUpdateExam.mockResolvedValue({});
    toastSuccess.mockReset();
    toastError.mockReset();
  });

  it("starts as an unchecked toggle with no fields or save button", () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    expect(toggle()).not.toBeChecked();
    expect(screen.queryByLabelText(/keterangan tambahan/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/link aturan lengkap/i)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /simpan aturan tambahan/i })).not.toBeInTheDocument();
  });

  it("opens the note and link fields when toggled on", () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    expect(screen.getByLabelText(/keterangan tambahan/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/link aturan lengkap/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /simpan aturan tambahan/i })).toBeInTheDocument();
  });

  it("rejects an enabled form with neither note nor link and does not call the API", async () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));
    expect(await screen.findByRole("alert")).toHaveTextContent(/isi keterangan atau link/i);
    expect(mockUpdateExam).not.toHaveBeenCalled();
  });

  it.each(["javascript:alert(1)", "ftp://example.com/x", "/relatif", "bukan link"])(
    "rejects an enabled form with the invalid link %s",
    async (value) => {
      render(<ExamAdditionalRulesCard exam={baseExam} />);
      fireEvent.click(toggle());
      fireEvent.input(screen.getByLabelText(/link aturan lengkap/i), { target: { value } });
      fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));
      expect(await screen.findByRole("alert")).toHaveTextContent(/http:\/\/ atau https:\/\//i);
      expect(mockUpdateExam).not.toHaveBeenCalled();
    },
  );

  it("saves only the two rules keys, then shows the stored item with an Edit button", async () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    fireEvent.input(screen.getByLabelText(/keterangan tambahan/i), { target: { value: "  Baca panduan.  " } });
    fireEvent.input(screen.getByLabelText(/link aturan lengkap/i), { target: { value: " https://example.com/p " } });
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));

    await waitFor(() => expect(mockUpdateExam).toHaveBeenCalledTimes(1));
    expect(mockUpdateExam).toHaveBeenCalledWith({
      additional_rules_note: "Baca panduan.",
      additional_rules_url: "https://example.com/p",
    });
    expect(await screen.findByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.getByText("Baca panduan.")).toBeInTheDocument();
    const link = screen.getByRole("link", { name: /baca aturan lengkap/i });
    expect(link).toHaveAttribute("href", "https://example.com/p");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.queryByLabelText(/keterangan tambahan/i)).not.toBeInTheDocument();
  });

  it("sends a missing field as null (note only)", async () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    fireEvent.input(screen.getByLabelText(/keterangan tambahan/i), { target: { value: "Hanya catatan" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));
    await waitFor(() =>
      expect(mockUpdateExam).toHaveBeenCalledWith({ additional_rules_note: "Hanya catatan", additional_rules_url: null }),
    );
  });

  it("renders stored values as an item with Edit instead of a form", () => {
    render(<ExamAdditionalRulesCard exam={withRules} />);
    expect(screen.getByText(/baca panduan teknis\./i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument();
    expect(screen.queryByLabelText(/keterangan tambahan/i)).not.toBeInTheDocument();
  });

  it("Edit reopens the form with the stored values; Cancel discards the draft without saving", () => {
    render(<ExamAdditionalRulesCard exam={withRules} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(toggle()).toBeChecked();
    expect(screen.getByLabelText(/link aturan lengkap/i)).toHaveValue("https://example.com/panduan");
    fireEvent.input(screen.getByLabelText(/keterangan tambahan/i), { target: { value: "draft yang dibuang" } });

    fireEvent.click(screen.getByRole("button", { name: "Batal" }));

    expect(mockUpdateExam).not.toHaveBeenCalled();
    expect(screen.getByText(/baca panduan teknis\./i)).toBeInTheDocument();
    expect(screen.queryByText(/draft yang dibuang/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText(/keterangan tambahan/i)).toHaveValue("Baca panduan teknis.\nJangan buka tab lain.");
  });

  it("Cancel on a fresh toggle returns to the unchecked state", () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    fireEvent.click(screen.getByRole("button", { name: "Batal" }));
    expect(toggle()).not.toBeChecked();
    expect(mockUpdateExam).not.toHaveBeenCalled();
  });

  it("turning the toggle off and saving clears both fields with null", async () => {
    render(<ExamAdditionalRulesCard exam={withRules} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.click(toggle());
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));

    await waitFor(() =>
      expect(mockUpdateExam).toHaveBeenCalledWith({ additional_rules_note: null, additional_rules_url: null }),
    );
    await waitFor(() => expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument());
    expect(toggle()).not.toBeChecked();
  });

  it("a disabled form never blocks save because of an invalid draft link", async () => {
    render(<ExamAdditionalRulesCard exam={withRules} />);
    fireEvent.click(screen.getByRole("button", { name: "Edit" }));
    fireEvent.input(screen.getByLabelText(/link aturan lengkap/i), { target: { value: "javascript:alert(1)" } });
    fireEvent.click(toggle());
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));

    await waitFor(() =>
      expect(mockUpdateExam).toHaveBeenCalledWith({ additional_rules_note: null, additional_rules_url: null }),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("stays editable when the exam already has sessions (not a policy field)", () => {
    render(<ExamAdditionalRulesCard exam={{ ...baseExam, format_version: 2, has_sessions: true }} />);
    expect(toggle()).toBeEnabled();
    fireEvent.click(toggle());
    expect(screen.getByLabelText(/keterangan tambahan/i)).toBeEnabled();
    expect(screen.getByRole("button", { name: /simpan aturan tambahan/i })).toBeEnabled();
  });

  it("keeps the draft and reports the error when saving fails", async () => {
    mockUpdateExam.mockRejectedValue(new Error("boom"));
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    fireEvent.click(toggle());
    fireEvent.input(screen.getByLabelText(/keterangan tambahan/i), { target: { value: "Catatan" } });
    fireEvent.click(screen.getByRole("button", { name: /simpan aturan tambahan/i }));

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("boom"));
    expect(screen.getByLabelText(/keterangan tambahan/i)).toHaveValue("Catatan");
    expect(screen.queryByRole("button", { name: "Edit" })).not.toBeInTheDocument();
  });

  it("does not claim to save together with the Test list", () => {
    render(<ExamAdditionalRulesCard exam={baseExam} />);
    expect(screen.getByText(/disimpan terpisah dari daftar test/i)).toBeInTheDocument();
    expect(within(screen.getByRole("heading", { name: "Aturan pengerjaan" }).closest("section")!).getByRole("checkbox")).toBeInTheDocument();
  });
});
