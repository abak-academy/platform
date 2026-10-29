import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { Checkbox } from "./checkbox";

describe("Checkbox", () => {
  it("is an accessible checkbox named by its label, toggled by click and by its label", () => {
    const onCheckedChange = vi.fn();
    render(
      <label>
        <Checkbox onCheckedChange={onCheckedChange} />
        Hanya aktif
      </label>,
    );
    const box = screen.getByRole("checkbox", { name: "Hanya aktif" });
    expect(box).not.toBeChecked();

    fireEvent.click(box);
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);

    fireEvent.click(screen.getByText("Hanya aktif"));
    expect(onCheckedChange).toHaveBeenCalledTimes(2);
  });

  it("shows a tick only when checked", () => {
    const { container, rerender } = render(<Checkbox checked={false} onCheckedChange={vi.fn()} aria-label="x" />);
    expect(container.querySelector("svg")).toBeNull();

    rerender(<Checkbox checked onCheckedChange={vi.fn()} aria-label="x" />);
    expect(screen.getByRole("checkbox")).toBeChecked();
    expect(container.querySelector("svg")).not.toBeNull();
  });

  it("cannot be toggled when disabled", () => {
    const onCheckedChange = vi.fn();
    render(<Checkbox disabled onCheckedChange={onCheckedChange} aria-label="x" />);

    fireEvent.click(screen.getByRole("checkbox"));

    expect(onCheckedChange).not.toHaveBeenCalled();
  });
});
