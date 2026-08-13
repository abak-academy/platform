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
  AnnouncementComposer: () => <div data-testid="announcement-composer" />,
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

  it("renders the purchase notification feed section on the default tab", () => {
    render(<NotificationsPage />);
    expect(screen.getByTestId("purchase-feed")).toHaveTextContent("Notifikasi Pembelian");
  });

  it("renders the announcement table section once its tab is selected", () => {
    render(<NotificationsPage />);

    expect(screen.queryByTestId("announcement-table")).not.toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Pengumuman" }));

    expect(screen.getByTestId("announcement-table")).toBeInTheDocument();
  });

  it("offers the header create action only on the announcements tab", () => {
    render(<NotificationsPage />);

    expect(screen.queryAllByRole("button", { name: /buat/i })).toHaveLength(0);

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Pengumuman" }));

    expect(screen.getAllByRole("button", { name: /buat/i }).length).toBeGreaterThan(0);
  });

  it("renders the announcement composer", () => {
    render(<NotificationsPage />);
    expect(screen.getByTestId("announcement-composer")).toBeInTheDocument();
  });
});
