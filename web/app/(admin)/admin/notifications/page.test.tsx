import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import NotificationsPage from "./page";

vi.mock("@/components/admin/PurchaseNotificationFeed", () => ({
  PurchaseNotificationFeed: () => <div data-testid="purchase-feed">Notifikasi Pembelian</div>,
}));

vi.mock("@/components/admin/AnnouncementTable", () => ({
  AnnouncementTable: ({ onCreateClick, onEdit }: { onCreateClick: () => void; onEdit: () => void }) => (
    <div data-testid="announcement-table">
      <button onClick={onCreateClick} data-testid="create-btn">Buat</button>
      <button onClick={() => onEdit()} data-testid="edit-btn">Edit</button>
    </div>
  ),
}));

vi.mock("@/components/admin/AnnouncementComposer", () => ({
  AnnouncementComposer: ({ inline }: { inline?: boolean }) => <div data-testid={inline ? "announcement-composer-inline" : "announcement-composer"} />,
  RECIPIENT_LABEL: { all: "notification_all_users" },
  TYPE_LABEL: { announcement: "notification_announcement" },
}));

vi.mock("@/lib/hooks/admin-announcements", () => ({
  useAdminAnnouncements: () => ({ data: [
    { id: "a1", title: "Libur sekolah", message: "Isi pengumuman", type: "announcement", recipients: "all", status: "sent", sent_at: "2026-11-20T08:00:00+07:00" },
    { id: "a2", title: "Draf rahasia", message: "x", type: "announcement", recipients: "all", status: "draft", sent_at: null },
  ] }),
}));

describe("NotificationsPage", () => {
  it("renders the page-level AdminPageHeader with title", () => {
    render(<NotificationsPage />);
    expect(screen.getByRole("heading", { level: 1, name: "Notifikasi" })).toBeInTheDocument();
  });

  // The page used to stack a second AdminPageHeader from AnnouncementTable
  // directly beneath its own.
  it("renders exactly one page heading", () => {
    render(<NotificationsPage />);
    expect(screen.getAllByRole("heading", { level: 1 })).toHaveLength(1);
  });

  it("opens on Pengumuman with the inline composer, the sent list, and the table", () => {
    render(<NotificationsPage />);
    expect(screen.getByTestId("announcement-composer-inline")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Terkirim" })).toBeInTheDocument();
    expect(screen.getByText("Libur sekolah")).toBeInTheDocument();
    expect(screen.queryByText("Draf rahasia")).not.toBeInTheDocument();
    expect(screen.getByTestId("announcement-table")).toBeInTheDocument();
    expect(screen.queryByTestId("purchase-feed")).not.toBeInTheDocument();
  });

  it("keeps the purchase notification feed on its own tab", () => {
    render(<NotificationsPage />);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Kotak masuk" }));
    expect(screen.getByTestId("purchase-feed")).toHaveTextContent("Notifikasi Pembelian");
  });

  it("renders the announcement composer", () => {
    render(<NotificationsPage />);
    expect(screen.getByTestId("announcement-composer")).toBeInTheDocument();
  });
});
