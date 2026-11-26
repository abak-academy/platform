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
const bellLink = () => screen.queryByRole("link", { name: "Notifikasi" });

describe("AppHeader — admin and affiliator roles", () => {
  it.each<UserRole>([...ADMIN_ROLES, "affiliator"])(
    "%s has a theme toggle and no search, language or account control",
    (role) => {
      const { container } = render(<AppHeader role={role} />);

      expect(themeBtn()).toBeInTheDocument();
      expect(screen.queryByRole("searchbox")).not.toBeInTheDocument();
      expect(container.querySelector("input")).toBeNull();
      expect(screen.queryByRole("button", { name: /^(ID|EN)$/ })).not.toBeInTheDocument();
      expect(screen.queryByRole("button", { name: /account/i })).not.toBeInTheDocument();
      expect(screen.queryByRole("link", { name: "Keranjang" })).not.toBeInTheDocument();
    }
  );

  it("renders the bell as a link to announcements, never a dead button", () => {
    render(<AppHeader role="super_admin" />);
    expect(bellLink()).toHaveAttribute("href", "/admin/notifications");
    expect(screen.queryByRole("button", { name: "Notifikasi" })).not.toBeInTheDocument();
  });

  it("omits the bell for a role without an announcements page", () => {
    render(<AppHeader role="affiliator" />);
    expect(bellLink()).not.toBeInTheDocument();
  });

  it("is a slim row, not a bordered 64px bar", () => {
    const { container } = render(<AppHeader role="super_admin" />);
    const header = container.querySelector("header")!;
    expect(header.className).not.toMatch(/h-16|border-b|sticky/);
  });
});

describe("AppHeader — student", () => {
  it("shows no theme, cart, bell, or language control", () => {
    render(<AppHeader role="student" />);

    expect(themeBtn()).not.toBeInTheDocument();
    expect(bellLink()).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Keranjang" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /^(ID|EN)$/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /account/i })).not.toBeInTheDocument();
  });

  it("is mobile-only so the desktop has no empty bar", () => {
    const { container } = render(<AppHeader role="student" />);
    expect(container.querySelector("header")).toHaveClass("lg:hidden");
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
    expect(button).toHaveClass("lg:hidden");
  });
});
