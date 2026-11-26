import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { CourierRateList } from "./CourierRateList";

// Arrival dates are computed from "now", so the clock is pinned. Monday
// 2026-07-27 keeps the fixture inside one week and away from a month boundary,
// which would otherwise make the expected strings depend on the day the suite
// happens to run. shouldAdvanceTime keeps userEvent's async machinery working.
beforeAll(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true });
  vi.setSystemTime(new Date("2026-07-27T09:00:00+07:00"));
});
afterAll(() => vi.useRealTimers());

const RATES = [
  { courier: "Tiki", service: "Same Day Service", price: 30000, estimated_days: 0 },
  { courier: "SiCepat", service: "Besok Sampai Tujuan", price: 14000, estimated_days: 1 },
  { courier: "JNE", service: "Yakin Esok Sampai (YES)", price: 18000, estimated_days: 1 },
  { courier: "Tiki", service: "Reguler", price: 9000, estimated_days: 4 },
  { courier: "JNE", service: "Reguler", price: 10000, estimated_days: 2 },
] as any;

const NO_DURATION = [
  { courier: "JNE", service: "Reguler", price: 20000, estimated_days: 0 },
] as any;

function renderList(props: Partial<Parameters<typeof CourierRateList>[0]> = {}) {
  return render(
    <CourierRateList
      rates={RATES}
      selectedKey={null}
      onSelect={() => {}}
      isLoading={false}
      isError={false}
      {...props}
    />,
  );
}

describe("CourierRateList", () => {
  // Prototype `.choices`: every quoted rate is on screen at once, nothing pre-picked.
  it("lists every option with nothing selected", () => {
    renderList();
    expect(screen.getAllByRole("radio")).toHaveLength(5);
    expect(screen.getAllByRole("radio").filter((el) => el.getAttribute("aria-checked") === "true")).toHaveLength(0);
  });

  it("says on each row how soon it arrives", () => {
    renderList();
    expect(within(screen.getByRole("radio", { name: /Same Day Service/ })).getByText("Sampai hari ini")).toBeTruthy();
    expect(within(screen.getByRole("radio", { name: /YES/ })).getByText("Sampai besok")).toBeTruthy();
    expect(within(screen.getByRole("radio", { name: /JNE · Reguler/ })).getByText("Beberapa hari · Rab, 29 Jul")).toBeTruthy();
  });

  // Within a band the spread can be several days, so ordering purely by price
  // would put a later delivery above an earlier one over a few thousand rupiah.
  it("orders by arrival first, then price", () => {
    renderList();
    const services = screen.getAllByRole("radio").map((el) => el.querySelector("b")?.textContent ?? "");
    expect(services[0]).toContain("Same Day Service");
    expect(services[1]).toContain("Besok Sampai Tujuan"); // 1 day, 14.000
    expect(services[2]).toContain("Yakin Esok Sampai"); // 1 day, 18.000
    expect(services[3]).toContain("JNE · Reguler"); // 2 days
    expect(services[4]).toContain("Tiki · Reguler"); // 4 days — cheaper, but later
  });

  // estimated_days is 0 both for same-day and for a duration the backend could
  // not read, and only "a few days" rows carry a date.
  it("shows no arrival date when the duration could not be read", () => {
    renderList({ rates: NO_DURATION });
    expect(screen.queryByText(/Jul/)).toBeNull();
  });

  // The rows were previously divs with tabIndex and a click handler, so they
  // took focus but could not be activated.
  it("selects a rate from the keyboard", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    renderList({ onSelect });

    await user.tab();
    await user.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect.mock.calls[0][0].service).toBe("Same Day Service");
  });

  it("marks the chosen rate for assistive technology", () => {
    renderList({ selectedKey: "JNE::Reguler" });

    const checked = screen
      .getAllByRole("radio")
      .filter((el) => el.getAttribute("aria-checked") === "true");
    expect(checked).toHaveLength(1);
    expect(within(checked[0]).getByText(/JNE · Reguler/)).toBeTruthy();
  });
});
