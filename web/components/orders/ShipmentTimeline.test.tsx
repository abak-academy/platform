import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ShipmentTimeline } from "./ShipmentTimeline";
import type { OrderShipmentEvent } from "@/lib/types";

const events: OrderShipmentEvent[] = [
  {
    id: "e1",
    order_id: "o1",
    status: "confirmed",
    occurred_at: "2026-07-20T01:00:00Z",
    created_at: "2026-07-20T01:00:05Z",
  },
  {
    id: "e2",
    order_id: "o1",
    status: "picked_up",
    occurred_at: "2026-07-22T01:00:00Z",
    created_at: "2026-07-22T01:00:05Z",
  },
  {
    id: "e3",
    order_id: "o1",
    status: "delivered",
    occurred_at: "2026-07-21T01:00:00Z",
    created_at: "2026-07-21T01:00:05Z",
  },
];

describe("ShipmentTimeline", () => {
  it("renders three events newest-first by occurred_at, not insertion order", () => {
    render(<ShipmentTimeline events={events} />);
    const rendered = screen.getAllByTestId("shipment-event-status").map((el) => el.textContent);
    expect(rendered).toEqual(["picked_up", "delivered", "confirmed"]);
  });

  it("renders no timeline block at all — heading absent — when there are no events", () => {
    const { container } = render(<ShipmentTimeline events={[]} />);
    expect(screen.queryByText("Riwayat Pengiriman")).toBeNull();
    expect(container.firstChild).toBeNull();
  });

  it("renders no timeline block when events is undefined", () => {
    const { container } = render(<ShipmentTimeline events={undefined} />);
    expect(container.firstChild).toBeNull();
  });
});
