import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { PurchaseNotificationFeed } from "./PurchaseNotificationFeed";

const mockMutate = vi.fn();
const mockFetchNextPage = vi.fn();
const mockUseAdminNotifications = vi.fn();
const mockUseMarkNotificationRead = vi.fn(() => ({
  mutate: mockMutate,
  isPending: false,
}));

vi.mock("@/lib/hooks/admin-notifications", () => ({
  useAdminNotifications: (...args: Parameters<typeof mockUseAdminNotifications>) =>
    mockUseAdminNotifications(...args),
  useMarkNotificationRead: () => mockUseMarkNotificationRead(),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

const sampleNotif = {
  id: "notif-1",
  type: "order_confirmed",
  order_id: "ord-1",
  student_name: "Budi Santoso",
  amount: 150000,
  created_at: "2026-07-05T10:00:00Z",
  read: false,
};

const sampleNotifRead = {
  ...sampleNotif,
  id: "notif-2",
  student_name: "Siti Rahma",
  order_id: "ord-2",
  read: true,
};

function buildQueryResult(overrides: Record<string, unknown> = {}) {
  return {
    items: [sampleNotif, sampleNotifRead],
    isLoading: false,
    isError: false,
    error: null,
    hasNextPage: true,
    isFetchingNextPage: false,
    fetchNextPage: mockFetchNextPage,
    refetch: vi.fn(),
    ...overrides,
  };
}

describe("PurchaseNotificationFeed", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders notification list with student names and order ids", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    expect(screen.getByText("Budi Santoso")).toBeInTheDocument();
    expect(screen.getByText("Siti Rahma")).toBeInTheDocument();
    expect(screen.getByText(/ord-1/)).toBeInTheDocument();
    expect(screen.getByText(/ord-2/)).toBeInTheDocument();
  });

  it("renders the full order id rather than a truncated prefix", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({
        items: [{ ...sampleNotif, order_id: "3f2b1c9e-77aa-4d1f-9c33-8b0a5e6d7f21" }],
      })
    );
    render(<PurchaseNotificationFeed />);

    expect(
      screen.getByText(/3f2b1c9e-77aa-4d1f-9c33-8b0a5e6d7f21/)
    ).toBeInTheDocument();
  });

  it("renders formatted amount for each row", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    expect(screen.getAllByText("Rp150.000").length).toBe(2);
  });

  // The producer used to multiply by 100 before storing; the row must render the
  // amount it was given, in rupiah, with no further scaling.
  it("renders the amount as plain rupiah without rescaling", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [{ ...sampleNotif, amount: 60000 }] })
    );
    render(<PurchaseNotificationFeed />);

    expect(screen.getByText("Rp60.000")).toBeInTheDocument();
    expect(screen.queryByText("Rp6.000.000")).not.toBeInTheDocument();
  });

  it("shows Load More button when another page is available", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult({ hasNextPage: true }));
    render(<PurchaseNotificationFeed />);

    expect(screen.getByText("notification_load_more")).toBeInTheDocument();
  });

  it("hides Load More button when there is no further page", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult({ hasNextPage: false }));
    render(<PurchaseNotificationFeed />);

    expect(screen.queryByText("notification_load_more")).not.toBeInTheDocument();
  });

  // The old feed seeded hasMore=true, so the button rendered over an empty feed
  // and did nothing when pressed.
  it("hides Load More button when the feed is empty", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [], hasNextPage: false })
    );
    render(<PurchaseNotificationFeed />);

    expect(screen.queryByText("notification_load_more")).not.toBeInTheDocument();
  });

  it("requests the next page when Load More is pressed", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult({ hasNextPage: true }));
    render(<PurchaseNotificationFeed />);

    fireEvent.click(screen.getByText("notification_load_more"));
    expect(mockFetchNextPage).toHaveBeenCalled();
  });

  it("shows loading placeholders, not the empty state, while the first page loads", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [], isLoading: true, hasNextPage: false })
    );
    const { container } = render(<PurchaseNotificationFeed />);

    expect(screen.queryByText("notification_inbox_empty")).not.toBeInTheDocument();
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0);
  });

  it("shows unread-only toggle and activates on click", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    const toggle = screen.getByText("notification_unread_only").closest("button")!;
    expect(toggle).toHaveAttribute("aria-pressed", "false");

    fireEvent.click(toggle);

    expect(mockUseAdminNotifications).toHaveBeenLastCalledWith({ unreadOnly: true });
  });

  it("shows the unread count on the unread filter", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    // One of the two sample rows is unread.
    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("calls mark-read mutation when an unread notification is clicked", async () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    fireEvent.click(screen.getByText("Budi Santoso"));

    await waitFor(() => {
      expect(mockMutate).toHaveBeenCalledWith("notif-1");
    });
  });

  it("does not re-mark a notification that is already read", () => {
    mockUseAdminNotifications.mockReturnValue(buildQueryResult());
    render(<PurchaseNotificationFeed />);

    fireEvent.click(screen.getByText("Siti Rahma"));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it("shows empty state when no notifications", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [], hasNextPage: false })
    );
    render(<PurchaseNotificationFeed />);

    expect(screen.getByText("notification_inbox_empty")).toBeInTheDocument();
  });

  it("shows a read-specific empty state under the unread filter", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [], hasNextPage: false })
    );
    render(<PurchaseNotificationFeed />);

    fireEvent.click(screen.getByText("notification_unread_only").closest("button")!);

    expect(screen.getByText("notification_inbox_empty_unread")).toBeInTheDocument();
  });

  it("shows an error state when the feed fails to load", () => {
    mockUseAdminNotifications.mockReturnValue(
      buildQueryResult({ items: [], isError: true, hasNextPage: false })
    );
    render(<PurchaseNotificationFeed />);

    expect(screen.getByText("notification_inbox_failed")).toBeInTheDocument();
    expect(screen.queryByText("notification_inbox_empty")).not.toBeInTheDocument();
  });
});
