import { describe, it, expect, vi } from "vitest";
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
