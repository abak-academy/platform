import { describe, it, expect } from "vitest";
import { scaleY, buildPath, buildAreaPath, niceMax, indexFromFraction, xPercentFor } from "./chart-utils";

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

describe("indexFromFraction", () => {
  it("maps point-mode fractions to the nearest data index", () => {
    // 5 points sit at 0, 0.25, 0.5, 0.75, 1 — nearest wins.
    expect(indexFromFraction(0, 5, "point")).toBe(0);
    expect(indexFromFraction(0.24, 5, "point")).toBe(1);
    expect(indexFromFraction(0.5, 5, "point")).toBe(2);
    expect(indexFromFraction(1, 5, "point")).toBe(4);
  });

  it("maps band-mode fractions to equal-width slots", () => {
    // 4 bars own [0,.25) [.25,.5) [.5,.75) [.75,1].
    expect(indexFromFraction(0, 4, "band")).toBe(0);
    expect(indexFromFraction(0.24, 4, "band")).toBe(0);
    expect(indexFromFraction(0.26, 4, "band")).toBe(1);
    expect(indexFromFraction(0.99, 4, "band")).toBe(3);
  });

  it("clamps a fraction past either edge instead of returning out of range", () => {
    // A pointer can leave the box between pointermove and pointerleave.
    expect(indexFromFraction(-0.4, 5, "point")).toBe(0);
    expect(indexFromFraction(1.4, 5, "point")).toBe(4);
    expect(indexFromFraction(1, 4, "band")).toBe(3);
  });

  it("returns null for an empty series rather than -1", () => {
    expect(indexFromFraction(0.5, 0, "point")).toBeNull();
    expect(indexFromFraction(0.5, 0, "band")).toBeNull();
  });

  it("puts a single point at index 0 and does not divide by zero", () => {
    expect(indexFromFraction(0.7, 1, "point")).toBe(0);
  });
});

describe("xPercentFor", () => {
  it("spreads point-mode indices across the full width", () => {
    expect(xPercentFor(0, 5, "point")).toBe(0);
    expect(xPercentFor(2, 5, "point")).toBe(50);
    expect(xPercentFor(4, 5, "point")).toBe(100);
  });

  it("centres band-mode indices inside their slot", () => {
    // Never 0 or 100 — a bar's centre is inset by half a slot.
    expect(xPercentFor(0, 4, "band")).toBe(12.5);
    expect(xPercentFor(3, 4, "band")).toBe(87.5);
  });

  it("pins a lone point at the left edge, matching buildPath's xAt", () => {
    // buildPath puts a single value at x=0, not at the centre — the marker
    // must land on the dot the chart actually drew.
    expect(xPercentFor(0, 1, "point")).toBe(0);
  });
});
