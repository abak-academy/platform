import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CourierRateList } from "./CourierRateList";

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
});
