import { describe, it, expect, vi } from "vitest";
import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AppHeader } from "./AppHeader";
import { ADMIN_ROLES, type UserRole } from "@/lib/nav-config";

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

const themeBtn = () => screen.queryByRole("button", { name: /dark_mode|light_mode/ });
const bellBtn = () => screen.queryByRole("button", { name: "Notifikasi" });

describe("AppHeader — admin and affiliator roles", () => {
  it.each<UserRole>([...ADMIN_ROLES, "affiliator"])(
    "%s has theme toggle and bell only",
    (role) => {
      const { container } = render(<AppHeader role={role} />);

      expect(themeBtn()).toBeInTheDocument();
      expect(bellBtn()).toBeInTheDocument();
      expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
      expect(container.querySelector("input")).toBeNull();
      expect(screen.queryByRole("button", { name: /^(ID|EN)$/ })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /account/i })).not.toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "Keranjang" })).not.toBeInTheDocument();
    }
  );

  it("keeps the permanent red dot on the bell", () => {
    render(<AppHeader role="super_admin" />);
    expect(bellBtn()?.querySelector(".bg-danger")).not.toBeNull();
  });
});

describe("AppHeader — student", () => {
  it("shows no theme, cart, bell, or language control", () => {
    render(<AppHeader role="student" />);

    expect(themeBtn()).not.toBeInTheDocument();
    expect(bellBtn()).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Keranjang" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^(ID|EN)$/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /account/i })).not.toBeInTheDocument();
  });
});

describe("AppHeader — menu button", () => {
  it("calls onMenuClick and exposes the button through menuButtonRef", async () => {
    const onMenuClick = vi.fn();
    const menuButtonRef = createRef<HTMLButtonElement>();
    render(<AppHeader role="student" onMenuClick={onMenuClick} menuButtonRef={menuButtonRef} />);

    const button = screen.getByRole("button", { name: "Toggle sidebar" });
    await userEvent.click(button);

    expect(onMenuClick).toHaveBeenCalledTimes(1);
    expect(menuButtonRef.current).toBe(button);
  });
});
