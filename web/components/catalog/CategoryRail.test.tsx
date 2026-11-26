import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CATALOG_CATEGORIES, CategoryRail } from "./CategoryRail";

describe("CategoryRail", () => {
  it("lists five categories, medal and merchandise included, and no exam tab (exams live on /event)", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(CATALOG_CATEGORIES.map((c) => c.value)).toEqual(["all", "book", "course", "merchandise", "medal"]);
    expect(screen.queryByRole("button", { name: /kompetisi/i })).toBeNull();
    expect(screen.getByRole("button", { name: "Merchandise" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Medali" })).toBeTruthy();
  });

  it("marks only the selected category as selected", () => {
    render(<CategoryRail value="book" onChange={() => {}} />);
    expect(screen.getByRole("button", { name: "Buku" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Kursus" })).toHaveAttribute("aria-pressed", "false");
  });

  it("reports the clicked category", () => {
    const onChange = vi.fn();
    render(<CategoryRail value="all" onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Medali" }));
    expect(onChange).toHaveBeenCalledWith("medal");
  });

  it("is a horizontal pill rail with an icon and a count per tab, like the prototype", () => {
    render(<CategoryRail value="all" onChange={() => {}} counts={{ all: 5, book: 2, course: 1, merchandise: 2, medal: 0 }} />);
    const rail = screen.getByTestId("category-rail");
    expect(rail).toHaveAttribute("role", "group");
    expect(rail.className).toContain("overflow-x-auto");
    const book = screen.getByRole("button", { name: /Buku/ });
    expect(book.querySelector("svg")).not.toBeNull();
    expect(book).toHaveTextContent("2");
  });
});
