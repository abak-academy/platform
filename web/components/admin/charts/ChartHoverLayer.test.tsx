import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { ChartHoverLayer } from "./ChartHoverLayer";

const ROWS = [
  { label: "Pendapatan", color: "#2F6FED", value: "Rp1.500.000", yFraction: 0.25 },
  { label: "Jumlah pesanan", color: "#D2691E", value: "12", yFraction: 0.5 },
];

describe("ChartHoverLayer", () => {
  it("renders the bucket title and every series value", () => {
    render(<ChartHoverLayer index={2} count={5} mode="point" title="3 Agu" rows={ROWS} />);

    const tip = screen.getByTestId("chart-tooltip");
    expect(tip).toHaveTextContent("3 Agu");
    expect(tip).toHaveTextContent("Pendapatan");
    expect(tip).toHaveTextContent("Rp1.500.000");
    expect(tip).toHaveTextContent("Jumlah pesanan");
    expect(tip).toHaveTextContent("12");
  });

  it("announces the hovered bucket to assistive tech", () => {
    render(<ChartHoverLayer index={2} count={5} mode="point" title="3 Agu" rows={ROWS} />);
    expect(screen.getByTestId("chart-tooltip")).toHaveAttribute("role", "status");
  });

  it("puts the point-mode guide rule at the data index, not the slot centre", () => {
    render(<ChartHoverLayer index={2} count={5} mode="point" title="3 Agu" rows={ROWS} />);
    expect(screen.getByTestId("chart-guide")).toHaveStyle({ left: "50%" });
  });

  it("draws one marker dot per series that supplies a yFraction", () => {
    render(<ChartHoverLayer index={2} count={5} mode="point" title="3 Agu" rows={ROWS} />);
    expect(screen.getAllByTestId("chart-marker")).toHaveLength(2);
  });

  it("highlights a whole slot and draws no dots in band mode", () => {
    const bars = [
      { label: "Digital", color: "#2F6FED", value: "Rp1.000" },
      { label: "Fisik", color: "#D6409F", value: "Rp500" },
    ];
    render(<ChartHoverLayer index={1} count={4} mode="band" title="3 Agu" rows={bars} />);

    expect(screen.getByTestId("chart-guide")).toHaveStyle({ left: "25%", width: "25%" });
    expect(screen.queryAllByTestId("chart-marker")).toHaveLength(0);
  });

  it("clamps the tooltip inward at the edges so it cannot hang off the card", () => {
    // The guide rule stays honest at 0%; only the box is pulled in.
    render(<ChartHoverLayer index={0} count={5} mode="point" title="1 Agu" rows={ROWS} />);
    expect(screen.getByTestId("chart-guide")).toHaveStyle({ left: "0%" });
    expect(screen.getByTestId("chart-tooltip")).toHaveStyle({ left: "12%" });
  });
});
