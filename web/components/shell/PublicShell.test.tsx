import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, createEvent } from "@testing-library/react";
import { PublicShell } from "./PublicShell";

vi.mock("./SiteNav", () => ({ SiteNav: () => <header>nav</header> }));
vi.mock("./SiteFooter", () => ({ SiteFooter: () => <footer>foot</footer> }));
vi.mock("@/components/motion/PublicMotion", () => ({ PublicMotion: () => <i data-testid="motion" /> }));
vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => k, lang: "id" }) }));

describe("PublicShell", () => {
  it("renders skip link to #main, main landmark and motion once", () => {
    render(<PublicShell><p>page</p></PublicShell>);
    expect(screen.getByRole("link", { name: "pub_nav_skip" })).toHaveAttribute("href", "#main");
    expect(screen.getByRole("main")).toHaveAttribute("id", "main");
    expect(screen.getByRole("main")).toHaveTextContent("page");
    expect(screen.getAllByTestId("motion")).toHaveLength(1);
  });

  it("marks itself guest-only only when asked, so signed-in users still see public pages", () => {
    const { container, rerender } = render(<PublicShell><p>page</p></PublicShell>);
    expect(container.querySelector("[data-public-shell]")).not.toHaveAttribute("data-guest-only");
    rerender(<PublicShell guestOnly><p>page</p></PublicShell>);
    expect(container.querySelector("[data-public-shell]")).toHaveAttribute("data-guest-only");
  });

  it("blocks the context menu inside the shell only", () => {
    render(
      <>
        <PublicShell><p>inside</p></PublicShell>
        <p>outside</p>
      </>,
    );
    const inside = createEvent.contextMenu(screen.getByText("inside"));
    fireEvent(screen.getByText("inside"), inside);
    expect(inside.defaultPrevented).toBe(true);
    const outside = createEvent.contextMenu(screen.getByText("outside"));
    fireEvent(screen.getByText("outside"), outside);
    expect(outside.defaultPrevented).toBe(false);
  });
});
