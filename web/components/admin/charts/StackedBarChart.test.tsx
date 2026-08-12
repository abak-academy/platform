import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { StackedBarChart } from "./StackedBarChart";

let reduced = false;
vi.mock("./chart-utils", async () => {
  const actual = await vi.importActual<typeof import("./chart-utils")>("./chart-utils");
  return { ...actual, usePrefersReducedMotion: () => reduced };
});

const base = {
  labels: ["1 Jul", "2 Jul", "3 Jul"],
  bottom: { values: [10, 20, 30], color: "#1A5CFF", label: "Digital" },
  top: { values: [5, 10, 15], color: "#00A37A", label: "Fisik" },
  emptyLabel: "Belum ada data",
};

beforeEach(() => {
  reduced = false;
});

describe("StackedBarChart", () => {
  it("draws two rects per bucket", () => {
    const { container } = render(<StackedBarChart {...base} />);
    expect(container.querySelectorAll("rect")).toHaveLength(6);
  });

  it("stacks the top segment above the bottom one", () => {
    const { container } = render(<StackedBarChart {...base} />);
    const rects = Array.from(container.querySelectorAll("rect"));
    const bottomY = parseFloat(rects[0].getAttribute("y")!);
    const topY = parseFloat(rects[1].getAttribute("y")!);
    // Smaller y is higher on screen.
    expect(topY).toBeLessThan(bottomY);
  });

  it("scales both segments on the stacked total", () => {
    const { container } = render(<StackedBarChart {...base} />);
    const rects = Array.from(container.querySelectorAll("rect"));
    rects.forEach((r) => {
      expect(parseFloat(r.getAttribute("y")!)).toBeGreaterThanOrEqual(0);
      expect(parseFloat(r.getAttribute("height")!)).toBeGreaterThanOrEqual(0);
    });
  });

  // Brief defect: getByText(/Digital/) (a loose regex) also matches the
  // sr-only summary's single text node ("... Digital 10, Fisik 5 ..."),
  // causing a "found multiple elements" failure. AreaLineChart and
  // MultiLineChart hit the same issue and settled on an exact string match
  // for this assertion — follow that established pattern.
  it("renders both segment labels", () => {
    render(<StackedBarChart {...base} />);
    expect(screen.getByText("Digital 67%")).toBeInTheDocument();
    expect(screen.getByText("Fisik 33%")).toBeInTheDocument();
  });

  // 67/200 = 33.5%, 133/200 = 66.5% — rounding each independently (both
  // halves round away from zero) gives 34% + 67% = 101%. The second
  // percentage must be the complement of the first, not its own rounding.
  it("keeps the two legend percentages summing to 100 at a rounding boundary", () => {
    render(
      <StackedBarChart
        {...base}
        labels={["1 Jul"]}
        bottom={{ ...base.bottom, values: [67] }}
        top={{ ...base.top, values: [133] }}
      />
    );
    expect(screen.getByText("Digital 34%")).toBeInTheDocument();
    expect(screen.getByText("Fisik 66%")).toBeInTheDocument();
  });

  it("shows an empty state on an all-zero series", () => {
    render(
      <StackedBarChart
        {...base}
        bottom={{ ...base.bottom, values: [0, 0, 0] }}
        top={{ ...base.top, values: [0, 0, 0] }}
      />
    );
    expect(screen.getByText("Belum ada data")).toBeInTheDocument();
  });

  it("applies no animation class under prefers-reduced-motion", () => {
    reduced = true;
    const { container } = render(<StackedBarChart {...base} />);
    expect(container.querySelector(".chart-grow")).toBeNull();
  });

  // Lesson from the sibling charts: the sr-only summary must cover BOTH
  // segments, each value paired with its own label, not just one series.
  it("exposes both segments, not just one, in the sr-only summary", () => {
    const { container } = render(<StackedBarChart {...base} />);
    const summary = container.querySelector(".sr-only")?.textContent ?? "";
    base.bottom.values.forEach((v) => expect(summary).toContain(String(v)));
    base.top.values.forEach((v) => expect(summary).toContain(String(v)));
    expect(summary).toContain(base.bottom.label);
    expect(summary).toContain(base.top.label);
  });

  // Nothing at the component boundary enforces labels.length === values.length
  // for either segment. Must not throw or emit "undefined" past the shortest
  // array's bound.
  it("does not assume labels.length matches either segment's length in the sr-only summary", () => {
    const { container } = render(
      <StackedBarChart
        labels={["1 Jul", "2 Jul"]}
        bottom={{ values: [10, 20, 30], color: "#1A5CFF", label: "Digital" }}
        top={{ values: [5, 10], color: "#00A37A", label: "Fisik" }}
        emptyLabel="Belum ada data"
      />
    );
    const summary = container.querySelector(".sr-only")?.textContent ?? "";
    expect(summary).not.toContain("undefined");
  });

  it("emits no NaN in any rect", () => {
    const { container } = render(<StackedBarChart {...base} />);
    container.querySelectorAll("rect").forEach((r) => {
      expect(r.getAttribute("x") ?? "").not.toContain("NaN");
      expect(r.getAttribute("y") ?? "").not.toContain("NaN");
      expect(r.getAttribute("width") ?? "").not.toContain("NaN");
      expect(r.getAttribute("height") ?? "").not.toContain("NaN");
    });
  });
});

describe("StackedBarChart hover", () => {
  const props = {
    labels: ["1 Agu", "2 Agu"],
    bottom: { values: [100, 200], color: "#2F6FED", label: "Digital", format: (v: number) => `Rp${v}` },
    top: { values: [50, 60], color: "#D6409F", label: "Fisik", format: (v: number) => `Rp${v}` },
    emptyLabel: "kosong",
  };

  it("highlights a bar slot and shows both segments", () => {
    render(<StackedBarChart {...props} />);
    const plot = screen.getByTestId("chart-hover-area");

    fireEvent.keyDown(plot, { key: "ArrowRight" });

    const tip = screen.getByTestId("chart-tooltip");
    expect(tip).toHaveTextContent("1 Agu");
    expect(tip).toHaveTextContent("Rp100");
    expect(tip).toHaveTextContent("Rp50");
    // A stacked bar has no single point to pin a dot to.
    expect(screen.queryAllByTestId("chart-marker")).toHaveLength(0);
  });
});
