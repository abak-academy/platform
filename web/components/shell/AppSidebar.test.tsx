import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import React from "react";
import { AppSidebar } from "./AppSidebar";
import { NAV_CONFIG } from "@/lib/nav-config";

const replace = vi.fn();

const nav = vi.hoisted(() => ({ pathname: "/dashboard" }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace }),
  usePathname: () => nav.pathname,
}));

// Runs onSettled the way the real mutation does, so the redirect the component
// registers actually fires.
const logoutMutate = vi.fn(
  (_input: unknown, opts?: { onSettled?: () => void }) => opts?.onSettled?.()
);

const meState = vi.hoisted(() => ({ data: undefined as unknown }));

vi.mock("@/lib/hooks/auth", () => ({
  useLogout: () => ({ mutate: logoutMutate }),
  useMe: () => meState,
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

let authStore = {
  user: null as {
    name?: string;
    email?: string;
    username?: string;
    role?: string;
    photo_url?: string;
  } | null,
};

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof authStore) => unknown) => selector(authStore),
}));


// Radix AvatarImage only renders <img> after native image load, which
// never fires in jsdom.  We mock the avatar UI to behave like a real
// browser: render <img> when src is provided, fallback otherwise.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children, className, ...props }: any) => (
    <div data-slot="avatar" className={className} {...props}>
      {children}
    </div>
  ),
  AvatarImage: ({ src, ...props }: { src?: string; [key: string]: any }) =>
    src ? <img src={src} alt="" data-slot="avatar-image" {...props} /> : null,
  AvatarFallback: ({ children, ...props }: any) => (
    <span data-slot="avatar-fallback" {...props}>
      {children}
    </span>
  ),
}));

describe("AppSidebar — avatar rendering", () => {
  beforeEach(() => {
    replace.mockClear();
    authStore = { user: null };
  });

  it("renders AvatarImage when user.photo_url is set", () => {
    authStore = {
      user: {
        name: "Budi Santoso",
        email: "budi@test.com",
        role: "student",
        photo_url: "https://example.com/photo.jpg",
      },
    };
    const { container } = render(<AppSidebar role="student" />);

    const av = container.querySelector('[data-slot="abak-avatar"]') as HTMLElement;
    expect(av.style.backgroundImage).toContain("https://example.com/photo.jpg");
  });

  it("falls back to the sprite avatar when photo_url is absent", () => {
    authStore = {
      user: {
        name: "Budi Santoso",
        email: "budi@test.com",
        role: "student",
        photo_url: undefined,
      },
    };
    render(<AppSidebar role="student" />);

    const av = document.querySelector('[data-slot="abak-avatar"]') as HTMLElement;
    expect(av.style.backgroundImage).toContain("/avatars/sprite.webp");
  });

  it("still renders an avatar when user has no name/email/username", () => {
    authStore = { user: { role: "student", photo_url: undefined } };
    render(<AppSidebar role="student" />);

    expect(document.querySelector('[data-slot="abak-avatar"]')).not.toBeNull();
  });
});

describe("AppSidebar — logout destination", () => {
  beforeEach(() => {
    replace.mockClear();
    logoutMutate.mockClear();
    authStore = { user: null };
  });

  async function clickLogout() {
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "user_menu_label" }));
    await user.click(await screen.findByRole("menuitem", { name: /logout/ }));
  }

  it("sends an admin to the admin login, not the student one", async () => {
    authStore = { user: { name: "Admin", role: "super_admin" } };
    render(<AppSidebar role="super_admin" />);

    await clickLogout();

    expect(logoutMutate).toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith("/admin/login");
    expect(replace).not.toHaveBeenCalledWith("/login");
  });

  it("sends every admin role to the admin login", async () => {
    for (const role of ["admin_store", "admin_exam", "admin_school"] as const) {
      replace.mockClear();
      authStore = { user: { name: "Admin", role } };
      const { unmount } = render(<AppSidebar role={role} />);

      await clickLogout();

      expect(replace, `${role} landed on the wrong login`).toHaveBeenCalledWith(
        "/admin/login"
      );
      unmount();
    }
  });

  it("still sends a student to the student login", async () => {
    authStore = { user: { name: "Budi", role: "student" } };
    render(<AppSidebar role="student" />);

    await clickLogout();

    expect(replace).toHaveBeenCalledWith("/login");
  });

  // It previously called mutate with no callback at all, leaving the tab where
  // it was until a route guard happened to bounce it.
  it("navigates on logout rather than leaving the tab in place", async () => {
    authStore = { user: { name: "Admin", role: "super_admin" } };
    render(<AppSidebar role="super_admin" />);

    await clickLogout();

    expect(replace).toHaveBeenCalledTimes(1);
  });
});

