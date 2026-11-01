import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Pagination, pageItems } from "./Pagination";

describe("pageItems", () => {
  it("lists every page when there are few", () => {
    expect(pageItems(2, 4)).toEqual([1, 2, 3, 4]);
  });

  it("keeps the first and last page and gaps the rest", () => {
    expect(pageItems(10, 20)).toEqual([1, "gap", 8, 9, 10, 11, 12, "gap", 20]);
    expect(pageItems(1, 20)).toEqual([1, 2, 3, "gap", 20]);
    expect(pageItems(20, 20)).toEqual([1, "gap", 18, 19, 20]);
  });
});

describe("Pagination", () => {
  it("renders nothing for a single page", () => {
    const { container } = render(<Pagination page={1} pages={1} onPageChange={vi.fn()} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("marks the current page and jumps to another", () => {
    const onPageChange = vi.fn();
    render(<Pagination page={2} pages={5} onPageChange={onPageChange} />);

    expect(screen.getByRole("button", { name: "2" })).toHaveAttribute("aria-current", "page");
    fireEvent.click(screen.getByRole("button", { name: "4" }));
    expect(onPageChange).toHaveBeenCalledWith(4);
    fireEvent.click(screen.getByRole("button", { name: "2" }));
    expect(onPageChange).toHaveBeenCalledTimes(1);
  });

  it("steps with the arrows and disables them at the ends", () => {
    const onPageChange = vi.fn();
    const { rerender } = render(<Pagination page={1} pages={3} onPageChange={onPageChange} />);
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(onPageChange).toHaveBeenCalledWith(2);

    rerender(<Pagination page={3} pages={3} onPageChange={onPageChange} />);
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
  });
});
