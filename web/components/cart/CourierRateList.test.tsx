import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { CourierRateList } from "./CourierRateList";

// Mirrors what Biteship actually returned for a Bekasi destination, including
// the estimated_days: 0 every rate currently carries.
const REAL_RATES = [
  { courier: "SiCepat", service: "Besok Sampai Tujuan", price: 14000, estimated_days: 0 },
  { courier: "SiCepat", service: "Reguler", price: 9000, estimated_days: 0 },
  { courier: "Tiki", service: "Same Day Service", price: 30000, estimated_days: 0 },
  { courier: "Tiki", service: "One Night Service", price: 18000, estimated_days: 0 },
  { courier: "AnterAja", service: "Reguler", price: 11500, estimated_days: 0 },
  { courier: "JNE", service: "Yakin Esok Sampai (YES)", price: 18000, estimated_days: 0 },
  { courier: "JNE", service: "JNE Trucking", price: 40000, estimated_days: 0 },
] as any;

function renderList(props: Partial<Parameters<typeof CourierRateList>[0]> = {}) {
  return render(
    <CourierRateList
      rates={REAL_RATES}
      selectedKey={null}
      onSelect={() => {}}
      isLoading={false}
      isError={false}
      {...props}
    />,
  );
}

describe("CourierRateList", () => {
  it("labels an estimate so it cannot be mistaken for a carrier quote", () => {
    render(
      <CourierRateList
        rates={[{ courier: "Flat", service: "Standard", price: 12000, is_estimate: true } as any]}
        selectedKey={null}
        onSelect={() => {}}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not label a real carrier quote", () => {
    render(
      <CourierRateList
        rates={[{ courier: "JNE", service: "REG", price: 18000 } as any]}
        selectedKey={null}
        onSelect={() => {}}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });

  it("groups rates by delivery speed, reading the tier from carrier-specific service names", () => {
    renderList();
    expect(screen.getByText("Sampai hari ini")).toBeTruthy();
    expect(screen.getByText("Sampai besok")).toBeTruthy();
    expect(screen.getByText("Beberapa hari")).toBeTruthy();
  });

  // The buyer is trading price against speed, so within a tier the cheapest
  // option has to come first — otherwise the comparison is theirs to do by hand.
  it("orders each group cheapest first", () => {
    renderList();
    const prices = screen
      .getAllByRole("radio")
      .map((el) => within(el).getByText(/^Rp/).textContent);
    // same-day: 30.000 · next-day: 14.000, 18.000, 18.000 · regular: 9.000, 11.500, 40.000
    expect(prices).toEqual([
      "Rp30.000",
      "Rp14.000",
      "Rp18.000",
      "Rp18.000",
      "Rp9.000",
      "Rp11.500",
      "Rp40.000",
    ]);
  });

  // estimated_days is 0 for every rate until the backend stops feeding Biteship's
  // human duration string to strconv.Atoi. Rendering that as "0 days" states a
  // delivery promise nobody made.
  it("renders no duration when the carrier duration could not be parsed", () => {
    renderList();
    expect(screen.queryByText(/0 hari/)).toBeNull();
    expect(screen.queryByText(/0 days/)).toBeNull();
  });

  it("renders the duration once it is a real number", () => {
    renderList({
      rates: [{ courier: "JNE", service: "Reguler", price: 9000, estimated_days: 3 }] as any,
    });
    expect(screen.getByText(/3 hari/)).toBeTruthy();
  });

  // The rows were previously divs with tabIndex and a click handler, so they took
  // focus but could not be activated from the keyboard.
  it("selects a rate from the keyboard", async () => {
    const onSelect = vi.fn();
    renderList({ onSelect });

    const user = userEvent.setup();
    await user.tab();
    await user.keyboard("{Enter}");

    expect(onSelect).toHaveBeenCalledTimes(1);
    expect(onSelect.mock.calls[0][0].service).toBe("Same Day Service");
  });

  it("marks the selected rate for assistive technology", () => {
    renderList({ selectedKey: "JNE::JNE Trucking" });
    const checked = screen.getAllByRole("radio").filter((el) => el.getAttribute("aria-checked") === "true");
    expect(checked).toHaveLength(1);
    expect(within(checked[0]).getByText("JNE Trucking")).toBeTruthy();
  });
});
