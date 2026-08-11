import { describe, it, expect } from "vitest";
import { scaleY, buildPath, buildAreaPath, niceMax } from "./chart-utils";

describe("scaleY", () => {
  it("puts the max at the top and zero at the baseline", () => {
    expect(scaleY(100, 100, 60)).toBe(0);
    expect(scaleY(0, 100, 60)).toBe(60);
    expect(scaleY(50, 100, 60)).toBe(30);
  });

  it("does not divide by zero on an all-zero series", () => {
    expect(scaleY(0, 0, 60)).toBe(60);
  });
});

describe("niceMax", () => {
  it("rounds up to a readable ceiling", () => {
    expect(niceMax([12, 47, 31])).toBeGreaterThanOrEqual(47);
  });

  it("returns a positive number for an all-zero series", () => {
    expect(niceMax([0, 0, 0])).toBeGreaterThan(0);
  });

  it("returns a positive number for an empty series", () => {
    expect(niceMax([])).toBeGreaterThan(0);
  });
});

describe("buildPath", () => {
  it("spans the full width", () => {
    const d = buildPath([0, 50, 100], 100, 300, 60);
    expect(d.startsWith("M0,")).toBe(true);
    expect(d).toContain("300,");
  });

  it("emits one command per value", () => {
    const d = buildPath([1, 2, 3, 4], 4, 300, 60);
    expect(d.split("L")).toHaveLength(4); // M + 3 L
  });

  it("returns an empty string for an empty series", () => {
    expect(buildPath([], 10, 300, 60)).toBe("");
  });

  it("handles a single point without NaN", () => {
    const d = buildPath([5], 10, 300, 60);
    expect(d).not.toContain("NaN");
  });
});

describe("buildAreaPath", () => {
  it("closes back to the baseline", () => {
    const d = buildAreaPath([0, 50, 100], 100, 300, 60);
    expect(d.endsWith("Z")).toBe(true);
    expect(d).toContain("60"); // baseline y
  });

  it("returns an empty string for an empty series", () => {
    expect(buildAreaPath([], 10, 300, 60)).toBe("");
  });
});
