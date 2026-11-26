import { describe, it, expect, vi, beforeEach } from "vitest";
import { render } from "@testing-library/react";
import { PublicShell } from "@/components/shell/PublicShell";
import { useUIStore } from "@/stores/ui";
import { ThemeSync } from "./providers";

vi.mock("@/components/shell/SiteNav", () => ({ SiteNav: () => <header>nav</header> }));
vi.mock("@/components/shell/SiteFooter", () => ({ SiteFooter: () => <footer>foot</footer> }));
vi.mock("@/components/motion/PublicMotion", () => ({ PublicMotion: () => null }));
vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => k, lang: "id" }) }));

const theme = () => document.documentElement.getAttribute("data-theme");

describe("ThemeSync + PublicShell", () => {
  beforeEach(() => useUIStore.setState({ theme: "dark", publicShells: 0 }));

  it("forces light while a PublicShell is mounted and restores the user theme after", () => {
    render(<ThemeSync />);
    expect(theme()).toBe("dark");
    const shell = render(<PublicShell><p>x</p></PublicShell>);
    expect(theme()).toBe("light");
    shell.unmount();
    expect(theme()).toBe("dark");
  });
});
