import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Switch } from "./switch";

describe("Switch", () => {
  it("is a role=switch toggled by click and keyboard", async () => {
    const onCheckedChange = vi.fn();
    render(<Switch aria-label="Terbit" onCheckedChange={onCheckedChange} />);
    const sw = screen.getByRole("switch", { name: "Terbit" });
    expect(sw).toHaveAttribute("aria-checked", "false");
    await userEvent.click(sw);
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);
    sw.focus();
    await userEvent.keyboard(" ");
    expect(onCheckedChange).toHaveBeenLastCalledWith(false);
  });

  it("uses the 46x28 track with a 22px knob and brand when on", () => {
    render(<Switch aria-label="x" defaultChecked />);
    const sw = screen.getByRole("switch");
    expect(sw).toHaveClass("h-7", "w-[46px]", "data-[state=checked]:bg-brand-600", "bg-line-strong");
    expect(sw).toHaveAttribute("data-state", "checked");
    expect(sw.firstElementChild).toHaveClass("size-[22px]", "data-[state=checked]:translate-x-[21px]");
  });

  it("supports a compact size and disabled", () => {
    render(<Switch aria-label="x" size="sm" disabled />);
    const sw = screen.getByRole("switch");
    expect(sw).toHaveAttribute("data-size", "sm");
    expect(sw).toBeDisabled();
  });
});
