import { describe, it, expect, vi, beforeAll } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { OrdersToolbar } from "./OrdersToolbar";
import type { AdminOrderQuery, OrderBucketCounts } from "@/lib/types";

// Radix Select drives its trigger through Pointer Events APIs that jsdom does
// not implement. Without these the listbox never opens and the option list
// cannot be asserted at all.
beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

const counts: OrderBucketCounts = {
  needs_confirm: 4,
  ready_to_ship: 7,
  shipment_failed: 1,
  in_transit: 12,
  created_this_month: 128,
  completed_this_month: 96,
  total: 1284,
};

function renderToolbar(value: AdminOrderQuery = { status: "all" }, onChange = vi.fn()) {
  render(<OrdersToolbar value={value} onChange={onChange} counts={counts} />);
  return { onChange };
}

async function openStatusMenu() {
  fireEvent.pointerDown(screen.getByTestId("orders-status-filter"), {
    button: 0,
    ctrlKey: false,
    pointerType: "mouse",
  });
  return screen.findByRole("listbox");
}

describe("OrdersToolbar status filter", () => {
  it("offers every status, including the failed-shipment filter", async () => {
    renderToolbar();
    const listbox = await openStatusMenu();

    for (const label of [
      "Semua",
      "Menunggu",
      "Dibayar",
      "Diproses",
      "Dikirim",
      "Pengiriman bermasalah",
    ]) {
      expect(
        screen.getAllByText(label).length,
        `option "${label}" is missing`,
      ).toBeGreaterThanOrEqual(1);
    }
    expect(listbox).toBeInTheDocument();
  });

  it("shows the count beside the buckets the API actually counts", async () => {
    renderToolbar();
    await openStatusMenu();

    // needs_confirm, ready_to_ship, in_transit, shipment_failed and total.
    expect(screen.getAllByText("1284").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("4").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("7").length).toBeGreaterThanOrEqual(1);
  });

  it("emits the chosen status", async () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);
    await openStatusMenu();

    fireEvent.click(screen.getByRole("option", { name: /Menunggu/ }));

    await waitFor(() =>
      expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ status: "pending" })),
    );
  });
});

describe("OrdersToolbar date mode", () => {
  it("defaults to single-date and emits the same day for from and to", async () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    expect(screen.getByTestId("orders-date-mode-single")).toHaveAttribute("aria-pressed", "true");

    fireEvent.change(document.getElementById("orders-date-single")!, { target: { value: "2026-08-02" } });

    // One calendar day is from === to; the API's upper bound is exclusive.
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: "2026-08-02", to: "2026-08-02" }),
    );
  });

  it("clearing the single date drops both bounds", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all", from: "2026-08-02", to: "2026-08-02" }, onChange);

    fireEvent.change(document.getElementById("orders-date-single")!, { target: { value: "" } });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: undefined, to: undefined }),
    );
  });

  it("range mode exposes both bounds independently", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    fireEvent.click(screen.getByTestId("orders-date-mode-range"));

    fireEvent.change(document.getElementById("orders-date-from")!, { target: { value: "2026-08-01" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ from: "2026-08-01" }));

    fireEvent.change(document.getElementById("orders-date-to")!, { target: { value: "2026-08-31" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ to: "2026-08-31" }));
  });

  // An open-ended range has no single day, so switching back must not leave a
  // `to` behind that the single input cannot show.
  it("collapses a range to its start date when switching back to single", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all", from: "2026-08-01", to: "2026-08-31" }, onChange);

    // A two-date value opens in range mode rather than silently collapsing.
    expect(screen.getByTestId("orders-date-mode-range")).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByTestId("orders-date-mode-single"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: "2026-08-01", to: "2026-08-01" }),
    );
  });
});

describe("OrdersToolbar search", () => {
  it("debounces before emitting, so a keystroke is not a request", async () => {
    vi.useFakeTimers();
    const onChange = vi.fn();
    render(<OrdersToolbar value={{ status: "all" }} onChange={onChange} counts={counts} />);

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "rani" } });
    expect(onChange).not.toHaveBeenCalled();

    vi.advanceTimersByTime(400);
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ q: "rani" }));
    vi.useRealTimers();
  });
});
