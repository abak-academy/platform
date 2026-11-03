import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { ExamModal } from "./ExamModal";
import type { ExamListItem } from "@/lib/types";

vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const mockCreateExam = vi.fn();
const mockUpdateExam = vi.fn();

vi.mock("@/lib/hooks/admin-exams", () => ({
  useCreateExam: () => ({ mutateAsync: mockCreateExam, isPending: false }),
  useUpdateExam: () => ({ mutateAsync: mockUpdateExam, isPending: false }),
}));

// ImageUploadInput (end screen) calls usePresignAdminImageUpload, a real
// useMutation — mock it so rendering the modal doesn't need a QueryClient.
vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminImageUpload: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

const sampleExam: ExamListItem = {
  id: "exam-1",
  title: "UTS Matematika",
  certificate_template: "modern",
  timer_mode: "overall",
  duration_minutes: 90,
  result_config: "score_only",
  result_release_at: "2026-07-02T10:00:00Z",
  check_in_window_minutes: 15,
  grace_window_minutes: 5,
  max_attempts: 2,
};

describe("ExamModal", () => {
  beforeEach(() => {
    mockCreateExam.mockReset();
    mockUpdateExam.mockReset();
  });

  it("FR-25c: saving from the modal never touches certificate design fields", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1", title: "UTS Matematika" });

    const customExam: ExamListItem = {
      ...sampleExam,
      certificate_template: "custom",
      certificate_background_key: "certificates/exam-1/bg.png",
      certificate_layout: {
        page: { width_mm: 297, height_mm: 210 },
        background: { kind: "custom", ref: "certificates/exam-1/bg.png" },
        fields: [],
      },
      certificate_design_updated_at: "2026-07-01T00:00:00Z",
    };

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={customExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toHaveValue("UTS Matematika");
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalled();
    });

    const payload = mockUpdateExam.mock.calls[0][0];
    expect(payload).not.toHaveProperty("certificate_template");
    expect(payload).not.toHaveProperty("certificate_background_key");
    expect(payload).not.toHaveProperty("certificate_layout");
    expect(payload).not.toHaveProperty("certificate_design_updated_at");
  });

  it("offers navigation instead of named modes for new exams", () => {
    render(<ExamModal open={true} onClose={vi.fn()} />);
    expect(screen.getByLabelText("Bebas")).toBeChecked();
    expect(screen.getByLabelText("Berurutan")).not.toBeChecked();
    expect(screen.queryByLabelText("UTBK")).not.toBeInTheDocument();
  });

  it.each(["Bebas", "Berurutan"])("sends %s navigation on create", async label => {
    mockCreateExam.mockResolvedValue({ id: "new-exam" });
    render(<ExamModal open={true} onClose={vi.fn()} />);
    fireEvent.input(screen.getByLabelText(/judul/i), { target:{value:"Configured Exam"} });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target:{value:"60"} });
    fireEvent.click(screen.getByLabelText(label));
    fireEvent.click(screen.getByRole("button", {name:/^simpan(?: pengaturan)?$/i}));
    await waitFor(() => expect(mockCreateExam).toHaveBeenCalledWith(expect.objectContaining({navigation_policy:label === "Bebas" ? "free" : "sequential"})));
    expect(mockCreateExam.mock.calls[0][0]).not.toHaveProperty("mode");
  });

  it("per-section timer requires sequential navigation", () => {
    render(<ExamModal open={true} onClose={vi.fn()} />);
    fireEvent.input(screen.getByLabelText(/judul/i), {target:{value:"Configured Exam"}});
    fireEvent.click(screen.getByLabelText("Per section"));
    expect(screen.getByLabelText("Berurutan")).toBeChecked();
    fireEvent.click(screen.getByLabelText("Bebas"));
    expect(screen.getByRole("button", {name:/^simpan(?: pengaturan)?$/i})).toBeDisabled();
  });

  it("includes mode in update payload", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1", title: "Updated" });

    render(
      <ExamModal
        open={true}
        onClose={vi.fn()}
        exam={{ ...sampleExam, mode: "utbk" }}
        onSaved={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("UTBK")).toBeChecked();
    });

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "Updated Title" },
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalledWith(
        expect.objectContaining({ mode: "utbk", title: "Updated Title" }),
      );
    });
  });

  it("shows a hint when utbk or ielts mode is selected", () => {
    const { rerender } = render(
      <ExamModal open={true} onClose={vi.fn()} exam={{...sampleExam, format_version:1}} onSaved={vi.fn()} />,
    );

    // Default: no hint
    expect(
      screen.queryByText(
        "Setiap tes terlampir akan menjadi sesi dengan timer tersendiri.",
      ),
    ).not.toBeInTheDocument();

    // Select UTBK → hint appears
    fireEvent.click(screen.getByLabelText("UTBK"));
    expect(
      screen.getByText(
        "Setiap tes terlampir akan menjadi sesi dengan timer tersendiri.",
      ),
    ).toBeInTheDocument();

    // Switch back to Standard → hint disappears
    fireEvent.click(screen.getByLabelText("Standar"));
    expect(
      screen.queryByText(
        "Setiap tes terlampir akan menjadi sesi dengan timer tersendiri.",
      ),
    ).not.toBeInTheDocument();
  });

  it("pre-fills mode from exam data on edit", async () => {
    const examWithMode: ExamListItem = {
      ...sampleExam,
      mode: "ielts",
    };

    render(
      <ExamModal
        open={true}
        onClose={vi.fn()}
        exam={examWithMode}
        onSaved={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("IELTS")).toBeChecked();
      expect(screen.getByLabelText("Standar")).not.toBeChecked();
    });
  });

  it("pre-fills extended package fields from exam data on edit", async () => {
    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(
        (screen.getByLabelText(/konfigurasi hasil/i) as HTMLSelectElement).value,
      ).toBe("score_only");
    });

    expect(
      (screen.getByLabelText(/jendela check-in/i) as HTMLInputElement).value,
    ).toBe("15");
    expect(
      (screen.getByLabelText(/jendela toleransi/i) as HTMLInputElement).value,
    ).toBe("5");
    expect(screen.getByLabelText("Izinkan lebih dari satu percobaan")).toBeChecked();
    expect(screen.getByLabelText("Jumlah percobaan")).toHaveValue(2);
  });

  it("submitted update payload includes extended package fields", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1", title: "UTS Matematika" });

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toBeInTheDocument();
    });

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "UTS Matematika Updated" },
    });
    fireEvent.change(screen.getByLabelText(/konfigurasi hasil/i), {
      target: { value: "score_pembahasan" },
    });
    fireEvent.input(screen.getByLabelText(/jendela check-in/i), {
      target: { value: "30" },
    });
    fireEvent.input(screen.getByLabelText(/jendela toleransi/i), {
      target: { value: "10" },
    });
    fireEvent.input(screen.getByLabelText("Jumlah percobaan"), {
      target: { value: "3" },
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "UTS Matematika Updated",
          result_config: "score_pembahasan",
          check_in_window_minutes: 30,
          grace_window_minutes: 10,
          max_attempts: 3,
        }),
      );
    });
  });

  it("number window inputs reject negatives via min attribute", () => {
    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    expect(screen.getByLabelText(/jendela check-in/i)).toHaveAttribute("min", "0");
    expect(screen.getByLabelText(/jendela toleransi/i)).toHaveAttribute("min", "0");
  });

  it("omits empty windows from create payload and defaults attempts to 1", async () => {
    mockCreateExam.mockResolvedValue({ id: "exam-1" });

    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "Paket Minimal" },
    });
    fireEvent.click(screen.getByLabelText("Per section"));

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockCreateExam).toHaveBeenCalledWith(
        expect.objectContaining({
          check_in_window_minutes: null,
          grace_window_minutes: null,
          max_attempts: 1,
        }),
      );
    });
  });

  it("FR-43: explains what students see for each result_config option, and that never publishing is a supported end state", () => {
    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    expect(
      screen.getByText(/siswa tidak melihat skor atau pembahasan/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/siswa hanya melihat skor akhir/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/siswa melihat skor akhir beserta pembahasan/i),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/tidak pernah mempublikasikan hasil.*bukan kesalahan konfigurasi/i),
    ).toBeInTheDocument();
  });

  it("result_config is constrained to allowed options", () => {
    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    const select = screen.getByLabelText(/konfigurasi hasil/i) as HTMLSelectElement;
    const options = Array.from(select.options).map((o) => o.value);
    expect(options).toContain("hidden");
    expect(options).toContain("score_only");
    expect(options).toContain("score_pembahasan");
  });

  it("pre-fills scheduled start and end from exam data on edit", async () => {
    render(
      <ExamModal
        open={true}
        onClose={vi.fn()}
        exam={{
          ...sampleExam,
          scheduled_at: "2026-08-01T09:00:00Z",
          scheduled_end_at: "2026-08-03T09:00:00Z",
        }}
        onSaved={vi.fn()}
      />,
    );

    await waitFor(() => {
      expect((screen.getByLabelText("Mulai akses") as HTMLInputElement).value).not.toBe("");
    });
    expect((screen.getByLabelText("Batas akhir mulai sesi") as HTMLInputElement).value).not.toBe("");
  });

  it("omits scheduled_end_at from the payload when left blank", async () => {
    mockCreateExam.mockResolvedValue({ id: "exam-1" });

    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "No Window Exam" },
    });
    fireEvent.click(screen.getByLabelText("Per section"));
    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockCreateExam).toHaveBeenCalledWith(
        expect.objectContaining({ scheduled_end_at: null }),
      );
    });
  });

  it("includes scheduled_end_at in the payload when set after start", async () => {
    mockCreateExam.mockResolvedValue({ id: "exam-1" });

    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "Window Exam" },
    });
    fireEvent.click(screen.getByLabelText("Per section"));
    fireEvent.input(screen.getByLabelText("Mulai akses"), {
      target: { value: "2026-08-01T09:00" },
    });
    fireEvent.input(screen.getByLabelText("Batas akhir mulai sesi"), {
      target: { value: "2026-08-03T09:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockCreateExam).toHaveBeenCalledWith(
        expect.objectContaining({ scheduled_end_at: expect.any(String) }),
      );
    });
  });

  it("blocks submit and shows an error when end is not after start", async () => {
    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    fireEvent.input(screen.getByLabelText(/judul/i), {
      target: { value: "Invalid Window Exam" },
    });
    fireEvent.input(screen.getByLabelText("Mulai akses"), {
      target: { value: "2026-08-01T09:00" },
    });
    fireEvent.input(screen.getByLabelText("Batas akhir mulai sesi"), {
      target: { value: "2026-08-01T08:00" },
    });

    expect(
      screen.getByText("Waktu berakhir harus setelah waktu mulai"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i })).toBeDisabled();
    expect(mockCreateExam).not.toHaveBeenCalled();
  });

  // ── End screen (FR-38/FR-39, Task 20) ──────────────────────────────────

  it("does not show the end screen section on create (exam does not exist yet)", () => {
    render(<ExamModal open={true} onClose={vi.fn()} onSaved={vi.fn()} />);

    expect(screen.queryByText("Layar Akhir Ujian")).not.toBeInTheDocument();
  });

  it("pre-fills end screen fields from exam data on edit", async () => {
    const examWithEndScreen: ExamListItem = {
      ...sampleExam,
      end_screen_image_url: "https://cdn.example.com/end-screen.png",
      end_screen_promo_text: "Thanks for taking the exam!",
    };

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={examWithEndScreen} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(
        (screen.getByLabelText("Gambar Layar Akhir") as HTMLInputElement).value,
      ).toBe("https://cdn.example.com/end-screen.png");
    });
    expect(
      (screen.getByLabelText("Teks Promo") as HTMLTextAreaElement).value,
    ).toBe("Thanks for taking the exam!");
  });

  it("submitted update payload includes end screen fields", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1", title: "UTS Matematika" });

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toBeInTheDocument();
    });

    fireEvent.input(screen.getByLabelText("Gambar Layar Akhir"), {
      target: { value: "https://cdn.example.com/end-screen.png" },
    });
    fireEvent.input(screen.getByLabelText("Teks Promo"), {
      target: { value: "Thanks for taking the exam!" },
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalledWith(
        expect.objectContaining({
          end_screen_image_url: "https://cdn.example.com/end-screen.png",
          end_screen_promo_text: "Thanks for taking the exam!",
        }),
      );
    });
  });

  it("submits null end screen fields when left blank", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1", title: "UTS Matematika" });

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/judul/i)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalledWith(
        expect.objectContaining({
          end_screen_image_url: null,
          end_screen_promo_text: null,
        }),
      );
    });
  });

  it("sends the edited card notes, dropping blank rows", async () => {
    mockUpdateExam.mockResolvedValue({ id: "exam-1" });

    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /tambah poin/i })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /tambah poin/i }));
    fireEvent.click(screen.getByRole("button", { name: /tambah poin/i }));
    fireEvent.input(screen.getByLabelText("Poin 1"), {
      target: { value: "  Bawa kartu identitas.  " },
    });
    // Poin 2 stays blank and must not reach the payload.

    fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    await waitFor(() => {
      expect(mockUpdateExam).toHaveBeenCalledWith(
        expect.objectContaining({ card_notes: ["Bawa kartu identitas."] }),
      );
    });
  });

  it("stops offering new bullets at the five-note cap", async () => {
    render(
      <ExamModal open={true} onClose={vi.fn()} exam={sampleExam} onSaved={vi.fn()} />,
    );

    const addButton = await screen.findByRole("button", { name: /tambah poin/i });
    for (let i = 0; i < 5; i++) fireEvent.click(addButton);

    expect(screen.getByLabelText("Poin 5")).toBeInTheDocument();
    expect(addButton).toBeDisabled();
  });

  describe("attempts", () => {
    async function submitCreate() {
      fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Paket Percobaan" } });
      fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });
      fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));
    }

    it("create defaults to a single attempt with the multi-attempt box off", async () => {
      mockCreateExam.mockResolvedValue({ id: "n" });
      render(<ExamModal open={true} onClose={vi.fn()} />);

      expect(screen.getByLabelText("Izinkan lebih dari satu percobaan")).not.toBeChecked();
      expect(screen.queryByLabelText("Jumlah percobaan")).not.toBeInTheDocument();
      await submitCreate();

      await waitFor(() => expect(mockCreateExam).toHaveBeenCalledWith(expect.objectContaining({ max_attempts: 1 })));
    });

    it("checking the box defaults the count to 2 and sends it", async () => {
      mockCreateExam.mockResolvedValue({ id: "n" });
      render(<ExamModal open={true} onClose={vi.fn()} />);

      fireEvent.click(screen.getByLabelText("Izinkan lebih dari satu percobaan"));
      expect(screen.getByLabelText("Jumlah percobaan")).toHaveValue(2);
      await submitCreate();

      await waitFor(() => expect(mockCreateExam).toHaveBeenCalledWith(expect.objectContaining({ max_attempts: 2 })));
    });

    it("Tanpa batas hides the count input and sends null", async () => {
      mockCreateExam.mockResolvedValue({ id: "n" });
      render(<ExamModal open={true} onClose={vi.fn()} />);

      fireEvent.click(screen.getByLabelText("Izinkan lebih dari satu percobaan"));
      fireEvent.click(screen.getByLabelText("Tanpa batas"));
      expect(screen.queryByLabelText("Jumlah percobaan")).not.toBeInTheDocument();
      await submitCreate();

      await waitFor(() => expect(mockCreateExam).toHaveBeenCalledWith(expect.objectContaining({ max_attempts: null })));
    });

    it.each(["", "1"])("disables submit when the count is %j and Tanpa batas is off", (value) => {
      render(<ExamModal open={true} onClose={vi.fn()} />);
      fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Paket" } });
      fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });
      fireEvent.click(screen.getByLabelText("Izinkan lebih dari satu percobaan"));
      fireEvent.input(screen.getByLabelText("Jumlah percobaan"), { target: { value } });

      expect(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i })).toBeDisabled();
    });

    it.each([
      [0, false, false, null],
      [1, false, false, null],
      [null, true, true, null],
      [5, true, false, 5],
    ])("edit with max_attempts=%j maps to multiple=%j unlimited=%j", (maxAttempts, multiple, unlimited, count) => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...sampleExam, max_attempts: maxAttempts }} />);

      const box = screen.getByLabelText("Izinkan lebih dari satu percobaan");
      if (multiple) expect(box).toBeChecked();
      else expect(box).not.toBeChecked();
      if (multiple) {
        if (unlimited) expect(screen.getByLabelText("Tanpa batas")).toBeChecked();
        else expect(screen.getByLabelText("Jumlah percobaan")).toHaveValue(count);
      }
    });

    it("opening and closing an existing exam without saving never calls the update", async () => {
      const onClose = vi.fn();
      render(<ExamModal open={true} onClose={onClose} exam={{ ...sampleExam, max_attempts: 0 }} />);

      fireEvent.click(screen.getByRole("button", { name: /^batal$/i }));

      expect(onClose).toHaveBeenCalled();
      expect(mockUpdateExam).not.toHaveBeenCalled();
    });

    it("saving an exam that was unlimited keeps null", async () => {
      mockUpdateExam.mockResolvedValue({ id: "exam-1" });
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...sampleExam, max_attempts: null }} />);

      fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

      await waitFor(() => expect(mockUpdateExam).toHaveBeenCalledWith(expect.objectContaining({ max_attempts: null })));
    });
  });

  describe("policy lock", () => {
    const composableExam: ExamListItem = { ...sampleExam, format_version: 2, navigation_policy: "free", randomize: true };

    it("disables navigation, timer, duration and randomize with an explanation, leaving other fields editable", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} policyLocked />);

      expect(screen.getByText(/terkunci karena ujian ini sudah memiliki sesi/i)).toBeInTheDocument();
      expect(screen.getByLabelText("Bebas")).toBeDisabled();
      expect(screen.getByLabelText("Berurutan")).toBeDisabled();
      expect(screen.getByLabelText("Seluruh ujian")).toBeDisabled();
      expect(screen.getByLabelText("Per section")).toBeDisabled();
      expect(screen.getByLabelText(/durasi/i)).toBeDisabled();
      expect(screen.getByLabelText("Acak Soal")).toBeDisabled();

      expect(screen.getByLabelText(/judul/i)).toBeEnabled();
      expect(screen.getByLabelText("Gratis")).toBeEnabled();
      expect(screen.getByLabelText(/jendela check-in/i)).toBeEnabled();
      expect(screen.getByLabelText("Izinkan lebih dari satu percobaan")).toBeEnabled();
    });

    it("leaves the policy controls enabled when not locked", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);

      expect(screen.queryByText(/terkunci karena ujian ini sudah memiliki sesi/i)).not.toBeInTheDocument();
      expect(screen.getByLabelText("Bebas")).toBeEnabled();
      expect(screen.getByLabelText(/durasi/i)).toBeEnabled();
      expect(screen.getByLabelText("Acak Soal")).toBeEnabled();
    });

    it("sends the hidden check-in and release fields untouched on save", async () => {
      mockUpdateExam.mockResolvedValue({ id: "exam-1" });
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...composableExam, requires_checkin: false }} policyLocked />);

      fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

      await waitFor(() =>
        expect(mockUpdateExam).toHaveBeenCalledWith(
          expect.objectContaining({ check_in_window_minutes: 15, grace_window_minutes: 5, result_release_at: "2026-07-02T10:00:00.000Z" }),
        ),
      );
    });

    it("keeps unsaved non-policy edits but resets policy fields when the exam is refetched after a 409", async () => {
      const { ApiError } = await import("@/lib/api");
      mockUpdateExam.mockRejectedValue(new ApiError("exam_has_sessions", "exam already has sessions", 409));
      const { rerender } = render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);

      fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Judul Belum Disimpan" } });
      fireEvent.click(screen.getByLabelText("Berurutan"));
      fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));
      await waitFor(() => expect(mockUpdateExam).toHaveBeenCalled());

      rerender(
        <ExamModal
          open={true}
          onClose={vi.fn()}
          exam={{ ...composableExam, navigation_policy: "free", duration_minutes: 45, randomize: false }}
          policyLocked
        />,
      );

      expect(screen.getByLabelText(/judul/i)).toHaveValue("Judul Belum Disimpan");
      expect(screen.getByLabelText("Bebas")).toBeChecked();
      expect(screen.getByLabelText(/durasi/i)).toHaveValue(45);
      expect(screen.getByLabelText("Acak Soal")).not.toBeChecked();
    });

    it("localizes a 409 exam_has_sessions error and reports the conflict", async () => {
      const { ApiError } = await import("@/lib/api");
      const { toast } = await import("sonner");
      const onSessionsConflict = vi.fn();
      mockUpdateExam.mockRejectedValue(new ApiError("exam_has_sessions", "exam already has sessions", 409));
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} onSessionsConflict={onSessionsConflict} />);

      fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

      await waitFor(() => expect(onSessionsConflict).toHaveBeenCalled());
      expect(toast.error).toHaveBeenCalledWith(expect.stringMatching(/sudah memiliki sesi/i));
    });
  });

  describe("whatsapp group link", () => {
    const WA = "https://chat.whatsapp.com/AbCdEf123";
    const submit = () => fireEvent.click(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i }));

    it("is pre-filled from the exam on edit", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...sampleExam, whatsapp_group_url: WA }} />);
      expect(screen.getByLabelText(/link grup whatsapp kompetisi/i)).toHaveValue(WA);
    });

    it("sends the trimmed link on update and stays editable while policy is locked", async () => {
      mockUpdateExam.mockResolvedValue({ id: "exam-1" });
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...sampleExam, format_version: 2, whatsapp_group_url: null }} policyLocked />);
      const input = screen.getByLabelText(/link grup whatsapp kompetisi/i);
      expect(input).toBeEnabled();
      fireEvent.input(input, { target: { value: `  ${WA}  ` } });
      submit();
      await waitFor(() => expect(mockUpdateExam).toHaveBeenCalledWith(expect.objectContaining({ whatsapp_group_url: WA })));
    });

    it("sends null when the field is blank", async () => {
      mockUpdateExam.mockResolvedValue({ id: "exam-1" });
      render(<ExamModal open={true} onClose={vi.fn()} exam={{ ...sampleExam, whatsapp_group_url: WA }} />);
      fireEvent.input(screen.getByLabelText(/link grup whatsapp kompetisi/i), { target: { value: "   " } });
      submit();
      await waitFor(() => expect(mockUpdateExam).toHaveBeenCalledWith(expect.objectContaining({ whatsapp_group_url: null })));
    });

    it("omits the key when the opened exam never carried the field, so an older backend value is not wiped", async () => {
      mockUpdateExam.mockResolvedValue({ id: "exam-1" });
      render(<ExamModal open={true} onClose={vi.fn()} exam={sampleExam} />);
      submit();
      await waitFor(() => expect(mockUpdateExam).toHaveBeenCalled());
      expect(mockUpdateExam.mock.calls[0][0]).not.toHaveProperty("whatsapp_group_url");
    });

    it("sends the link on create and null when left empty", async () => {
      mockCreateExam.mockResolvedValue({ id: "new-exam" });
      render(<ExamModal open={true} onClose={vi.fn()} />);
      fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Baru" } });
      fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });
      submit();
      await waitFor(() => expect(mockCreateExam).toHaveBeenCalledWith(expect.objectContaining({ whatsapp_group_url: null })));
    });

    it.each([
      ["http scheme", "http://chat.whatsapp.com/AbC"],
      ["another host", "https://example.com/AbC"],
      ["lookalike host", "https://chat.whatsapp.com.evil.com/AbC"],
      ["no invite code", "https://chat.whatsapp.com/"],
      ["explicit port", "https://chat.whatsapp.com:443/AbC"],
      ["opaque scheme", "https:chat.whatsapp.com/AbC"],
      ["userinfo", "https://x@chat.whatsapp.com/AbC"],
      ["not a url", "bukan link"],
    ])("blocks submit for %s", async (_name, value) => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={sampleExam} />);
      fireEvent.input(screen.getByLabelText(/link grup whatsapp kompetisi/i), { target: { value } });
      expect(await screen.findByRole("alert")).toHaveTextContent(/chat\.whatsapp\.com/i);
      expect(screen.getByRole("button", { name: /^simpan(?: pengaturan)?$/i })).toBeDisabled();
    });
  });
  describe("prototype layout", () => {
    const composableExam: ExamListItem = { ...sampleExam, format_version: 2, navigation_policy: "free" };

    it("titles an existing exam 'Pengaturan ujian' with a subtitle and a 'Simpan pengaturan' footer", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      expect(screen.getByRole("heading", { name: "Pengaturan ujian" })).toBeInTheDocument();
      expect(screen.getByText(/dipisahkan agar tidak tertukar/i)).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Simpan pengaturan" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Batal" })).toBeInTheDocument();
    });

    it("keeps the create title and Simpan label for a new exam", () => {
      render(<ExamModal open={true} onClose={vi.fn()} />);
      expect(screen.getByRole("heading", { name: "Buat Ujian" })).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Simpan" })).toBeInTheDocument();
    });

    it("groups the fields under the five prototype section headings", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      for (const name of ["Informasi & jadwal akses", "Aturan pengerjaan", "Akses peserta", "Hasil ujian", "Kartu & layar selesai"]) {
        expect(screen.getByRole("heading", { name })).toBeInTheDocument();
      }
    });

    it("omits the card section for a new exam", () => {
      render(<ExamModal open={true} onClose={vi.fn()} />);
      expect(screen.queryByRole("heading", { name: "Kartu & layar selesai" })).not.toBeInTheDocument();
    });

    it("describes each navigation and timer choice and keeps the title as the accessible name", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      expect(screen.getByLabelText("Berurutan")).toHaveAccessibleDescription("Section selesai tidak dapat dibuka lagi.");
      expect(screen.getByLabelText("Per section")).toHaveAccessibleDescription("Durasi diambil dari setiap Test.");
    });

    it("explains the timer choice and swaps the help for the sequential-navigation error", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      expect(screen.getByText(/timer terus berjalan ketika berpindah section/i)).toBeInTheDocument();
      fireEvent.click(screen.getByLabelText("Per section"));
      expect(screen.getByText(/timer dimulai ulang pada section berikutnya/i)).toBeInTheDocument();
      fireEvent.click(screen.getByLabelText("Bebas"));
      expect(screen.getByRole("alert")).toHaveTextContent(/memerlukan navigasi berurutan/i);
      expect(screen.queryByText(/timer dimulai ulang pada section berikutnya/i)).not.toBeInTheDocument();
    });

    it("shows the 'Yang perlu dibedakan' panel, and drops the post-session note once locked", () => {
      const { rerender } = render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      const panel = screen.getByRole("complementary");
      expect(within(panel).getByText("Yang perlu dibedakan")).toBeInTheDocument();
      expect(within(panel).getByText(/kapan sesi baru boleh dimulai/i)).toBeInTheDocument();
      expect(within(panel).getByText(/setelah ada sesi, aturan pengerjaan/i)).toBeInTheDocument();
      rerender(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} policyLocked />);
      expect(within(screen.getByRole("complementary")).queryByText(/setelah ada sesi, aturan pengerjaan/i)).not.toBeInTheDocument();
    });

    it("hides the panel below the md breakpoint via the hidden class", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      expect(screen.getByRole("complementary", { hidden: true }).className).toMatch(/(^|\s)hidden(\s|$)/);
    });

    it("puts check-in fields and the attempts checkbox under Akses peserta", () => {
      render(<ExamModal open={true} onClose={vi.fn()} exam={composableExam} />);
      const access = screen.getByRole("heading", { name: "Akses peserta" }).closest("section")!;
      expect(within(access).getByLabelText(/jendela check-in/i)).toBeInTheDocument();
      expect(within(access).getByLabelText("Izinkan lebih dari satu percobaan")).toBeInTheDocument();
      expect(within(access).getByLabelText("Tampilkan Leaderboard")).toBeInTheDocument();
    });
  });
});
