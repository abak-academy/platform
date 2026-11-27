import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { RevenueMixCard } from "./RevenueMixCard";
import { formatRupiah } from "@/lib/format";

const mix = (d: number, p: number, pd?: number, pp?: number) => (
  <RevenueMixCard
    digital={{ value: d, prev: pd }}
    physical={{ value: p, prev: pp }}
  />
);

describe("RevenueMixCard", () => {
  it("shows the digital share in the ring and formatted amounts per row", () => {
    render(mix(750_000, 250_000));
    expect(screen.getByRole("img")).toHaveTextContent("75%");
    expect(screen.getByText(formatRupiah(750_000))).toBeTruthy();
    expect(screen.getByText(formatRupiah(250_000))).toBeTruthy();
    expect(screen.getByText("75%", { selector: "[data-testid=mix-digital-share]" })).toBeTruthy();
    expect(screen.getByTestId("mix-physical-share")).toHaveTextContent("25%");
  });

  it("shares always sum to 100 (1/3 split)", () => {
    render(mix(100, 200));
    expect(screen.getByTestId("mix-digital-share")).toHaveTextContent("33%");
    expect(screen.getByTestId("mix-physical-share")).toHaveTextContent("67%");
  });

  it("says the digital share rose, in whole points", () => {
    render(mix(600, 400, 500, 500));
    expect(screen.getByTestId("mix-change")).toHaveTextContent(
      "Porsi digital naik 10 poin dibanding periode sebelumnya",
    );
  });

  it("says the digital share fell", () => {
    render(mix(400, 600, 500, 500));
    expect(screen.getByTestId("mix-change")).toHaveTextContent(
      "Porsi digital turun 10 poin dibanding periode sebelumnya",
    );
  });

  it("says the share is unchanged", () => {
    render(mix(500, 500, 100, 100));
    expect(screen.getByTestId("mix-change")).toHaveTextContent(
      "Porsi digital sama seperti periode sebelumnya",
    );
  });

  it("treats a missing prev side as zero revenue", () => {
    render(mix(500, 500, undefined, 500));
    expect(screen.getByTestId("mix-change")).toHaveTextContent("naik 50 poin");
  });

  it("omits the change line when the previous window had no revenue", () => {
    render(mix(500, 500));
    expect(screen.queryByTestId("mix-change")).toBeNull();
    render(mix(500, 500, 0, 0));
    expect(screen.queryByTestId("mix-change")).toBeNull();
  });

  it("shows the empty label instead of a 0% ring when the current window has no revenue", () => {
    render(mix(0, 0, 500, 500));
    expect(screen.getByText("Belum ada data pada periode ini")).toBeTruthy();
    expect(screen.queryByRole("img")).toBeNull();
    expect(screen.queryByText("0%")).toBeNull();
  });

  it("renders the title and the changed sub-copy", () => {
    render(mix(1, 1));
    expect(screen.getByText("Digital vs fisik")).toBeTruthy();
    expect(
      screen.getByText("Komposisi pendapatan periode ini. Fisik = buku, merchandise, medali."),
    ).toBeTruthy();
  });

  it("clamps a refunded (negative) side to 0 for shares but shows the real amount", () => {
    render(mix(-100_000, 300_000));
    expect(screen.getByTestId("mix-digital-share")).toHaveTextContent("0%");
    expect(screen.getByTestId("mix-physical-share")).toHaveTextContent("100%");
    expect(screen.getByRole("img")).toHaveTextContent("0%");
    expect(screen.getByText(formatRupiah(-100_000))).toBeTruthy();
  });

  it("compares against a clamped previous share when the previous digital side was negative", () => {
    render(mix(500, 500, -100, 300));
    expect(screen.getByTestId("mix-change")).toHaveTextContent("naik 50 poin");
  });

  it("renders a refund-only window instead of the empty label", () => {
    render(mix(-100_000, 0));
    expect(screen.queryByText(/Belum ada data/)).toBeNull();
    expect(screen.getByText(formatRupiah(-100_000))).toBeTruthy();
  });
});
