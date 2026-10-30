import { describe, expect, it } from "vitest";
import { formatCommissionRule } from "./affiliate-rules";

describe("formatCommissionRule", () => {
  it("shows a percent rule as a percentage", () => {
    expect(formatCommissionRule("percent", 7.5, "/unit")).toBe("7.5%");
  });

  it("shows a fixed rule as rupiah per unit", () => {
    expect(formatCommissionRule("fixed", 5000, "/unit")).toBe("Rp5.000/unit");
  });
});
