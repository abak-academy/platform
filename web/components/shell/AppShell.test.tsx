import { describe, it, expect, vi, beforeAll, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import React from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppShell } from "./AppShell";
import { PageHead } from "./PageHead";
import type { UserRole } from "@/lib/nav-config";

let pathname = "/dashboard";

beforeAll(() => {
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  });
});

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  usePathname: () => pathname,
}));

vi.mock("@/lib/hooks/auth", () => ({
  useLogout: () => ({ mutate: vi.fn() }),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

vi.mock("./AppSidebar", () => ({
  AppSidebar: ({ counts }: { counts?: Record<string, number> }) => (
    <div data-testid="sidebar" data-counts={JSON.stringify(counts ?? null)}>
      <a href="/somewhere">go</a>
    </div>
  ),
}));

vi.mock("./AppHeader", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./AppHeader")>()),
  AppHeader: ({
    onMenuClick,
    menuButtonRef,
  }: {
    onMenuClick: () => void;
    menuButtonRef: React.Ref<HTMLButtonElement>;
  }) => (
    <button ref={menuButtonRef} data-testid="menu-button" onClick={onMenuClick}>
      menu
    </button>
  ),
}));

function renderShell(role: UserRole = "super_admin", counts?: Record<string, number>) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const ui = () => (
    <QueryClientProvider client={qc}>
      <AppShell role={role} counts={counts}>
        <div>content</div>
      </AppShell>
    </QueryClientProvider>
  );
  const utils = render(ui());
  return { ...utils, rerenderShell: () => utils.rerender(ui()) };
}

describe("AppShell — admin-shell class", () => {
  beforeEach(() => {
    pathname = "/dashboard";
  });

  it.each(["super_admin", "admin_store"] as const)("adds admin-shell class when role is %s", (role) => {
    const { container } = renderShell(role);
    expect(container.querySelector(".admin-shell")).not.toBeNull();
  });

  it("does NOT add admin-shell class when role is student", () => {
    const { container } = renderShell("student");
    expect(container.querySelector(".admin-shell")).toBeNull();
  });
});

describe("AppShell — layout", () => {
  beforeEach(() => {
    pathname = "/dashboard";
  });

  it("renders a fixed 264px sidebar and no icon rail", () => {
    const { container } = renderShell();
    const aside = container.querySelector("aside");
    expect(aside?.className).toContain("w-sidebar");
    expect(container.innerHTML).not.toMatch(/\bw-16\b|\bpl-16\b|\bw-\[252px\]|\bpl-\[252px\]/);
  });

  it("caps content at the page width with 32px gutters, 16px below the narrow breakpoint", () => {
    renderShell();
    const wrapper = screen.getByText("content").parentElement!;
    expect(wrapper.className).toContain("mx-auto");
    expect(wrapper.className).toContain("w-full");
    expect(wrapper.className).toContain("max-w-page");
    expect(wrapper.className).toContain("px-4");
    expect(wrapper.className).toContain("narrow:px-8");
  });

  it("lays blocks out as a flat 20px column with the prototype's top and bottom padding", () => {
    renderShell();
    const wrapper = screen.getByText("content").parentElement!;
    for (const c of ["flex", "flex-col", "gap-5", "pt-4", "lg:pt-6", "pb-[90px]", "lg:pb-[100px]"]) {
      expect(wrapper).toHaveClass(c);
    }
  });

  it("renders no bordered 64px header bar", () => {
    const { container } = renderShell();
    expect(container.querySelector("header.h-16, header.border-b")).toBeNull();
  });

  it("shows the fallback menu button only while no PageHead is mounted", () => {
    const { rerender } = render(
      <QueryClientProvider client={new QueryClient()}>
        <AppShell role="super_admin"><PageHead title="Halaman" /></AppShell>
      </QueryClientProvider>
    );
    expect(screen.queryByTestId("menu-button")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Toggle sidebar" })).toBeInTheDocument();
    rerender(
      <QueryClientProvider client={new QueryClient()}>
        <AppShell role="super_admin"><div>plain</div></AppShell>
      </QueryClientProvider>
    );
    expect(screen.getByTestId("menu-button")).toBeInTheDocument();
  });

  it("opens the drawer from the PageHead menu button and returns focus to it", async () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <AppShell role="student"><PageHead title="Halaman" /></AppShell>
      </QueryClientProvider>
    );
    const menu = screen.getByRole("button", { name: "Toggle sidebar" });
    await userEvent.click(menu);
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await userEvent.keyboard("{Escape}");
    expect(menu).toHaveFocus();
  });

  it("forwards counts to the sidebar", () => {
    renderShell("super_admin", { exams: 3 });
    expect(screen.getAllByTestId("sidebar")[0]).toHaveAttribute("data-counts", '{"exams":3}');
  });
});

describe("AppShell — drawer below 1024px", () => {
  beforeEach(() => {
    pathname = "/dashboard";
  });

  const drawer = () => screen.queryByRole("dialog");

  it("makes the main column and desktop aside inert only while open", async () => {
    const { container } = renderShell();
    const column = screen.getByRole("main").parentElement!;
    const desktopAside = container.querySelector("aside")!;
    expect(column).not.toHaveAttribute("inert");
    expect(desktopAside).not.toHaveAttribute("inert");

    await userEvent.click(screen.getByTestId("menu-button"));
    expect(column).toHaveAttribute("inert");
    expect(desktopAside).toHaveAttribute("inert");
    expect(drawer()).not.toHaveAttribute("inert");

    await userEvent.keyboard("{Escape}");
    expect(column).not.toHaveAttribute("inert");
    expect(desktopAside).not.toHaveAttribute("inert");
  });

  it("is closed until the menu button is pressed", () => {
    renderShell();
    expect(drawer()).not.toBeInTheDocument();
  });

  it("opens from the menu button with focus inside and a scrim", async () => {
    renderShell();
    await userEvent.click(screen.getByTestId("menu-button"));

    const d = drawer()!;
    expect(d).toBeInTheDocument();
    expect(d.className).toContain("w-sidebar");
    expect(d.className).toContain("shadow-pop");
    expect(d.contains(document.activeElement)).toBe(true);
    expect(within(d).getByTestId("sidebar")).toBeInTheDocument();
    expect(screen.getByTestId("drawer-scrim")).toBeInTheDocument();
  });

  it("closes on Escape and returns focus to the menu button", async () => {
    renderShell();
    await userEvent.click(screen.getByTestId("menu-button"));
    await userEvent.keyboard("{Escape}");

    expect(drawer()).not.toBeInTheDocument();
    expect(screen.getByTestId("menu-button")).toHaveFocus();
  });

  it("closes on scrim click and returns focus to the menu button", async () => {
    renderShell();
    await userEvent.click(screen.getByTestId("menu-button"));
    await userEvent.click(screen.getByTestId("drawer-scrim"));

    expect(drawer()).not.toBeInTheDocument();
    expect(screen.getByTestId("menu-button")).toHaveFocus();
  });

  it("closes from its close button", async () => {
    renderShell();
    await userEvent.click(screen.getByTestId("menu-button"));
    await userEvent.click(within(drawer()!).getByRole("button", { name: "nav_close_menu" }));

    expect(drawer()).not.toBeInTheDocument();
    expect(screen.getByTestId("menu-button")).toHaveFocus();
  });

  it("closes when the pathname changes", async () => {
    const { rerenderShell } = renderShell();
    await userEvent.click(screen.getByTestId("menu-button"));
    expect(drawer()).toBeInTheDocument();

    pathname = "/somewhere";
    rerenderShell();

    expect(drawer()).not.toBeInTheDocument();
  });
});
