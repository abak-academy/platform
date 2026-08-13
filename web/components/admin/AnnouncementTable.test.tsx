import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AnnouncementTable } from "./AnnouncementTable";

const mockUseAdminAnnouncements = vi.fn();
const mockDeleteMutate = vi.fn();
const mockSendMutate = vi.fn();

vi.mock("@/lib/hooks/admin-announcements", () => ({
  useAdminAnnouncements: (...args: unknown[]) => mockUseAdminAnnouncements(...args),
  useDeleteAnnouncement: () => ({ mutate: mockDeleteMutate, isPending: false }),
  useSendAnnouncement: () => ({ mutate: mockSendMutate, isPending: false }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const draftAnnouncement = {
  id: "ann-draft-1",
  title: "Draft Title",
  message: "Draft message",
  type: "announcement",
  recipients: "all",
  status: "draft",
  scheduled_at: null,
  sent_at: null,
  recipient_count: null,
  created_by: "u1",
  created_at: "2026-07-01T00:00:00Z",
  updated_at: "2026-07-01T00:00:00Z",
};

const scheduledAnnouncement = {
  ...draftAnnouncement,
  id: "ann-sched-1",
  title: "Scheduled Title",
  type: "promo",
  recipients: "students",
  status: "scheduled",
  scheduled_at: "2026-07-10T00:00:00Z",
};

const sentAnnouncement = {
  ...draftAnnouncement,
  id: "ann-sent-1",
  title: "Sent Title",
  type: "exam",
  recipients: "admins",
  status: "sent",
  sent_at: "2026-07-05T00:00:00Z",
  recipient_count: 42,
};

const allAnnouncements = [draftAnnouncement, scheduledAnnouncement, sentAnnouncement];

function mockData(data: typeof allAnnouncements) {
  mockUseAdminAnnouncements.mockReturnValue({ data, isLoading: false });
}

describe("AnnouncementTable", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders stat cards with correct counts", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    // Stat card labels from mocked t are the keys
    // "tab_all" => total=3, "notification_sent" => 1, "notification_scheduled" => 1, "notification_draft" => 1
    expect(screen.getByText("3")).toBeInTheDocument();
    // Three "1"s appear: sent, scheduled, draft
    expect(screen.getAllByText("1")).toHaveLength(3);
  });

  it("shows all announcements in 'all' tab by default", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.getByText("Draft Title")).toBeInTheDocument();
    expect(screen.getByText("Scheduled Title")).toBeInTheDocument();
    expect(screen.getByText("Sent Title")).toBeInTheDocument();
  });

  it("filters to sent announcements when sent tab is clicked", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    // Radix TabsTrigger uses onMouseDown, not onClick
    fireEvent.mouseDown(screen.getByRole("tab", { name: /notification_sent/i }));

    expect(screen.getByText("Sent Title")).toBeInTheDocument();
    expect(screen.queryByText("Draft Title")).not.toBeInTheDocument();
    expect(screen.queryByText("Scheduled Title")).not.toBeInTheDocument();
  });

  it("filters to scheduled announcements when scheduled tab is clicked", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: /notification_scheduled/i }));

    expect(screen.getByText("Scheduled Title")).toBeInTheDocument();
    expect(screen.queryByText("Draft Title")).not.toBeInTheDocument();
    expect(screen.queryByText("Sent Title")).not.toBeInTheDocument();
  });

  it("filters to draft announcements when draft tab is clicked", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: /notification_draft/i }));

    expect(screen.getByText("Draft Title")).toBeInTheDocument();
    expect(screen.queryByText("Scheduled Title")).not.toBeInTheDocument();
    expect(screen.queryByText("Sent Title")).not.toBeInTheDocument();
  });

  it("shows recipient_count for sent announcements", () => {
    mockData([sentAnnouncement]);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.getByText("42")).toBeInTheDocument();
  });

  it("shows dash for recipient_count when null (draft)", () => {
    mockData([draftAnnouncement]);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    // Draft rows show "—" in both time and recipient_count columns (2 total)
    expect(screen.getAllByText("—")).toHaveLength(2);
  });

  it("hides dropdown menu for sent announcements", () => {
    mockData([sentAnnouncement]);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    // Sent rows show a sent-row-actions indicator (dash)
    expect(screen.getByTestId("row-actions-sent")).toBeInTheDocument();
    // No dropdown triggers for sent rows
    expect(screen.queryByTestId("row-actions-dropdown")).not.toBeInTheDocument();
  });

  it("shows dropdown menu for draft announcements", () => {
    mockData([draftAnnouncement]);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    // Draft row has a dropdown trigger
    expect(screen.getByTestId("row-actions-dropdown")).toBeInTheDocument();
    // No sent-row dash
    expect(screen.queryByTestId("row-actions-sent")).not.toBeInTheDocument();
  });

  // The primary create action now lives on the page header; the table keeps one
  // only as the empty-state call to action.
  it("calls onCreateClick from the empty-state create button", () => {
    const onCreateClick = vi.fn();
    mockData([]);
    render(<AnnouncementTable onCreateClick={onCreateClick} onEdit={vi.fn()} />);

    fireEvent.click(screen.getByRole("button", { name: /create/i }));
    expect(onCreateClick).toHaveBeenCalledOnce();
  });

  it("does not render its own page header", () => {
    mockData(allAnnouncements);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.queryByRole("heading", { level: 1 })).not.toBeInTheDocument();
  });

  it("shows an empty state rather than a loading message when there are no announcements", () => {
    mockData([]);
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.getByText("notification_announcements_empty")).toBeInTheDocument();
    expect(screen.queryByText(/sys_loading_data/i)).not.toBeInTheDocument();
  });

  it("distinguishes an empty filtered tab from an empty feed", () => {
    mockData(allAnnouncements.filter((a) => a.status === "sent"));
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    fireEvent.mouseDown(screen.getByRole("tab", { name: /notification_draft/i }));

    expect(
      screen.getByText(/notification_announcements_empty_filtered/i)
    ).toBeInTheDocument();
  });

  it("shows loading state", () => {
    mockUseAdminAnnouncements.mockReturnValue({ data: [], isLoading: true });
    render(<AnnouncementTable onCreateClick={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.getByText(/sys_loading/i)).toBeInTheDocument();
  });
});
