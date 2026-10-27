import { afterEach, describe, expect, it, vi } from "vitest";
import { queryRange } from "./period";

describe("queryRange", () => {
  afterEach(() => vi.useRealTimers());

  it("spells out the 30-day window in Jakarta time, counting today as day one", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-11-30T20:00:00Z")); // 03:00 on 1 Dec in Jakarta

    expect(queryRange("30d", {})).toEqual({ from: "2026-11-02" });
  });

  it("passes every other preset through unchanged", () => {
    expect(queryRange("all_time", {})).toEqual({});
    expect(queryRange("7d", { from: "2026-10-01" })).toEqual({ from: "2026-10-01" });
    expect(queryRange("custom", { from: "2026-10-01", to: "2026-10-31" })).toEqual({ from: "2026-10-01", to: "2026-10-31" });
  });
});
