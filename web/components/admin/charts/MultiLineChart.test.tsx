import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { MultiLineChart } from "./MultiLineChart";

let reduced = false;
vi.mock("./chart-utils", async () => {
  const actual = await vi.importActual<typeof import("./chart-utils")>("./chart-utils");
  return { ...actual, usePrefersReducedMotion: () => reduced };
});

const base = {
  labels: ["1 Jul", "2 Jul", "3 Jul"],
  series: [
    { values: [1, 2, 3], color: "#1A5CFF", label: "Baru" },
    { values: [4, 5, 6], color: "#F5A623", label: "Ikut ujian" },
    { values: [7, 8, 9], color: "#9B51E0", label: "Belanja" },
  ],
  emptyLabel: "Belum ada data",
};

beforeEach(() => {
  reduced = false;
});

describe("MultiLineChart", () => {
  it("draws one path per series", () => {
    const { container } = render(<MultiLineChart {...base} />);
    expect(container.querySelectorAll("path")).toHaveLength(3);
  });

  it("renders every series label", () => {
    render(<MultiLineChart {...base} />);
    expect(screen.getByText("Baru")).toBeInTheDocument();
    expect(screen.getByText("Ikut ujian")).toBeInTheDocument();
    expect(screen.getByText("Belanja")).toBeInTheDocument();
  });

  it("scales all series on one axis so they stay comparable", () => {
    const { container } = render(<MultiLineChart {...base} />);
    const paths = Array.from(container.querySelectorAll("path"));
    const firstY = parseFloat(paths[0].getAttribute("d")!.split(",")[1]);
    const lastY = parseFloat(paths[2].getAttribute("d")!.split(",")[1]);
    // Series 3 has larger values, so it must sit higher (smaller y).
    expect(lastY).toBeLessThan(firstY);
  });

  it("shows an empty state on an all-zero series", () => {
    render(
      <MultiLineChart
        {...base}
        series={base.series.map((s) => ({ ...s, values: [0, 0, 0] }))}
      />
    );
    expect(screen.getByText("Belum ada data")).toBeInTheDocument();
  });

  it("emits no NaN in any path", () => {
    const { container } = render(<MultiLineChart {...base} />);
    container.querySelectorAll("path").forEach((p) => {
      expect(p.getAttribute("d") ?? "").not.toContain("NaN");
    });
  });

  it("applies no animation class under prefers-reduced-motion", () => {
    reduced = true;
    const { container } = render(<MultiLineChart {...base} />);
    expect(container.querySelector(".chart-draw")).toBeNull();
  });

  // Not in the brief: chart-utils.buildPath([5], ...) returns a lone "M" with
  // no "L", which SVG renders as nothing. A single-bucket period is real
  // data, not an empty state — render a dot per series instead. Each series
  // is gated independently since series can have different lengths.
  it("renders a dot instead of a blank path for a single-point series", () => {
    const { container } = render(
      <MultiLineChart
        {...base}
        labels={["1 Jul"]}
        series={base.series.map((s) => ({ ...s, values: [s.values[0]] }))}
      />
    );
    expect(screen.queryByText("Belum ada data")).not.toBeInTheDocument();
    const circles = container.querySelectorAll("circle");
    expect(circles.length).toBeGreaterThanOrEqual(3);
    circles.forEach((c) => {
      expect(c.getAttribute("cx") ?? "").not.toContain("NaN");
      expect(c.getAttribute("cy") ?? "").not.toContain("NaN");
    });
  });

  // Nothing at the component boundary enforces values.length === labels.length,
  // and series can independently be single-point vs multi-point. A mix must
  // not crash or emit NaN.
  it("handles series of different lengths without crashing or emitting NaN", () => {
    const { container } = render(
      <MultiLineChart
        labels={["1 Jul"]}
        series={[
          { values: [5], color: "#1A5CFF", label: "Baru" },
          { values: [4, 5, 6], color: "#F5A623", label: "Ikut ujian" },
          { values: [7, 8, 9, 10], color: "#9B51E0", label: "Belanja" },
        ]}
        emptyLabel="Belum ada data"
      />
    );
    container.querySelectorAll("path").forEach((p) => {
      expect(p.getAttribute("d") ?? "").not.toContain("NaN");
    });
    container.querySelectorAll("circle").forEach((c) => {
      expect(c.getAttribute("cx") ?? "").not.toContain("NaN");
      expect(c.getAttribute("cy") ?? "").not.toContain("NaN");
    });
  });

  it("exposes every series, not just one, in the sr-only summary", () => {
    const { container } = render(<MultiLineChart {...base} />);
    const summary = container.querySelector(".sr-only")?.textContent ?? "";
    base.series.forEach((s, i) => {
      expect(summary).toContain(s.label);
      expect(summary).toContain(String(s.values[i]));
    });
  });

  it("does not assume labels.length === values.length in the sr-only summary", () => {
    const { container } = render(
      <MultiLineChart
        labels={["1 Jul", "2 Jul"]}
        series={[{ values: [1, 2, 3], color: "#1A5CFF", label: "Baru" }]}
        emptyLabel="Belum ada data"
      />
    );
    // Must not throw, and must not render an "undefined" value past the
    // shorter array's bound.
    const summary = container.querySelector(".sr-only")?.textContent ?? "";
    expect(summary).not.toContain("undefined");
  });
});