describe("AppSidebar — affiliator program groups", () => {
  const states = (affiliate: string, partner: string) => ({
    data: { affiliator_programs: { affiliate, partner, account_active: true } },
  });

  beforeEach(() => {
    authStore = { user: { name: "Mitra", role: "affiliator" } };
  });

  it("hides the Affiliator group from an account that is only in the Mitra program", () => {
    Object.assign(meState, states("not_enrolled", "active"));
    render(<AppSidebar role="affiliator" />);

    expect(screen.getByText("aff_nav_group_partner")).toBeInTheDocument();
    expect(screen.queryByText("aff_nav_group_affiliator")).toBeNull();
  });

  it("hides the Mitra group from a plain Affiliator", () => {
    Object.assign(meState, states("active", "not_enrolled"));
    render(<AppSidebar role="affiliator" />);

    expect(screen.getByText("aff_nav_group_affiliator")).toBeInTheDocument();
    expect(screen.queryByText("aff_nav_group_partner")).toBeNull();
  });

  it("shows both groups to an account in both programs, and while the state is unknown", () => {
    Object.assign(meState, states("active", "pending"));
    const { unmount } = render(<AppSidebar role="affiliator" />);
    expect(screen.getByText("aff_nav_group_affiliator")).toBeInTheDocument();
    expect(screen.getByText("aff_nav_group_partner")).toBeInTheDocument();
    unmount();

    meState.data = undefined;
    render(<AppSidebar role="affiliator" />);
    expect(screen.getByText("aff_nav_group_affiliator")).toBeInTheDocument();
    expect(screen.getByText("aff_nav_group_partner")).toBeInTheDocument();
  });
});


describe("AppSidebar — accordion", () => {
  beforeEach(() => {
    nav.pathname = "/admin";
    authStore = { user: { name: "Admin", role: "super_admin" } };
  });

  const header = (name: string) => screen.getByRole("button", { name: new RegExp(name) });
  const panelOf = (name: string) => document.getElementById(header(name).getAttribute("aria-controls")!)!;

  it("keeps panel ids unique across two mounted sidebars, each header controlling its own panel", () => {
    const { container } = render(
      <>
        <div data-testid="a"><AppSidebar role="super_admin" /></div>
        <div data-testid="b"><AppSidebar role="super_admin" /></div>
      </>,
    );
    const ids = [...container.querySelectorAll("[aria-controls]")].map((e) => e.getAttribute("aria-controls")!);
    expect(ids.length).toBeGreaterThan(1);
    expect(new Set(ids).size).toBe(ids.length);
    for (const id of ["a", "b"]) {
      const inst = screen.getByTestId(id);
      for (const h of inst.querySelectorAll("[aria-controls]")) {
        const panel = document.getElementById(h.getAttribute("aria-controls")!);
        expect(panel).not.toBeNull();
        expect(inst.contains(panel)).toBe(true);
      }
    }
  });

  it("opens only the group containing the active route", () => {
    nav.pathname = "/admin/exam/tests";
    render(<AppSidebar role="super_admin" />);

    expect(header("nav_group_academic")).toHaveAttribute("aria-expanded", "true");
    for (const k of ["nav_group_store", "nav_group_affiliate", "system"]) {
      expect(header(k)).toHaveAttribute("aria-expanded", "false");
    }
  });

  it("opening a closed header closes the others, clicking the open header closes it", async () => {
    nav.pathname = "/admin/exam/tests";
    const user = userEvent.setup();
    render(<AppSidebar role="super_admin" />);

    await user.click(header("nav_group_store"));
    expect(header("nav_group_store")).toHaveAttribute("aria-expanded", "true");
    expect(header("nav_group_academic")).toHaveAttribute("aria-expanded", "false");

    await user.click(header("nav_group_store"));
    expect(header("nav_group_store")).toHaveAttribute("aria-expanded", "false");
  });

  it("re-derives the open group on route change, overriding manual state", () => {
    nav.pathname = "/admin/exam/tests";
    const { rerender } = render(<AppSidebar role="super_admin" />);
    nav.pathname = "/admin/orders";
    rerender(<AppSidebar role="super_admin" />);

    expect(header("nav_group_store")).toHaveAttribute("aria-expanded", "true");
    expect(header("nav_group_academic")).toHaveAttribute("aria-expanded", "false");
  });

  it("renders untitled groups as flat rows, never as a header", () => {
    render(<AppSidebar role="super_admin" />);
    expect(screen.getByRole("link", { name: "nav_dashboard" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /nav_dashboard/ })).toBeNull();
  });

  it("gives every titled group an icon tile in its header", () => {
    render(<AppSidebar role="super_admin" />);
    for (const g of NAV_CONFIG.super_admin.filter((g) => g.titleKey)) {
      expect(g.icon).toBeDefined();
      expect(header(g.titleKey!).querySelector("svg")).not.toBeNull();
      expect(header(g.titleKey!).querySelector("span.size-8")).not.toBeNull();
    }
  });

  it("renders the active top item on a brand tile and no caption under the logo", () => {
    render(<AppSidebar role="super_admin" />);
    const home = screen.getByRole("link", { name: "nav_dashboard" });
    expect(home.querySelector("span.bg-brand-600")).not.toBeNull();
    expect(screen.queryByText("admin_panel")).toBeNull();
    expect(screen.queryByText("app_tag")).toBeNull();
  });

  it("uses focus-visible rings only, never :focus", () => {
    render(<AppSidebar role="super_admin" />);
    for (const el of [header("nav_group_store"), screen.getByRole("link", { name: "nav_dashboard" })]) {
      expect(el.className).toMatch(/focus-visible:ring-2/);
      expect(el.className).not.toMatch(/(^|\s)focus:/);
    }
  });

  it("sizes the group badge to the prototype (20px) and item badges to 22px", async () => {
    nav.pathname = "/admin";
    const orders = NAV_CONFIG.super_admin.flatMap((g) => g.items).find((i) => i.labelKey === "admin_nav_orders")!;
    orders.countKey = "orders";
    try {
      const user = userEvent.setup();
      render(<AppSidebar role="super_admin" counts={{ orders: 2 }} />);
      expect(within(header("nav_group_store")).getByText("2").className).toMatch(/h-5/);
      await user.click(header("nav_group_store"));
      expect(within(panelOf("nav_group_store")).getByText("2").className).toMatch(/h-\[22px\]/);
    } finally {
      delete orders.countKey;
    }
  });

  describe("counts", () => {
    const orders = NAV_CONFIG.super_admin.flatMap((g) => g.items).find((i) => i.labelKey === "admin_nav_orders")!;
    const promos = NAV_CONFIG.super_admin.flatMap((g) => g.items).find((i) => i.labelKey === "promos")!;
    beforeEach(() => {
      orders.countKey = "orders";
      promos.countKey = "promos";
    });
    afterEach(() => {
      delete orders.countKey;
      delete promos.countKey;
    });

    it("shows the sum of item counts on a collapsed header and each item's own count when open", async () => {
      nav.pathname = "/admin";
      const user = userEvent.setup();
      render(<AppSidebar role="super_admin" counts={{ orders: 3, promos: 4 }} />);

      expect(within(header("nav_group_store")).getByText("7")).toBeInTheDocument();

      await user.click(header("nav_group_store"));
      expect(within(header("nav_group_store")).queryByText("7")).toBeNull();
      expect(within(panelOf("nav_group_store")).getByText("3")).toBeInTheDocument();
      expect(within(panelOf("nav_group_store")).getByText("4")).toBeInTheDocument();
    });

    it("hides a zero sum", () => {
      render(<AppSidebar role="super_admin" counts={{ orders: 0 }} />);
      expect(within(header("nav_group_store")).queryByText("0")).toBeNull();
    });
  });

  describe("motion", () => {
    const stubReduced = (matches: boolean) =>
      vi.stubGlobal("matchMedia", (q: string) => ({
        matches: matches && q.includes("prefers-reduced-motion"),
        media: q,
        addEventListener: () => {},
        removeEventListener: () => {},
      }));
    afterEach(() => vi.unstubAllGlobals());

    it("animates the panel height by default", () => {
      stubReduced(false);
      render(<AppSidebar role="super_admin" />);
      expect(panelOf("nav_group_academic").className).toMatch(/transition/);
    });

    it("applies no transition class under prefers-reduced-motion", () => {
      stubReduced(true);
      render(<AppSidebar role="super_admin" />);
      for (const k of ["nav_group_academic", "nav_group_store"]) {
        expect(panelOf(k).className).not.toMatch(/transition/);
        expect(panelOf(k).innerHTML).not.toMatch(/transition/);
      }
      expect(header("nav_group_store").innerHTML).not.toMatch(/transition/);
    });
  });
});

