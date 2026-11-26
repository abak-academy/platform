import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SiteNav } from "./SiteNav";
import { ordersKeys } from "@/lib/hooks/orders";

const nav = vi.hoisted(() => ({ pathname: "/" }));
const auth = vi.hoisted(() => ({ token: null as string | null, user: null as { name: string } | null, role: undefined as string | undefined }));
const orders = vi.hoisted(() => ({ data: [] as unknown[], fail: false }));
const regs = vi.hoisted(() => ({ data: [] as unknown[], fail: false }));
const setLang = vi.hoisted(() => vi.fn());

vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => k, lang: "id" }) }));
vi.mock("@/stores/ui", () => ({ useUIStore: (sel: (s: unknown) => unknown) => sel({ setLang }) }));
vi.mock("@/stores/auth", () => ({
  useAuthStore: (sel: (s: unknown) => unknown) => sel({ token: auth.token, user: auth.user }),
}));
vi.mock("@/lib/hooks/use-capability", () => ({ useResolvedRole: () => ({ role: auth.role }) }));
const authFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", () => ({ authFetch }));
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
const renderNav = () => {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  lastQc = qc;
  return render(
    <QueryClientProvider client={qc}>
      <SiteNav />
    </QueryClientProvider>,
  );
};
let lastQc: QueryClient;
const header = () => screen.getByRole("banner");

beforeEach(() => {
  nav.pathname = "/";
  auth.token = null;
  auth.user = null;
  auth.role = undefined;
  Object.assign(orders, { data: [], fail: false });
  Object.assign(regs, { data: [], fail: false });
  authFetch.mockReset().mockImplementation(async (path: string) => {
    const src = path === "/orders" ? orders : regs;
    if (src.fail) throw new Error("401");
    return { data: src.data };
  });
  setLang.mockReset();
  Object.defineProperty(window, "scrollY", { value: 0, configurable: true });
});

describe("SiteNav — guest", () => {
  it("shows links, language, split Masuk/Daftar and no Dashboard, theme or cart", () => {
    renderNav();
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

  it("matches the prototype: centred links, wordmark, ember dot under the active link, arrow on Daftar", () => {
    nav.pathname = "/event";
    renderNav();
    const event = screen.getByRole("link", { name: "pub_nav_event" });
    expect(event.closest("ul")).toHaveClass("mx-auto");
    expect(event.lastElementChild).toHaveClass("scale-100", "bg-ember-600", "absolute");
    expect(screen.getByRole("link", { name: "pub_nav_home" })).toHaveTextContent("abak");
    expect(screen.getByRole("link", { name: "pub_nav_signup" }).querySelector("svg")).not.toBeNull();
  });

  it("switches language through the toggle", async () => {
    renderNav();
    await userEvent.click(screen.getByRole("button", { name: "EN" }));
    expect(setLang).toHaveBeenCalledWith("en");
  });

  it("marks the active link, treating /catalog/* as Toko", () => {
    nav.pathname = "/catalog/abc";
    renderNav();
    expect(screen.getByRole("link", { name: "pub_nav_shop" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("link", { name: "pub_nav_event" })).not.toHaveAttribute("aria-current");
  });

  it("opens the mobile panel with the same links", async () => {
    renderNav();
    const btn = screen.getByRole("button", { name: "pub_nav_menu_open" });
    expect(btn).toHaveAttribute("aria-expanded", "false");
    await userEvent.click(btn);
    expect(screen.getByRole("button", { name: "pub_nav_menu_close" })).toHaveAttribute("aria-expanded", "true");
    expect(screen.getAllByRole("link", { name: "pub_nav_event" })).toHaveLength(2);
  });
});

describe("SiteNav — signed in", () => {
  it("student sees Dashboard with count of pending orders + actionable registrations", async () => {
    auth.token = "t";
    auth.role = "student";
    auth.user = { name: "Siswa Uji" };
    orders.data = [{ status: "payment_pending" }, { status: "completed" }];
    regs.data = [{}, {}];
    renderNav();
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveAttribute("href", "/");
    expect(await screen.findByText("3")).toBeInTheDocument();
    expect(authFetch).toHaveBeenCalledWith("/orders", undefined, { redirectOn401: false });
    expect(screen.queryByRole("link", { name: "pub_nav_login" })).not.toBeInTheDocument();
  });

  it("invalidating the orders root refetches the nav count", async () => {
    auth.token = "t";
    auth.role = "student";
    renderNav();
    await waitFor(() => expect(authFetch).toHaveBeenCalledTimes(2));
    expect(screen.queryByText("1")).not.toBeInTheDocument();
    orders.data = [{ status: "payment_pending" }];
    await act(async () => {
      await lastQc.invalidateQueries({ queryKey: ordersKeys.all });
    });
    expect(await screen.findByText("1")).toBeInTheDocument();
  });

  it("student with nothing pending shows no count", () => {
    auth.token = "t";
    auth.role = "student";
    renderNav();
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveTextContent(/pub_nav_dashboard$/);
  });

  it("admin links to the admin home, with no count and no order fetch", () => {
    auth.token = "t";
    auth.role = "super_admin";
    renderNav();
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveAttribute("href", "/admin");
    expect(authFetch).not.toHaveBeenCalled();
  });

  it("an expired session shows the Dashboard link with no count and never navigates", async () => {
    auth.token = "stale";
    auth.role = "student";
    orders.fail = true;
    regs.fail = true;
    renderNav();
    await waitFor(() => expect(authFetch).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("link", { name: /pub_nav_dashboard/ })).toHaveTextContent(/pub_nav_dashboard$/);
    expect(authFetch.mock.calls.every((c) => c[2]?.redirectOn401 === false)).toBe(true);
  });
});

describe("SiteNav — language toggle contrast", () => {
  it("inactive language reads light over the transparent hero and dark in pill state", () => {
    renderNav();
    expect(screen.getByRole("button", { name: "EN" }).className).not.toContain("text-ink-600");
    setY(50);
    expect(screen.getByRole("button", { name: "EN" }).className).toContain("text-ink-600");
  });
});

describe("SiteNav — scroll states", () => {
  it("is transparent at the top of the landing and a pill after 8px", () => {
    renderNav();
    expect(header()).toHaveAttribute("data-state", "top");
    setY(8);
    expect(header()).toHaveAttribute("data-state", "top");
    setY(9);
    expect(header()).toHaveAttribute("data-state", "pill");
  });

  it("starts as a pill on other pages", () => {
    nav.pathname = "/event";
    renderNav();
    expect(header()).toHaveAttribute("data-state", "pill");
  });

  it("hides past 120px on scroll down, returns after a 5px scroll up", () => {
    nav.pathname = "/event";
    renderNav();
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
    renderNav();
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
    renderNav();
    setY(700);
    expect(header()).toHaveAttribute("data-hidden", "false");
    setY(900);
    expect(header()).toHaveAttribute("data-hidden", "true");
    hero.remove();
  });
});
