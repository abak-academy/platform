import { describe, it, expect, vi, afterEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { PeriodBar, presetRange } from "./PeriodBar";

describe("presetRange", () => {
  it("sends nothing for 30d — the server default already is now-30d", () => {
    expect(presetRange("30d")).toEqual({});
  });

  it("sends a from date for 7d and 90d", () => {
    expect(presetRange("7d").from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(presetRange("90d").from).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });

  it("sends the first of the month for this_month", () => {
    expect(presetRange("this_month").from).toMatch(/^\d{4}-\d{2}-01$/);
  });
});

// The server's `to` is an exclusive today+1, so an N-day window ending today
// must start at today-(N-1) — the boundary this pins for every preset.
describe("presetRange — exact day counts", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function daysBetween(fromISO: string, toISOExclusive: string): number {
    const from = new Date(`${fromISO}T00:00:00Z`);
    const to = new Date(`${toISOExclusive}T00:00:00Z`);
    return Math.round((to.getTime() - from.getTime()) / 86400000);
  }

  it("7d covers exactly 7 days ending today", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-10T05:00:00.000Z")); // 2026-08-10T12:00 WIB
    const from = presetRange("7d").from!;
    // today (Jakarta) is 2026-08-10; the window is [from, tomorrow)
    expect(daysBetween(from, "2026-08-11")).toBe(7);
  });

  it("90d covers exactly 90 days ending today", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-08-10T05:00:00.000Z"));
    const from = presetRange("90d").from!;
    expect(daysBetween(from, "2026-08-11")).toBe(90);
  });
});

// vi.setSystemTime pins an absolute instant, so these pass under any runner
// timezone — not only when the runner's local clock happens to agree with
// Asia/Jakarta.
describe("presetRange — Asia/Jakarta calendar dates, not UTC", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  it("computes the Jakarta calendar day, not the UTC one, near the UTC/WIB day boundary", () => {
    vi.useFakeTimers();
    // 2026-08-02T17:30:00Z = 2026-08-03T00:30:00+07:00 — after Jakarta's
    // midnight but before UTC's, the exact window toISOString() gets wrong.
    vi.setSystemTime(new Date("2026-08-02T17:30:00.000Z"));

    expect(presetRange("7d").from).toBe("2026-07-28");
    expect(presetRange("90d").from).toBe("2026-05-06");
    expect(presetRange("this_month").from).toBe("2026-08-01");
  });
});

describe("PeriodBar", () => {
  it("marks the active preset as pressed", () => {
    render(<PeriodBar preset="30d" range={{}} onChange={vi.fn()} />);
    const active = screen.getByRole("button", { name: /30 hari/i });
    expect(active.getAttribute("aria-pressed")).toBe("true");
  });

  it("emits the preset and its range on click", () => {
    const onChange = vi.fn();
    render(<PeriodBar preset="30d" range={{}} onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: /7 hari/i }));

    expect(onChange).toHaveBeenCalledWith("7d", expect.objectContaining({ from: expect.any(String) }));
  });

  it("shows the resolved range so a default period is never read as all-time", () => {
    render(
      <PeriodBar
        preset="30d"
        range={{}}
        onChange={vi.fn()}
        resolvedFrom="2026-07-08"
        resolvedTo="2026-08-06"
      />
    );
    expect(screen.getByText(/2026-07-08/)).toBeInTheDocument();
    expect(screen.getByText(/2026-08-06/)).toBeInTheDocument();
  });

  it("reveals date inputs when custom is chosen", () => {
    render(<PeriodBar preset="custom" range={{}} onChange={vi.fn()} />);
    expect(screen.getAllByDisplayValue("").length).toBeGreaterThanOrEqual(2);
  });

  // Nothing on the server rejected from >= to until this fix; the date
  // inputs bounding each other is the first line of defense against a
  // super admin picking an inverted or empty custom range at all.
  it("bounds each date input by the other, so an inverted range can't be entered", () => {
    const { container } = render(
      <PeriodBar preset="custom" range={{ from: "2026-08-01", to: "2026-08-10" }} onChange={vi.fn()} />
    );
    const inputs = container.querySelectorAll('input[type="date"]');
    expect(inputs[0].getAttribute("max")).toBe("2026-08-10");
    expect(inputs[1].getAttribute("min")).toBe("2026-08-01");
  });
});
