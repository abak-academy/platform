import { describe, it, expect } from "vitest";
import { buildTimeline } from "./timeline";
import type { Order } from "@/lib/types";

// The label lookup is not what is under test here; identity keeps the
// assertions readable as the step keys they actually are.
const t = (k: string) => k;

const completedOrder = {
  id: "o1",
  status: "completed",
  created_at: "2026-07-27T07:01:00Z",
  checked_out_at: "2026-08-04T02:39:00Z",
  paid_at: "2026-08-04T02:40:00Z",
  shipped_at: "2026-08-04T06:31:00Z",
  completed_at: "2026-08-04T07:20:00Z",
} as unknown as Order;

const keys = (o: Order) => buildTimeline(o, t).map((s) => s.key);

describe("buildTimeline", () => {
  // The old filter always kept completed and cancelled, so a finished order
  // still listed "Pesanan dibatalkan" in grey — a step that can no longer
  // happen, presented as though it were still pending.
  it("omits a step the order never reached", () => {
    expect(keys(completedOrder)).toEqual(["created", "checkout", "paid", "shipped", "completed"]);
  });

  it("stops at the step the order is currently on", () => {
    const inFlight = { ...completedOrder, status: "shipped", completed_at: undefined } as unknown as Order;
    expect(keys(inFlight)).toEqual(["created", "checkout", "paid", "shipped"]);
  });

  // completed and cancelled are terminal alternatives; listing both was the
  // clearest symptom of the old filter.
  it("shows cancellation instead of completion when that is what happened", () => {
    const cancelled = {
      ...completedOrder,
      status: "cancelled",
      completed_at: undefined,
      cancelled_at: "2026-08-04T08:00:00Z",
    } as unknown as Order;
    const k = keys(cancelled);
    expect(k).toContain("cancelled");
    expect(k).not.toContain("completed");
  });

  it("marks the cancellation so it can be rendered as a failure, not a tick", () => {
    const cancelled = { ...completedOrder, status: "cancelled", completed_at: undefined } as unknown as Order;
    expect(buildTimeline(cancelled, t).find((s) => s.key === "cancelled")?.cancelled).toBe(true);
  });

  it("keeps a brand-new order down to the one step it has", () => {
    const fresh = { id: "o", status: "cart", created_at: "2026-08-04T07:00:00Z" } as unknown as Order;
    expect(keys(fresh)).toEqual(["created"]);
  });
});
