import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { TrackingModal } from "./TrackingModal";
import type { OrderTracking } from "@/lib/types";

const courierLog: OrderTracking = {
  waybill: "JP1234567890",
  courier: "JNE",
  service: "REG",
  status: "delivered",
  source: "courier",
  history: [
    { status: "delivered", occurred_at: "2026-08-03T17:30:00Z", note: "Diterima Budi" },
    { status: "in_transit", occurred_at: "2026-08-02T09:00:00Z" },
  ],
};

describe("TrackingModal", () => {
  it("leads with the waybill, since that is the number on the box", () => {
    render(<TrackingModal open onOpenChange={vi.fn()} tracking={courierLog} isLoading={false} />);
    expect(screen.getByRole("heading", { name: "JP1234567890" })).toBeTruthy();
  });

  it("translates the current position", () => {
    render(<TrackingModal open onOpenChange={vi.fn()} tracking={courierLog} isLoading={false} />);
    expect(screen.getByTestId("tracking-current-status").textContent).toBe("Diterima");
  });

  it("lists every checkpoint, newest first and translated", () => {
    render(<TrackingModal open onOpenChange={vi.fn()} tracking={courierLog} isLoading={false} />);
    const entries = within(screen.getByTestId("tracking-history")).getAllByRole("listitem");
    expect(entries).toHaveLength(2);
    expect(entries[0].textContent).toContain("Diterima");
    expect(entries[1].textContent).toContain("Dalam perjalanan");
  });

  // A history we assembled from our own webhooks is thinner than the
  // courier's. Presenting it unlabelled would read as "the courier only
  // scanned this twice", which is a different and wrong claim.
  it("says when the history is ours rather than the courier's", () => {
    render(
      <TrackingModal
        open
        onOpenChange={vi.fn()}
        tracking={{ ...courierLog, source: "local" }}
        isLoading={false}
      />,
    );
    expect(screen.getByText(/riwayat kami/i)).toBeTruthy();
  });

  it("credits the courier's scan log when that is what answered", () => {
    render(<TrackingModal open onOpenChange={vi.fn()} tracking={courierLog} isLoading={false} />);
    expect(screen.getByText("Dari catatan kurir")).toBeTruthy();
  });

  it("explains an empty log instead of showing a blank panel", () => {
    render(
      <TrackingModal
        open
        onOpenChange={vi.fn()}
        tracking={{ ...courierLog, status: "confirmed", history: [] }}
        isLoading={false}
      />,
    );
    expect(screen.getByText("Kurir belum mengirim pembaruan untuk resi ini.")).toBeTruthy();
    expect(screen.queryByTestId("tracking-history")).toBeNull();
  });

  it("marks a dead parcel so the dialog does not read as normal progress", () => {
    render(
      <TrackingModal
        open
        onOpenChange={vi.fn()}
        tracking={{ ...courierLog, status: "courier_not_found" }}
        isLoading={false}
      />,
    );
    const status = screen.getByTestId("tracking-current-status");
    expect(status.textContent).toBe("Kurir tidak tersedia — dibatalkan");
    expect(status.className).toContain("text-danger");
  });

  it("shows an error in place of the journey when tracking cannot be loaded", () => {
    render(
      <TrackingModal
        open
        onOpenChange={vi.fn()}
        tracking={null}
        isLoading={false}
        error="Gagal memuat pelacakan."
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe("Gagal memuat pelacakan.");
    expect(screen.queryByTestId("tracking-history")).toBeNull();
  });
});
