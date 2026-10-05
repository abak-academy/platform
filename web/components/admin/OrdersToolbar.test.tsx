import { describe, it, expect, vi, beforeAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act, within } from "@testing-library/react";
import { OrdersToolbar } from "./OrdersToolbar";
import { useUIStore } from "@/stores/ui";
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
  completed: 23,
  payment_expired: 5,
  cancelled: 6,
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

afterEach(() => act(() => useUIStore.setState({ lang: "id" })));

const filterCases = [
  ["all", "Semua", "All", 1284],
  ["pending", "Menunggu pembayaran", "Awaiting payment", 4],
  ["ready_to_ship", "Siap kirim", "Ready to ship", 7],
  ["shipment_failed", "Pengiriman bermasalah", "Delivery problem", 1],
  ["in_transit", "Dalam pengiriman", "In transit", 12],
  ["completed", "Selesai", "Completed", 23],
  ["payment_expired", "Kedaluwarsa", "Payment expired", 5],
  ["cancelled", "Dibatalkan", "Cancelled", 6],
] as const;

describe("OrdersToolbar status filter", () => {
  it.each(["id", "en"] as const)("offers exactly eight ordered options with counts in %s", async (lang) => {
    useUIStore.setState({ lang });
    renderToolbar();
    const listbox = await openStatusMenu();
    expect(within(listbox).getAllByRole("option").map((option) => option.textContent))
      .toEqual(filterCases.map(([, id, en, count]) => `${lang === "id" ? id : en}${count}`));
  });

  it.each(filterCases)("selecting %s clears the other filter dimension", async (option, label) => {
    const onChange = vi.fn();
    const isQueue = ["ready_to_ship", "shipment_failed", "in_transit"].includes(option);
    renderToolbar(isQueue ? { status: "pending" } : { status: "all", queue: "shipment_failed" }, onChange);
    await openStatusMenu();
    fireEvent.click(screen.getByRole("option", { name: new RegExp(label) }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining(
      isQueue ? { status: "all", queue: option } : { status: option, queue: undefined },
    ));
  });

  it("shows the queue-selected filter as the trigger's selected value", () => {
    renderToolbar({ status: "all", queue: "ready_to_ship" });
    expect(screen.getByTestId("orders-status-filter")).toHaveTextContent("Siap kirim");
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

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ q: "rani" }));
    vi.useRealTimers();
  });
});


describe("OrdersToolbar page size", () => {
  it.each([20, 50, 100] as const)("selects %i orders per page", async (limit) => {
    const { onChange } = renderToolbar();
    fireEvent.pointerDown(screen.getByRole("combobox", { name: "Pesanan per halaman" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
    await screen.findByRole("listbox");
    fireEvent.click(screen.getByRole("option", { name: String(limit) }));
    if (limit !== 20) expect(onChange).toHaveBeenCalledWith({ status: "all", limit });
    else expect(screen.getByRole("combobox", { name: "Pesanan per halaman" })).toHaveTextContent("20");
  });
});
