import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BrandPanel } from "./BrandPanel";

describe("BrandPanel", () => {
  it("renders the wordmark, headline, copy and proof line", () => {
    const { container } = render(<BrandPanel />);
    expect(screen.getByText("abak")).toBeInTheDocument();
    expect(screen.getByText("ABAK ACADEMY HUB")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Prestasi besar dimulai dari sini." }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Daftar lomba, ikut les dan kursus/)).toBeInTheDocument();
    expect(screen.getByText("20.000+ pelajar")).toBeInTheDocument();
    expect(container.querySelector("img")?.getAttribute("src")).toBe("/landing/hero-award.webp");
  });

  it("admin variant swaps the copy and drops the proof line", () => {
    render(<BrandPanel variant="admin" />);
    expect(screen.getByText("ABAK ACADEMY · ADMIN")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Kelola platform dari satu tempat." }),
    ).toBeInTheDocument();
    expect(screen.queryByText("20.000+ pelajar")).toBeNull();
  });
});
