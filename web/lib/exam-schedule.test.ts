import { describe, it, expect } from "vitest";

import { accessWindow, countdownParts, workingMinutes } from "./exam-schedule";

describe("accessWindow", () => {
  it("reports a same-day window in Asia/Jakarta, not UTC", () => {
    // 17:00Z is already the next day in WIB (00:00); both ends fall on 16 Jul in WIB.
    const w = accessWindow("2026-07-15T17:00:00Z", "2026-07-15T19:00:00Z", "id");
    expect(w?.sameDay).toBe(true);
    expect(w?.from.time).toMatch(/^00[.:]00$/);
    expect(w?.to?.time).toMatch(/^02[.:]00$/);
    expect(w?.from.date).toContain("16");
  });

  it("reports a window crossing midnight WIB as two dates", () => {
    const w = accessWindow("2026-07-15T10:00:00Z", "2026-07-15T18:00:00Z", "id");
    expect(w?.sameDay).toBe(false);
    expect(w?.from.date).toContain("15");
    expect(w?.to?.date).toContain("16");
  });

  it("handles an open-ended or missing schedule", () => {
    expect(accessWindow("2026-07-15T02:00:00Z", null, "en")?.to).toBeNull();
    expect(accessWindow(null, null, "id")).toBeNull();
    expect(accessWindow("not-a-date", null, "id")).toBeNull();
  });
});

describe("accessWindow weekday", () => {
  it("prefixes the full weekday when asked, in Asia/Jakarta", () => {
    const w = accessWindow("2026-11-07T01:00:00Z", "2026-11-07T03:00:00Z", "id", { weekday: true });
    expect(w?.from.date).toBe("Sabtu, 7 Nov 2026");
    expect(w?.from.time).toMatch(/^08[.:]00$/);
    expect(w?.to?.time).toMatch(/^10[.:]00$/);
    expect(accessWindow("2026-11-07T01:00:00Z", null, "id")?.from.date).toBe("7 Nov 2026");
  });

  it("localizes the weekday in English", () => {
    expect(accessWindow("2026-11-07T01:00:00Z", null, "en", { weekday: true })?.from.date).toContain("Saturday");
  });
});

describe("countdownParts", () => {
  const MIN = 60_000;
  const H = 60 * MIN;
  const D = 24 * H;
  it("reports days with the leftover hours when a day or more remains", () => {
    expect(countdownParts(5 * D + 3 * H)).toEqual([{ unit: "day", n: 5 }, { unit: "hour", n: 3 }]);
    expect(countdownParts(D + 23 * H + 10 * MIN)).toEqual([{ unit: "day", n: 1 }, { unit: "hour", n: 23 }]);
  });

  it("skips a zero unit", () => {
    expect(countdownParts(2 * D)).toEqual([{ unit: "day", n: 2 }]);
    expect(countdownParts(2 * D + 30 * MIN)).toEqual([{ unit: "day", n: 2 }, { unit: "minute", n: 30 }]);
  });

  it("reports hours with the leftover minutes below one day", () => {
    expect(countdownParts(2 * H + 40 * MIN)).toEqual([{ unit: "hour", n: 2 }, { unit: "minute", n: 40 }]);
    expect(countdownParts(2 * H)).toEqual([{ unit: "hour", n: 2 }]);
  });

  it("switches to minutes below one hour and never shows 0", () => {
    expect(countdownParts(45 * MIN)).toEqual([{ unit: "minute", n: 45 }]);
    expect(countdownParts(10_000)).toEqual([{ unit: "minute", n: 1 }]);
  });

  it("is null once the moment has passed", () => {
    expect(countdownParts(0)).toBeNull();
    expect(countdownParts(-5 * MIN)).toBeNull();
  });
});

describe("workingMinutes", () => {
  it("uses the exam duration for one overall timer and the section sum for per-test timers", () => {
    expect(workingMinutes({ timer_mode: "overall", duration_minutes: 90, sections_duration_minutes: 60 })).toBe(90);
    expect(workingMinutes({ timer_mode: "per_test", duration_minutes: 90, sections_duration_minutes: 60 })).toBe(60);
  });

  it("falls back when the preferred value is missing", () => {
    expect(workingMinutes({ timer_mode: "per_test", duration_minutes: 90, sections_duration_minutes: null })).toBe(90);
    expect(workingMinutes({ timer_mode: "overall", duration_minutes: null, sections_duration_minutes: 60 })).toBeNull();
  });
});
