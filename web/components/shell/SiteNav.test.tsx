import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { SiteNav } from "./SiteNav";

const nav = vi.hoisted(() => ({ pathname: "/" }));
const auth = vi.hoisted(() => ({ token: null as string | null, user: null as { name: string } | null, role: undefined as string | undefined }));
const orders = vi.hoisted(() => vi.fn());
const regs = vi.hoisted(() => vi.fn());
const setLang = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => k, lang: "id" }) }));
vi.mock("@/stores/ui", () => ({ useUIStore: (sel: (s: unknown) => unknown) => sel({ setLang }) }));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) => sel({ token: auth.token, user: auth.user }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useResolvedRole: () => ({ role: auth.role }) }));
vi.mock("@/lib/hooks/orders", () => ({ useOrders: orders }));
vi.mock("@/lib/hooks/exam", () => ({ useRegistrations: regs }));
vi.mock("@/lib/student-home", async (orig) => ({
  ...(await orig<typeof import("@/lib/student-home")>()),
  kompetisiCount: (r: unknown[]) => r.length,
}));

const setY = (y: number) => {
  Object.defineProperty(window, "scrollY", { value: y, configurable: true });
  act(() => {
    window.dispatchEvent(new Event("scroll"));
  });
};
const header = () => screen.getByRole("banner");

beforeEach(() => {
  nav.pathname = "/";
  auth.token = null;
  auth.user = null;
  auth.role = undefined;
  orders.mockReset().mockReturnValue({ data: [] });
  regs.mockReset().mockReturnValue({ data: [] });
  setLang.mockReset();
  Object.defineProperty(window, "scrollY", { value: 0, configurable: true });
});

describe("SiteNav — guest", () => {
  it("shows links, language, split Masuk/Daftar and no Dashboard, theme or cart", () => {
    render(<SiteNav />);
    expect(screen.getByRole("link", { name: "pub_nav_event" })).toHaveAttribute("href", "/event");
    expect(screen.getByRole("link", { name: "pub_nav_program" })).toHaveAttribute("href", "/program");
    expect(screen.getByRole("link", { name: "pub_nav_shop" })).toHaveAttribute("href", "/catalog");
    expect(screen.getByRole("link", { name: "pub_nav_login" })).toHaveAttribute("href", "/login");
    expect(screen.getByRole("link", { name: "pub_nav_signup" })).toHaveAttribute("href", "/register");
    expect(screen.getByRole("button", { name: "ID" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /pub_nav_dashboard/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /theme|dark|light/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: /cart|keranjang/i })).not.toBeInTheDocument();
  });

  it("switches language through the toggle", async () => {
    render(<SiteNav />);
    await userEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(setLang).toHaveBeenCalledWith("en");
  });

  it("marks the active link, treating /catalog/* as Toko", () => {
    nav.pathname = "/catalog/abc";
    render(<SiteNav />);
    expect(screen.getByRole("link", { name: "pub_nav_shop" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "pub_nav_event" })).not.toHaveAttribute("aria-current");
  });

  it("opens the mobile panel with the same links", async () => {
    render(<SiteNav />);
    const btn = screen.getByRole("button", { name: "pub_nav_menu_open" });
    expect(btn).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(btn);
    expect(screen.getByRole("button", { name: "pub_nav_menu_close" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("link", { name: "pub_nav_event" })).toHaveLength(2);
  });
});

describe("SiteNav — signed in", () => {
  it("student sees Dashboard with count of pending orders + actionable registrations", () => {
    auth.token = "t";
    auth.role = "student";
    auth.user = { name: "Siswa Uji" };
    orders.mockReturnValue({ data: [{ status: "payment_pending" }, { status: "completed" }] });
    regs.mockReturnValue({ data: [{}, {}] });
    render(<SiteNav />);
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveAttribute("href", "/");
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "pub_nav_login" })).not.toBeInTheDocument();
  });

  it("student with nothing pending shows no count", () => {
    auth.token = "t";
    auth.role = "student";
    render(<SiteNav />);
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveTextContent(/pub_nav_dashboard$/);
  });

  it("admin links to the admin home, with no count and no order fetch", () => {
    auth.token = "t";
    auth.role = "super_admin";
    render(<SiteNav />);
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveAttribute("href", "/admin");
    expect(orders).not.toHaveBeenCalled();
    expect(regs).not.toHaveBeenCalled();
  });
});

describe("SiteNav — scroll states", () => {
  it("is transparent at the top of the landing and a pill after 8px", () => {
    render(<SiteNav />);
    expect(header()).toHaveAttribute("data-state", "top");
    setY(8);
    expect(header()).toHaveAttribute("data-state", "top");
    setY(9);
    expect(header()).toHaveAttribute("data-state", "pill");
  });

  it("starts as a pill on other pages", () => {
    nav.pathname = "/event";
    render(<SiteNav />);
    expect(header()).toHaveAttribute("data-state", "pill");
  });

  it("hides past 120px on scroll down, returns after a 5px scroll up", () => {
    nav.pathname = "/event";
    render(<SiteNav />);
    setY(100);
    expect(header()).toHaveAttribute("data-hidden", "false");
    setY(300);
    expect(header()).toHaveAttribute("data-hidden", "true");
    setY(297);
    expect(header()).toHaveAttribute("data-hidden", "true");
    setY(294);
    expect(header()).toHaveAttribute("data-hidden", "false");
  });

  it("reappears when focus enters it", () => {
    nav.pathname = "/event";
    render(<SiteNav />);
    setY(400);
    expect(header()).toHaveAttribute("data-hidden", "true");
    act(() => screen.getByRole("link", { name: "pub_nav_program" }).focus());
    expect(header()).toHaveAttribute("data-hidden", "false");
  });

  it("on the landing, hides only past the hero height", () => {
    const hero = document.createElement("section");
    hero.setAttribute("data-hero", "");
    Object.defineProperty(hero, "offsetHeight", { value: 800 });
    document.body.appendChild(hero);
    render(<SiteNav />);
    setY(700);
    expect(header()).toHaveAttribute("data-hidden", "false");
    setY(900);
    expect(header()).toHaveAttribute("data-hidden", "true");
    hero.remove();
  });
});
