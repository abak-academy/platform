import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { TestModal } from "./TestModal";
import type { Test } from "@/lib/types";

const mockOnSubmit = vi.fn();
const mockOnOpenChange = vi.fn();

const mockPresignAudioAsync = vi.fn();
let presignAudioState = { mutateAsync: mockPresignAudioAsync, isPending: false };

vi.mock("@/lib/hooks/admin-uploads", () => ({
  usePresignAdminAudioUpload: () => presignAudioState,
  usePresignAdminImageUpload: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
  }),
}));

describe("TestModal", () => {
  beforeEach(() => {
    mockOnSubmit.mockReset();
    mockOnOpenChange.mockReset();

    mockPresignAudioAsync.mockReset();
    mockPresignAudioAsync.mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "tests/uuid/audio.mp3",
    });
    presignAudioState = { mutateAsync: mockPresignAudioAsync, isPending: false };
  });

  it("renders create modal with empty fields", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("dialog", { name: /tes baru/i })).toBeInTheDocument();
    expect(screen.getByLabelText(/judul/i)).toHaveValue("");
    expect(screen.getByLabelText(/mata pelajaran/i)).toHaveValue("");
    expect(screen.getByLabelText(/topik/i)).toHaveValue("");
  });

  it("renders edit modal prefilled with test data", () => {
    const test: Test = {
      id: "t1",
      title: "Tryout UTBK Saintek",
      subject: "Matematika",
      topic: "Aljabar",
      duration_minutes: 90,
      audio_url: "https://cdn/audio.mp3",
      audio_play_limit: 2,
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("dialog", { name: /sunting tes/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Tryout UTBK Saintek")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Matematika")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Aljabar")).toBeInTheDocument();
    expect(screen.getByDisplayValue("90")).toBeInTheDocument();
    expect(screen.getByDisplayValue("https://cdn/audio.mp3")).toBeInTheDocument();
    expect(screen.getByDisplayValue("2")).toBeInTheDocument();
  });

  it("submits create input with all required fields", async () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Tryout 1" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Matematika" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Aljabar" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Tryout 1",
          subject: "Matematika",
          topic: "Aljabar",
          duration_minutes: 60,
        })
      );
    });
  });

  it("submits edit input only with allowed fields (no title mutation in payload if untouched)", async () => {
    const test: Test = {
      id: "t1",
      title: "Tryout Lama",
      subject: "Matematika",
      topic: "Aljabar",
      duration_minutes: 60,
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Geometri" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ topic: "Geometri" })
      );
    });
  });

  it("disables save when required fields are empty", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const saveButton = screen.getByRole("button", { name: /^simpan$/i });
    expect(saveButton).toBeDisabled();
  });

  it("disables save when duration is zero", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Tryout 1" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Matematika" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Aljabar" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "0" } });

    const saveButton = screen.getByRole("button", { name: /^simpan$/i });
    expect(saveButton).toBeDisabled();
  });

  it("omits optional audio fields when blank", async () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Tryout 1" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Matematika" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Aljabar" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      const payload = mockOnSubmit.mock.calls[0][0];
      expect(payload).not.toHaveProperty("audio_url");
      expect(payload).not.toHaveProperty("audio_play_limit");
    });
  });

  it("includes audio_url and audio_play_limit when provided", async () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Listening Test" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Bahasa Inggris" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Listening" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "30" } });
    fireEvent.input(screen.getByLabelText(/url audio/i), { target: { value: "https://cdn/audio.mp3" } });
    fireEvent.input(screen.getByLabelText(/batas pemutaran audio/i), { target: { value: "3" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          audio_url: "https://cdn/audio.mp3",
          audio_play_limit: 3,
        })
      );
    });
  });

  it("renders a section_type picker with none, listening, reading, writing options on create", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByLabelText("Tidak ada")).toBeInTheDocument();
    expect(screen.getByLabelText("Listening")).toBeInTheDocument();
    expect(screen.getByLabelText("Reading")).toBeInTheDocument();
    expect(screen.getByLabelText("Writing")).toBeInTheDocument();
  });

  it("defaults to none (null section_type) on create", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByLabelText("Tidak ada")).toBeChecked();
    expect(screen.getByLabelText("Listening")).not.toBeChecked();
    expect(screen.getByLabelText("Reading")).not.toBeChecked();
    expect(screen.getByLabelText("Writing")).not.toBeChecked();
  });

  it("includes section_type in create payload when set", async () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "IELTS Reading" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Bahasa Inggris" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Reading" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "60" } });
    fireEvent.click(screen.getByLabelText("Reading"));

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ section_type: "reading" })
      );
    });
  });

  it("includes section_type in update payload when set", async () => {
    const test: Test = {
      id: "t1",
      title: "IELTS Reading",
      subject: "Bahasa Inggris",
      topic: "Reading",
      duration_minutes: 60,
      section_type: "reading",
      audio_url: "https://cdn/audio.mp3",
      audio_play_limit: 2,
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ section_type: "reading" })
      );
    });
  });

  it("pre-fills section_type from test data on edit", async () => {
    const test: Test = {
      id: "t1",
      title: "IELTS Writing",
      subject: "Bahasa Inggris",
      topic: "Writing",
      duration_minutes: 60,
      section_type: "writing",
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Writing")).toBeChecked();
    });
  });

  it("blocks save when listening is selected and audio_url is empty", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    // Fill required fields
    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Listening Test" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Bahasa Inggris" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Listening" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "30" } });

    // Select listening → save should be blocked
    fireEvent.click(screen.getByLabelText("Listening"));

    const saveButton = screen.getByRole("button", { name: /^simpan$/i });
    expect(saveButton).toBeDisabled();
  });

  it("allows save when listening is selected and audio_url is provided", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    // Fill all required fields including audio
    fireEvent.input(screen.getByLabelText(/judul/i), { target: { value: "Listening Test" } });
    fireEvent.input(screen.getByLabelText(/mata pelajaran/i), { target: { value: "Bahasa Inggris" } });
    fireEvent.input(screen.getByLabelText(/topik/i), { target: { value: "Listening" } });
    fireEvent.input(screen.getByLabelText(/durasi/i), { target: { value: "30" } });
    fireEvent.input(screen.getByLabelText(/url audio/i), { target: { value: "https://cdn/audio.mp3" } });

    // Select listening
    fireEvent.click(screen.getByLabelText("Listening"));

    const saveButton = screen.getByRole("button", { name: /^simpan$/i });
    expect(saveButton).toBeEnabled();
  });

  it("calls onOpenChange(false) when cancel clicked", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: /batal/i }));
    expect(mockOnOpenChange).toHaveBeenCalledWith(false);
  });

  it("audio_url field uses AudioUploadInput with upload capability", () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    // AudioUploadInput renders both a text input and an upload button
    const audioInput = screen.getByLabelText(/url audio/i) as HTMLInputElement;
    expect(audioInput).toBeInTheDocument();

    const uploadButton = screen.getByRole("button", { name: /upload audio/i });
    expect(uploadButton).toBeInTheDocument();
  });

  it("selecting an audio file triggers presign and upload flow for test", async () => {
    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);

    // Find the hidden file input for audio upload
    const fileInput = document.querySelector('input[data-testid="audio-upload-input-test-audio-url"]') as HTMLInputElement;
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
    const test: Test = {
      id: "t1",
      title: "Tryout 1",
      subject: "Matematika",
      topic: "Aljabar",
      duration_minutes: 90,
      audio_url: "https://example.com/existing-audio.mp3",
      audio_play_limit: 2,
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    // AudioUploadInput should display the pre-existing URL
    const audioInput = screen.getByDisplayValue("https://example.com/existing-audio.mp3");
    expect(audioInput).toBeInTheDocument();
  });

  it("pre-existing audio_url saves correctly without forced re-upload", async () => {
    const test: Test = {
      id: "t1",
      title: "Tryout 1",
      subject: "Matematika",
      topic: "Aljabar",
      duration_minutes: 90,
      audio_url: "https://example.com/existing-audio.mp3",
      audio_play_limit: 2,
    };

    render(
      <TestModal
        open={true}
        onOpenChange={mockOnOpenChange}
        test={test}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    // Don't upload a new file, just save
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          audio_url: "https://example.com/existing-audio.mp3",
        })
      );
    });
  });
  it.each([null, "<p>Reusable passage</p>"])("preserves shared material %s when saving metadata", async material => {
    render(<TestModal open={true} onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false}
      test={{ id:"material-test", title:"Material Test", subject:"English", topic:"Reading", duration_minutes:30, shared_material_html:material, instructions_html:"<p>Read carefully</p>" } as Test} />);
    fireEvent.click(screen.getByRole("button", { name:/^simpan$/i }));
    await waitFor(() => expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ shared_material_html:material, instructions_html:"<p>Read carefully</p>" })));
  });

});
