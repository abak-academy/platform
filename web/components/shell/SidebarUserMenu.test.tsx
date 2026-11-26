import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import React from "react";
import { useUIStore } from "@/stores/ui";

vi.hoisted(() => {
  process.env.NEXT_PUBLIC_SITE_URL = "https://abak.test";
});

import { SidebarUserMenu } from "./SidebarUserMenu";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

const logoutMutate = vi.fn((_i: unknown, opts?: { onSettled?: () => void }) => opts?.onSettled?.());
vi.mock("@/lib/hooks/auth", () => ({
  useLogout: () => ({ mutate: logoutMutate }),
  useMe: () => ({ data: undefined }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k }),
}));

let authStore: { user: Record<string, string> | null } = { user: null };
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: typeof authStore) => unknown) => sel(authStore),
}));

vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children }: any) => <div data-slot="avatar">{children}</div>,
  AvatarImage: () => null,
  AvatarFallback: ({ children }: any) => <span>{children}</span>,
}));

async function openMenu(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: "user_menu_label" }));
  return screen.findByRole("menu");
}

describe("SidebarUserMenu", () => {
  beforeEach(() => {
    replace.mockClear();
    logoutMutate.mockClear();
    localStorage.clear();
    useUIStore.setState({ lang: "id", theme: "light" });
    authStore = { user: { name: "Rizky Admin", email: "rizky@abak.test", role: "super_admin" } };
  });

  it("shows sprite avatar, name and role label on the card", () => {
    render(<SidebarUserMenu role="super_admin" />);
    const card = screen.getByRole("button", { name: "user_menu_label" });
    expect(card.querySelector('[data-slot="abak-avatar"]')).not.toBeNull();
    expect(within(card).getByText("Rizky Admin")).toBeInTheDocument();
    expect(within(card).getByText("role_super_admin")).toBeInTheDocument();
  });

  it("opens with exactly the admin items in order, public site with hint, Keluar last", async () => {
    const user = userEvent.setup();
    render(<SidebarUserMenu role="super_admin" />);
    const menu = await openMenu(user);

    const labels = within(menu)
      .getAllByRole("menuitem")
      .map((el) => el.textContent ?? "");
    expect(labels).toHaveLength(4);
    expect(labels[0]).toContain("user_menu_account_role");
    expect(labels[1]).toContain("user_menu_language");
    expect(labels[1]).toContain("ID");
    expect(labels[1]).toContain("EN");
    expect(labels[2]).toContain("user_menu_public_site");
    expect(labels[2]).toContain("user_menu_public_site_hint");
    expect(labels[3]).toContain("logout");
    expect(within(menu).getByText("rizky@abak.test", { exact: false })).toBeInTheDocument();

    const site = within(menu).getByRole("menuitem", { name: /user_menu_public_site/ });
    expect(site).toHaveAttribute("href", "https://abak.test");
    expect(site).toHaveAttribute("target", "_blank");
    expect(site).toHaveAttribute("rel", "noopener noreferrer");
    expect(within(menu).getByRole("menuitem", { name: /user_menu_account_role/ })).toHaveAttribute(
      "href",
      "/admin/profile"
    );
  });

  it("gives the student Keranjang with count, theme, Bahasa, Profil & akun, Keluar and no Notifikasi", async () => {
    const user = userEvent.setup();
    const { unmount } = render(<SidebarUserMenu role="student" counts={{ cart: 2 }} />);
    let menu = await openMenu(user);
    expect(within(menu).getAllByRole("menuitem").map((e) => e.textContent)).toEqual([
      expect.stringMatching(/stu_menu_cart.*2/),
      expect.stringContaining("dark_mode"),
      expect.stringContaining("user_menu_language"),
      expect.stringContaining("stu_menu_profile"),
      expect.stringContaining("logout"),
    ]);
    expect(within(menu).getByRole("menuitem", { name: /stu_menu_cart/ })).toHaveAttribute("href", "/cart");
    expect(within(menu).getByRole("menuitem", { name: /stu_menu_profile/ })).toHaveAttribute("href", "/profile");
    expect(within(menu).queryByRole("menuitem", { name: /notif/i })).toBeNull();
    unmount();

    useUIStore.setState({ theme: "dark" });
    render(<SidebarUserMenu role="student" />);
    menu = await openMenu(user);
    expect(within(menu).getByRole("menuitem", { name: /light_mode/ })).toBeInTheDocument();
    expect(within(menu).queryByRole("menuitem", { name: /dark_mode/ })).toBeNull();
  });

  it("shows the subtitle on the card instead of the role label, and falls back when empty", () => {
    const { rerender } = render(<SidebarUserMenu role="student" subtitle="Kelas 12 · SMAN 1" />);
    const card = screen.getByRole("button", { name: "user_menu_label" });
    expect(within(card).getByText("Kelas 12 · SMAN 1")).toBeInTheDocument();
    expect(within(card).queryByText("role_student")).toBeNull();
    rerender(<SidebarUserMenu role="student" subtitle="" />);
    expect(within(card).getByText("role_student")).toBeInTheDocument();
  });

  it("gives the affiliator Profil, Bahasa, Keluar", async () => {
    const user = userEvent.setup();
    let menu: HTMLElement;
    render(<SidebarUserMenu role="affiliator" />);
    menu = await openMenu(user);
    expect(within(menu).getByRole("menuitem", { name: /nav_profile/ })).toHaveAttribute("href", "/affiliate/account");
  });

  it("switches language, persists it, and keeps the menu open", async () => {
    const user = userEvent.setup();
    render(<SidebarUserMenu role="super_admin" />);
    const menu = await openMenu(user);

    await user.click(within(menu).getByRole("button", { name: "EN" }));

    expect(useUIStore.getState().lang).toBe("en");
    expect(JSON.parse(localStorage.getItem("abak-ui")!).state.lang).toBe("en");
    expect(screen.getByRole("menu")).toBeInTheDocument();

    await user.click(within(screen.getByRole("menu")).getByRole("button", { name: "ID" }));
    expect(useUIStore.getState().lang).toBe("id");
  });

  it("closes on Escape and returns focus to the card", async () => {
    const user = userEvent.setup();
    render(<SidebarUserMenu role="super_admin" />);
    const card = screen.getByRole("button", { name: "user_menu_label" });
    await openMenu(user);

    await user.keyboard("{Escape}");

    expect(screen.queryByRole("menu")).toBeNull();
    expect(card).toHaveFocus();
  });

  it("keeps Escape from reaching the drawer behind the menu", async () => {
    const user = userEvent.setup();
    const drawerEscape = vi.fn();
    const onKeyDown = (e: KeyboardEvent) => e.key === "Escape" && drawerEscape();
    document.addEventListener("keydown", onKeyDown);
    try {
      render(<SidebarUserMenu role="super_admin" />);
      await openMenu(user);
      await user.keyboard("{Escape}");
      expect(screen.queryByRole("menu")).toBeNull();
      expect(drawerEscape).not.toHaveBeenCalled();
    } finally {
      document.removeEventListener("keydown", onKeyDown);
    }
  });

  it.each(["super_admin", "admin_store", "admin_exam", "admin_school"] as const)(
    "Keluar logs %s out to /admin/login",
    async (role) => {
      const user = userEvent.setup();
      authStore = { user: { name: "A", role } };
      render(<SidebarUserMenu role={role} />);
      const menu = await openMenu(user);

      await user.click(within(menu).getByRole("menuitem", { name: /logout/ }));

      expect(logoutMutate).toHaveBeenCalled();
      expect(replace).toHaveBeenCalledWith("/admin/login");
    }
  );

  it("sends a student to /login on logout", async () => {
    const user = userEvent.setup();
    authStore = { user: { name: "Budi", role: "student" } };
    render(<SidebarUserMenu role="student" />);
    const menu = await openMenu(user);
    await user.click(within(menu).getByRole("menuitem", { name: /logout/ }));
    expect(replace).toHaveBeenCalledWith("/login");
  });

  it("renders the card dot only when the dot count is positive", () => {
    const { rerender } = render(<SidebarUserMenu role="student" counts={{ cart: 3 }} dotCountKey="cart" />);
    expect(screen.getByTestId("user-menu-dot")).toHaveTextContent("3");
    rerender(<SidebarUserMenu role="student" counts={{ cart: 0 }} dotCountKey="cart" />);
    expect(screen.queryByTestId("user-menu-dot")).toBeNull();
    rerender(<SidebarUserMenu role="student" counts={{ cart: 3 }} />);
    expect(screen.queryByTestId("user-menu-dot")).toBeNull();
  });

  describe("with a stubbed menu config", () => {
    it("renders item counts and theme action items from counts", async () => {
      const cfg = await import("@/lib/nav-config");
      const { Bell, Moon } = await import("lucide-react");
      const original = cfg.USER_MENU.super_admin;
      cfg.USER_MENU.super_admin = [
        { labelKey: "nav_announcements", icon: Bell, href: "/admin/notifications", countKey: "inbox" },
        { labelKey: "dark_mode", icon: Moon, action: "theme" },
      ];
      try {
        const user = userEvent.setup();
        render(<SidebarUserMenu role="super_admin" counts={{ inbox: 5 }} />);
        const menu = await openMenu(user);

        expect(within(menu).getByRole("menuitem", { name: /nav_announcements/ })).toHaveTextContent("5");
        await user.click(within(menu).getByRole("menuitem", { name: /dark_mode/ }));
        expect(useUIStore.getState().theme).toBe("dark");
      } finally {
        cfg.USER_MENU.super_admin = original;
      }
    });
  });
});
