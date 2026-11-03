import { describe, it, expect } from "vitest";

import { accessWindow, workingMinutes } from "./exam-schedule";

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
