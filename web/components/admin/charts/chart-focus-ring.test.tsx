import { describe, it, expect, vi } from "vitest";
import { render } from "@testing-library/react";
import { AreaLineChart } from "./AreaLineChart";
import { MultiLineChart } from "./MultiLineChart";
import { StackedBarChart } from "./StackedBarChart";

vi.mock("./chart-utils", async () => {
  const actual = await vi.importActual<typeof import("./chart-utils")>("./chart-utils");
  return { ...actual, usePrefersReducedMotion: () => false };
});

describe("chart hover container focus ring", () => {
  it("keeps a focus-visible outline class on every chart's keyboard-focusable container", () => {
    const results = [
      render(
        <AreaLineChart
          labels={["1 Jul"]}
          area={{ values: [10], color: "#1A5CFF", label: "Pendapatan" }}
          line={{ values: [1], color: "#00A37A", label: "Pesanan" }}
          emptyLabel="Belum ada data"
        />,
      ),
      render(
        <MultiLineChart
          labels={["1 Jul"]}
          series={[{ values: [1], color: "#1A5CFF", label: "Baru" }]}
          emptyLabel="Belum ada data"
        />,
      ),
      render(
        <StackedBarChart
          labels={["1 Jul"]}
          bottom={{ values: [10], color: "#1A5CFF", label: "Digital" }}
          top={{ values: [5], color: "#00A37A", label: "Fisik" }}
          emptyLabel="Belum ada data"
        />,
      ),
    ];

    results.forEach(({ container }) => {
      const target = container.querySelector('[data-testid="chart-hover-area"]');
      expect(target?.className).toContain("focus-visible:outline");
    });
  });
});
