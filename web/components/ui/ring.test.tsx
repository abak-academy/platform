import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Ring } from "./ring";

describe("Ring", () => {
  it("is an image with the accessible label and renders the centre slot", () => {
    render(<Ring value={40} label="Porsi digital 40%">40%</Ring>);
    const ring = screen.getByRole("img", { name: "Porsi digital 40%" });
    expect(ring).toHaveTextContent("40%");
  });

  it.each([
    [150, "100%"],
    [-5, "0%"],
    [Number.NaN, "0%"],
  ])("clamps value %s", (value, stop) => {
    render(<Ring value={value} label="r" />);
    expect(screen.getByRole("img").style.background).toContain(stop);
  });

  it("paints the fill up to the value using tokens only", () => {
    render(<Ring value={25} label="r" />);
    const bg = screen.getByRole("img").style.background;
    expect(bg).toContain("conic-gradient");
    expect(bg).toContain("25%");
    expect(bg).not.toMatch(/#[0-9a-f]{3,8}/i);
  });
});
