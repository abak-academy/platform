import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, act } from "@testing-library/react";
import { AnnouncementComposer } from "./AnnouncementComposer";

const mockCreateMutate = vi.fn();
const mockUpdateMutate = vi.fn();

vi.mock("@/lib/hooks/admin-announcements", () => ({
  useCreateAnnouncement: () => ({ mutate: mockCreateMutate, isPending: false }),
  useUpdateAnnouncement: () => ({ mutate: mockUpdateMutate, isPending: false }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const draftAnnouncement = {
  id: "ann-1",
  title: "Existing Draft",
  message: "Existing message",
  type: "announcement",
  recipients: "students",
  status: "draft",
  scheduled_at: null,
  sent_at: null,
  recipient_count: null,
  created_by: "u1",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
};

describe("AnnouncementComposer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const succeed = (_input: unknown, opts?: { onSuccess?: () => void }) => opts?.onSuccess?.();
    mockCreateMutate.mockImplementation(succeed);
    mockUpdateMutate.mockImplementation(succeed);
  });

  it("renders dialog with form fields when open", () => {
    render(<AnnouncementComposer open={true} onOpenChange={vi.fn()} />);
    expect(screen.getByText(/notification_title/i)).toBeInTheDocument();
    expect(screen.getByText(/notification_message/i)).toBeInTheDocument();
    expect(screen.getByText(/notification_type/i)).toBeInTheDocument();
    expect(screen.getByText(/notification_recipients/i)).toBeInTheDocument();
    expect(screen.getByText(/notification_scheduled_at/i)).toBeInTheDocument();
  });

  it("renders inline as a page card and clears the form after sending", () => {
    const onOpenChange = vi.fn();
    render(<AnnouncementComposer inline open onOpenChange={onOpenChange} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "notification_compose_title" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "cancel" })).not.toBeInTheDocument();
    const title = screen.getByPlaceholderText(/notification_title/i) as HTMLInputElement;
    fireEvent.change(title, { target: { value: "Libur" } });
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(mockCreateMutate).toHaveBeenCalledWith(expect.objectContaining({ title: "Libur", status: "sent" }), expect.anything());
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(title.value).toBe("");
  });

  it("does not render when closed", () => {
    render(<AnnouncementComposer open={false} onOpenChange={vi.fn()} />);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("calls create mutation with status=draft on Save Draft", () => {
    const onOpenChange = vi.fn();
    render(<AnnouncementComposer open={true} onOpenChange={onOpenChange} />);

    fireEvent.change(screen.getByPlaceholderText(/notification_title/i), {
      target: { value: "Test Title" },
    });
    fireEvent.change(screen.getByPlaceholderText(/notification_message/i), {
      target: { value: "Test message content" },
    });

    fireEvent.click(screen.getByRole("button", { name: /save.*notification_draft/i }));

    expect(mockCreateMutate).toHaveBeenCalledWith({
      title: "Test Title",
      message: "Test message content",
      type: "announcement",
      recipients: "all",
      status: "draft",
    }, expect.anything());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("calls create mutation with status=scheduled on Schedule when scheduled_at is set", () => {
    render(<AnnouncementComposer open={true} onOpenChange={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText(/notification_title/i), {
      target: { value: "Scheduled Test" },
    });
    fireEvent.change(screen.getByPlaceholderText(/notification_message/i), {
      target: { value: "Scheduled message" },
    });

    // Set scheduled_at via the datetime-local input
    const dateInput = screen.getByTestId("scheduled-at-input");
    fireEvent.change(dateInput, { target: { value: "2026-07-10T10:00" } });

    fireEvent.click(screen.getByRole("button", { name: /notification_schedule/i }));

    expect(mockCreateMutate).toHaveBeenCalledWith({
      title: "Scheduled Test",
      message: "Scheduled message",
      type: "announcement",
      recipients: "all",
      status: "scheduled",
      scheduled_at: expect.stringContaining("2026"),
    }, expect.anything());
  });

  it("calls create mutation with status=sent on Send Now", () => {
    const onOpenChange = vi.fn();
    render(<AnnouncementComposer open={true} onOpenChange={onOpenChange} />);

    fireEvent.change(screen.getByPlaceholderText(/notification_title/i), {
      target: { value: "Send Now Test" },
    });
    fireEvent.change(screen.getByPlaceholderText(/notification_message/i), {
      target: { value: "Send immediately" },
    });

    fireEvent.click(screen.getByRole("button", { name: /send/i }));

    expect(mockCreateMutate).toHaveBeenCalledWith({
      title: "Send Now Test",
      message: "Send immediately",
      type: "announcement",
      recipients: "all",
      status: "sent",
    }, expect.anything());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables Schedule button when scheduled_at is empty", () => {
    render(<AnnouncementComposer open={true} onOpenChange={vi.fn()} />);

    const scheduleBtn = screen.getByRole("button", { name: /notification_schedule/i });
    expect(scheduleBtn).toBeDisabled();
  });

  it("pre-fills form and calls update mutation in edit mode", () => {
    const onOpenChange = vi.fn();
    render(
      <AnnouncementComposer
        open={true}
        onOpenChange={onOpenChange}
        announcement={draftAnnouncement}
      />
    );

    // Check pre-filled value
    expect(screen.getByDisplayValue("Existing Draft")).toBeInTheDocument();

    // Click Save Draft
    fireEvent.click(screen.getByRole("button", { name: /save.*notification_draft/i }));

    expect(mockUpdateMutate).toHaveBeenCalledWith({
      id: "ann-1",
      input: {
        title: "Existing Draft",
        message: "Existing message",
        type: "announcement",
        recipients: "students",
      },
    }, expect.anything());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("keeps the typed text inline when the server rejects, clears it only on success", () => {
    mockCreateMutate.mockImplementation(() => {});
    render(<AnnouncementComposer inline open onOpenChange={vi.fn()} />);
    const title = screen.getByPlaceholderText(/notification_title/i) as HTMLInputElement;
    fireEvent.change(title, { target: { value: "Libur" } });
    fireEvent.click(screen.getByRole("button", { name: /send/i }));
    expect(title.value).toBe("Libur");
    const { onSuccess } = mockCreateMutate.mock.calls[0][1];
    act(() => onSuccess());
    expect(title.value).toBe("");
  });
});
