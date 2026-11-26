import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CATALOG_CATEGORIES, CategoryRail } from "./CategoryRail";

describe("CategoryRail", () => {
  it("lists five categories, medal and merchandise included, and no exam tab (exams live on /event)", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(CATALOG_CATEGORIES.map((c) => c.value)).toEqual(["all", "book", "course", "merchandise", "medal"]);
    expect(screen.queryByRole("tab", { name: /kompetisi/i })).toBeNull();
    expect(screen.getByRole("tab", { name: "Merchandise" })).toBeTruthy();
    expect(screen.getByRole("tab", { name: "Medali" })).toBeTruthy();
  });

  it("marks only the selected category as selected", () => {
    render(<CategoryRail value="book" onChange={() => {}} />);
    expect(screen.getByRole("tab", { name: "Buku" })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tab", { name: "Kursus" })).toHaveAttribute("aria-selected", "false");
  });

  it("reports the clicked category", () => {
    const onChange = vi.fn();
    render(<CategoryRail value="all" onChange={onChange} />);
    fireEvent.click(screen.getByRole("tab", { name: "Medali" }));
    expect(onChange).toHaveBeenCalledWith("medal");
  });

  it("is a horizontal pill rail with an icon and a count per tab, like the prototype", () => {
    render(<CategoryRail value="all" onChange={() => {}} counts={{ all: 5, book: 2, course: 1, merchandise: 2, medal: 0 }} />);
    const rail = screen.getByTestId("category-rail");
    expect(rail).toHaveAttribute("role", "tablist");
    expect(rail.className).toContain("overflow-x-auto");
    const book = screen.getByRole("tab", { name: /Buku/ });
    expect(book.querySelector("svg")).not.toBeNull();
    expect(book).toHaveTextContent("2");
  });
});
