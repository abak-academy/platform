import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";
import { AbakAvatar, avatarIndex, avatarPosition } from "./abak-avatar";

const AV_F = [0, 1, 5, 7, 10, 15, 19, 20, 26, 27, 30, 31, 33];
const AV_M = [4, 8, 9, 11, 14, 17, 18, 22, 23, 25, 28, 29];
const AV_TF = [13, 32, 6, 24];
const AV_TM = [2, 12, 16, 21, 34, 3];

describe("avatarIndex", () => {
  it("is deterministic per name", () => {
    expect(avatarIndex("Joko Susilo")).toBe(avatarIndex("Joko Susilo"));
  });
  it("picks from the right pool", () => {
    expect(AV_F).toContain(avatarIndex("Nadia Putri"));
    expect(AV_M).toContain(avatarIndex("Joko Susilo"));
    // "Budi" and "Ranindra" only start with "bu"/"rani"; they must not land in the female pool.
    expect(AV_M).toContain(avatarIndex("Budi Santoso"));
    expect(AV_M).toContain(avatarIndex("Ranindra"));
    expect(AV_TF).toContain(avatarIndex("Bu Ratna"));
    expect(AV_TM).toContain(avatarIndex("Pak Joko"));
  });
});

describe("avatarPosition", () => {
  it("maps index 9 to col 2 row 1 of the 7x5 grid", () => {
    expect(avatarPosition(9)).toBe(`${(2 / 6) * 100}% 25%`);
    expect(avatarPosition(34)).toBe("100% 100%");
  });
});

describe("AbakAvatar", () => {
  it("renders the sprite hidden from AT with the xl double ring", () => {
    const { container } = render(<AbakAvatar name="Budi" size="xl" />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.getAttribute("aria-hidden")).toBe("true");
    expect(el.style.width).toBe("104px");
    expect(el.style.backgroundSize).toBe("700% 500%");
    expect(el.style.boxShadow).toContain("6px");
  });
  it("an explicit index overrides the name-derived sprite", () => {
    const derived = avatarIndex("Budi");
    const chosen = derived === 9 ? 10 : 9;
    const { container } = render(<AbakAvatar name="Budi" index={chosen} />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.getAttribute("data-avatar-index")).toBe(String(chosen));
    expect(el.style.backgroundPosition).toBe(avatarPosition(chosen));
  });
  it("index 0 is honoured, null falls back to the name", () => {
    const zero = render(<AbakAvatar name="Budi" index={0} />).container.firstElementChild as HTMLElement;
    expect(zero.getAttribute("data-avatar-index")).toBe("0");
    const nul = render(<AbakAvatar name="Budi" index={null} />).container.firstElementChild as HTMLElement;
    expect(nul.getAttribute("data-avatar-index")).toBe(String(avatarIndex("Budi")));
  });
  it("src overrides the sprite", () => {
    const { container } = render(<AbakAvatar name="Budi" src="/p.jpg" size={34} ring={false} />);
    const el = container.firstElementChild as HTMLElement;
    expect(el.style.backgroundImage).toContain("/p.jpg");
    expect(el.style.backgroundSize).toBe("cover");
    expect(el.style.width).toBe("34px");
  });
});
