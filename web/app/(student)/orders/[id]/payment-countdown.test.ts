import { describe, expect, it } from "vitest";
import { formatCountdown } from "./payment-countdown";

describe("formatCountdown", () => {
  it.each([
    [1182, "19:42"],
    [0, "00:00"],
    [59, "00:59"],
    [60, "01:00"],
    [600, "10:00"],
    [3599, "59:59"],
  ])("formats %i seconds as %s", (seconds, expected) => {
    expect(formatCountdown(seconds)).toBe(expected);
  });

  it("clamps a negative remainder to zero", () => {
    expect(formatCountdown(-5)).toBe("00:00");
  });
});
