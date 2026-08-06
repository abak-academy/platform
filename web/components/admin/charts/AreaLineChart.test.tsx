import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { AreaLineChart } from "./AreaLineChart";

let reduced = false;
vi.mock("./chart-utils", async () => {
  const actual = await vi.importActual<typeof import("./chart-utils")>("./chart-utils");
  return { ...actual, usePrefersReducedMotion: () => reduced };
});

const base = {
  labels: ["1 Jul", "2 Jul", "3 Jul"],
  area: { values: [10, 20, 30], color: "#1A5CFF", label: "Pendapatan" },
  line: { values: [1, 2, 3], color: "#00A37A", label: "Pesanan" },
  emptyLabel: "Belum ada data",
};

beforeEach(() => {
  reduced = false;
});

describe("AreaLineChart", () => {
  it("draws an area path and a line path", () => {
    const { container } = render(<AreaLineChart {...base} />);
    const paths = container.querySelectorAll("path");
    expect(paths.length).toBeGreaterThanOrEqual(2);
    expect(paths[0].getAttribute("d")).toMatch(/Z$/); // area closes
    expect(paths[1].getAttribute("d")).not.toMatch(/Z$/); // line does not
  });

  it("renders both series labels", () => {
    render(<AreaLineChart {...base} />);
    expect(screen.getByText("Pendapatan")).toBeInTheDocument();
    expect(screen.getByText("Pesanan")).toBeInTheDocument();
  });

  it("shows an empty state instead of a broken axis on an all-zero series", () => {
    render(
      <AreaLineChart
        {...base}
        area={{ ...base.area, values: [0, 0, 0] }}
        line={{ ...base.line, values: [0, 0, 0] }}
      />
    );
    expect(screen.getByText("Belum ada data")).toBeInTheDocument();
  });

  it("shows an empty state when there are no points at all", () => {
    render(
      <AreaLineChart
        {...base}
        labels={[]}
        area={{ ...base.area, values: [] }}
        line={{ ...base.line, values: [] }}
      />
    );
    expect(screen.getByText("Belum ada data")).toBeInTheDocument();
  });

  it("emits no NaN in any path", () => {
    const { container } = render(<AreaLineChart {...base} />);
    container.querySelectorAll("path").forEach((p) => {
      expect(p.getAttribute("d") ?? "").not.toContain("NaN");
    });
  });

  it("applies the draw animation class by default", () => {
    const { container } = render(<AreaLineChart {...base} />);
    expect(container.querySelector(".chart-draw")).toBeTruthy();
  });

  it("applies no animation class under prefers-reduced-motion", () => {
    reduced = true;
    const { container } = render(<AreaLineChart {...base} />);
    expect(container.querySelector(".chart-draw")).toBeNull();
  });

  // Not in the brief: chart-utils.buildPath([5], ...) returns a lone "M" with
  // no "L", which SVG renders as nothing. A single-bucket period (e.g. a
  // one-day custom range) is real data, not an empty state — render it as a
  // dot per series instead of silently drawing a blank chart.
  it("renders a dot instead of a blank chart for a single-point series", () => {
    const { container } = render(
      <AreaLineChart
        {...base}
        labels={["1 Jul"]}
        area={{ ...base.area, values: [42] }}
        line={{ ...base.line, values: [3] }}
      />
    );
    expect(screen.queryByText("Belum ada data")).not.toBeInTheDocument();
    const circles = container.querySelectorAll("circle");
    expect(circles.length).toBeGreaterThanOrEqual(2);
    circles.forEach((c) => {
      expect(c.getAttribute("cx") ?? "").not.toContain("NaN");
      expect(c.getAttribute("cy") ?? "").not.toContain("NaN");
    });
  });
});