describe("AppSidebar — student", () => {
  beforeEach(() => {
    nav.pathname = "/";
    authStore = { user: { name: "Budi", role: "student" } };
  });

  it("lists exactly the four student destinations", () => {
    render(<AppSidebar role="student" />);
    const hrefs = within(screen.getByRole("navigation")).getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toEqual(["/", "/exam", "/courses", "/orders"]);
  });

  it("badges Kompetisi and Tagihan with their counts and hides zero", () => {
    const { rerender } = render(<AppSidebar role="student" counts={{ kompetisi: 2, tagihan: 0, cart: 0 }} />);
    expect(screen.getByRole("link", { name: /nav_competition/ })).toHaveTextContent("2");
    expect(screen.getByRole("link", { name: /nav_billing/ })).not.toHaveTextContent(/\d/);
    rerender(<AppSidebar role="student" counts={{ kompetisi: 0, tagihan: 5, cart: 0 }} />);
    expect(screen.getByRole("link", { name: /nav_billing/ })).toHaveTextContent("5");
    expect(screen.getByRole("link", { name: /nav_competition/ })).not.toHaveTextContent(/\d/);
  });

  it("puts the cart count on the user card dot, for students only", () => {
    const { unmount } = render(<AppSidebar role="student" counts={{ cart: 3 }} />);
    expect(screen.getByTestId("user-menu-dot")).toHaveTextContent("3");
    unmount();
    authStore = { user: { name: "Admin", role: "super_admin" } };
    render(<AppSidebar role="super_admin" counts={{ cart: 3 }} />);
    expect(screen.queryByTestId("user-menu-dot")).toBeNull();
  });

  it("marks no nav item active on /cart", () => {
    nav.pathname = "/cart";
    render(<AppSidebar role="student" />);
    expect(within(screen.getByRole("navigation")).queryByRole("link", { current: "page" })).toBeNull();
  });
});
