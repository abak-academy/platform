import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProgramGrid } from "./ProgramGrid";
import { PROGRAMS } from "@/lib/programs";

describe("ProgramGrid", () => {
  it("renders one card per program, each linking to its /program anchor", () => {
    render(<ProgramGrid />);
    const links = screen.getAllByRole("link", { name: /^Selengkapnya/ });
    expect(links.map((a) => a.getAttribute("href"))).toEqual(PROGRAMS.map((p) => `/program#${p.id}`));
    expect(links).toHaveLength(6);
    expect(screen.getByRole("heading", { name: "Kompetisi & Event" })).toBeInTheDocument();
  });

  it("offers a WhatsApp 'Tanya program' CTA", () => {
    render(<ProgramGrid />);
    expect(screen.getByRole("link", { name: "Tanya program" }).getAttribute("href")).toMatch(/^https:\/\/wa\.me\/\d+\?text=/);
  });

  it("marks the grid and cards for the motion layer", () => {
    const { container } = render(<ProgramGrid />);
    expect(container.querySelector("section[data-reveal] [data-reveal-grid]")).not.toBeNull();
    expect(container.querySelectorAll("[data-reveal-grid] > [data-tilt]")).toHaveLength(6);
  });
});
