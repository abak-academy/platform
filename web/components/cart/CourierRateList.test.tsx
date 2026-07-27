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

const FALLBACK = [
  { courier: "Ongkir Flat", service: "Standar", price: 20000, estimated_days: 0, is_estimate: true },
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

async function openList() {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { expanded: false }));
  return user;
}

describe("CourierRateList", () => {
  // Nothing is chosen on the buyer's behalf: shipping is a real charge, so it
  // stays a deliberate act. The control starts empty and says what it wants.
  it("starts closed with a prompt and nothing selected", () => {
    renderList();
    expect(screen.getByText("Pilih jasa pengiriman")).toBeTruthy();
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
  });

  it("opens the full list of options", async () => {
    renderList();
    await openList();
    expect(screen.getAllByRole("radio")).toHaveLength(5);
  });

  it("separates the options by how soon they arrive", async () => {
    renderList();
    await openList();
    expect(screen.getByText("Sampai hari ini")).toBeTruthy();
    expect(screen.getByText("Sampai besok")).toBeTruthy();
    expect(screen.getByText("Beberapa hari")).toBeTruthy();
  });

  // Within a band the spread can be several days, so ordering purely by price
  // would put a later delivery above an earlier one over a few thousand rupiah.
  it("orders each band by arrival first, then price", async () => {
    renderList();
    await openList();
    const services = screen
      .getAllByRole("radio")
      .map((el) => within(el).getByText(/·/).textContent ?? "");
    expect(services[0]).toContain("Same Day Service");
    expect(services[1]).toContain("Besok Sampai Tujuan"); // 1 day, 14.000
    expect(services[2]).toContain("Yakin Esok Sampai"); // 1 day, 18.000
    expect(services[3]).toContain("JNE · Reguler"); // 2 days
    expect(services[4]).toContain("Tiki · Reguler"); // 4 days — cheaper, but later
  });

  // The band heading already says when it arrives, so repeating it on the row
  // would answer the same question twice.
  it("omits the date under the same-day heading and shows it elsewhere", async () => {
    renderList();
    await openList();
    const sameDay = screen.getByRole("radio", { name: /Same Day Service/ });
    expect(within(sameDay).queryByText(/Estimasi tiba/)).toBeNull();
    expect(screen.getByText("Estimasi tiba Rab, 29 Jul")).toBeTruthy();
  });

  it("collapses to the chosen option once something is picked", () => {
    renderList({ selectedKey: "JNE::Reguler" });
    expect(screen.queryAllByRole("radio")).toHaveLength(0);
    expect(screen.getByText(/JNE · Reguler/)).toBeTruthy();
    expect(screen.getByText(/Beberapa hari · Rab, 29 Jul/)).toBeTruthy();
  });

  it("labels an estimate so it cannot be mistaken for a carrier quote", async () => {
    renderList({ rates: FALLBACK });
    await openList();
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  // estimated_days is 0 both for same-day and for a duration the backend could
  // not read, and only the first deserves a date.
  it("shows no arrival date when the duration could not be read", async () => {
    renderList({ rates: FALLBACK });
    await openList();
    expect(screen.queryByText(/Estimasi tiba/)).toBeNull();
  });

  // The rows were previously divs with tabIndex and a click handler, so they
  // took focus but could not be activated.
  it("selects a rate from the keyboard", async () => {
    const onSelect = vi.fn();
    renderList({ onSelect });
    const user = await openList();

    await user.tab();
    await user.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect.mock.calls[0][0].service).toBe("Same Day Service");
  });

  it("marks the chosen rate for assistive technology", async () => {
    renderList({ selectedKey: "JNE::Reguler" });
    await openList();

    const checked = screen
      .getAllByRole("radio")
      .filter((el) => el.getAttribute("aria-checked") === "true");
    expect(checked).toHaveLength(1);
    expect(within(checked[0]).getByText(/JNE · Reguler/)).toBeTruthy();
  });
});
