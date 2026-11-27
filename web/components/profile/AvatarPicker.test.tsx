import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { AvatarPicker } from "./AvatarPicker";

const mutate = vi.fn();
vi.mock("@/lib/hooks/profile", () => ({
  useUpdateOwnAvatar: () => ({ mutate: (...a: unknown[]) => mutate(...a), isPending: false }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

function options(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLButtonElement>("#avatar-picker-grid button[role=radio]"));
}

describe("AvatarPicker", () => {
  beforeEach(() => mutate.mockReset());

  it("is collapsed until opened, then offers all 35 sprites", () => {
    const { container } = render(<AvatarPicker name="Budi" />);
    expect(options(container)).toHaveLength(0);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(options(container)).toHaveLength(35);
  });

  it("clicking a sprite sends that zero-based index", () => {
    const { container } = render(<AvatarPicker name="Budi" />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    fireEvent.click(options(container)[6]);
    expect(mutate).toHaveBeenCalledWith(6, expect.any(Object));
  });

  it("marks the current choice and the reset button sends null", () => {
    const { container } = render(<AvatarPicker name="Budi" current={12} />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    const opts = options(container);
    expect(opts.filter((b) => b.getAttribute("aria-checked") === "true")).toEqual([opts[12]]);
    const reset = container.querySelector("#avatar-picker-grid > button") as HTMLButtonElement;
    fireEvent.click(reset);
    expect(mutate).toHaveBeenCalledWith(null, expect.any(Object));
  });

  it("arrow keys move focus and pick the neighbour", () => {
    const { container } = render(<AvatarPicker name="Budi" current={4} />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    const opts = options(container);
    expect(opts[4].tabIndex).toBe(0);
    expect(opts[5].tabIndex).toBe(-1);
    opts[4].focus();
    fireEvent.keyDown(opts[4], { key: "ArrowRight" });
    expect(mutate).toHaveBeenCalledWith(5, expect.any(Object));
    expect(document.activeElement).toBe(opts[5]);
  });

  it("shows no reset button while nothing is chosen", () => {
    const { container } = render(<AvatarPicker name="Budi" current={null} />);
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(container.querySelector("#avatar-picker-grid > button")).toBeNull();
  });
});
