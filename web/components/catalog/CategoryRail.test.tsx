import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CATALOG_CATEGORIES, CategoryRail } from "./CategoryRail";

describe("CategoryRail", () => {
  it("lists all six categories, medal and merchandise included", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(CATALOG_CATEGORIES).toHaveLength(6);
    expect(screen.getByRole("button", { name: "Merchandise" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Medali" })).toBeTruthy();
  });

  it("marks only the selected category as current", () => {
    render(<CategoryRail value="book" onChange={() => {}} />);
    const selected = screen.getByRole("button", { name: "Buku" });
    expect(selected.getAttribute("aria-current")).toBe("true");
    expect(screen.getByRole("button", { name: "Kursus" }).getAttribute("aria-current")).toBeNull();
  });

  it("reports the clicked category", () => {
    const onChange = vi.fn();
    render(<CategoryRail value="all" onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Medali" }));
    expect(onChange).toHaveBeenCalledWith("medal");
  });

  it("stays reachable while the grid scrolls", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(screen.getByTestId("category-rail").className).toContain("md:sticky");
  });
});
