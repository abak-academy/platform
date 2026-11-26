import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { PageHead, ShellChromeContext, type ShellChrome } from "./PageHead";

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

function chrome(over: Partial<ShellChrome> = {}): ShellChrome {
  return { role: "super_admin", showActions: true, openMenu: vi.fn(), menuButtonRef: null, registerHead: vi.fn(() => vi.fn()), ...over };
}

describe("PageHead", () => {
  it("renders the title as a 30px/800 display h1 and the sub line", () => {
    render(<PageHead title="Pesanan" sub="Kelola pesanan" />);
    const h1 = screen.getByRole("heading", { level: 1, name: "Pesanan" });
    expect(h1).toHaveClass("type-h1");
    expect(screen.getByText("Kelola pesanan")).toHaveClass("text-sm", "text-ink-600");
  });

  it("renders eyebrow, back link and actions", () => {
    render(
      <PageHead
        eyebrow="Admin"
        title="Detail"
        back={{ href: "/admin/orders", label: "Kembali" }}
        actions={<button>Simpan</button>}
      />
    );
    expect(screen.getByText("Admin")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Kembali" })).toHaveAttribute("href", "/admin/orders");
    expect(screen.getByRole("button", { name: "Simpan" })).toBeInTheDocument();
  });

  it("renders no menu button or header actions outside the shell", () => {
    render(<PageHead title="x" />);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("inside the shell: mobile-only menu button that opens the drawer, plus theme toggle and bell for admin", async () => {
    const c = chrome();
    render(
      <ShellChromeContext.Provider value={c}>
        <PageHead title="x" actions={<button>Aksi</button>} />
      </ShellChromeContext.Provider>
    );
    const menu = screen.getByRole("button", { name: "toggle_sidebar" });
    expect(menu).toHaveClass("lg:hidden");
    await userEvent.click(menu);
    expect(c.openMenu).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: /dark_mode|light_mode/ })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "notifications" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Aksi" })).toBeInTheDocument();
    expect(c.registerHead).toHaveBeenCalled();
  });

  it("inside the shell as a student: menu button only, no theme or bell", () => {
    render(
      <ShellChromeContext.Provider value={chrome({ role: "student", showActions: false })}>
        <PageHead title="x" />
      </ShellChromeContext.Provider>
    );
    expect(screen.getByRole("button", { name: "toggle_sidebar" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /dark_mode|light_mode/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "notifications" })).not.toBeInTheDocument();
  });
  it("wraps the last match of highlight in the ember .hl span with the squiggle", () => {
    render(<PageHead title="Selamat pagi, Pagi" highlight="Pagi" size="home" />);
    const h1 = screen.getByRole("heading", { level: 1, name: "Selamat pagi, Pagi" });
    const hl = h1.querySelector("span.text-ember-700");
    expect(hl).toHaveTextContent(/^Pagi$/);
    expect(hl?.querySelector("svg[aria-hidden='true'] path")).toHaveAttribute("stroke", "var(--color-gold-600)");
    expect(h1).toHaveClass("text-[clamp(32px,4vw,48px)]", "leading-[1.08]", "tracking-[-0.04em]");
    expect(h1).not.toHaveClass("type-h1");
  });

  it("leaves the title plain when highlight is missing or absent from it", () => {
    const { rerender } = render(<PageHead title="Halo, Budi" />);
    expect(screen.getByRole("heading", { level: 1 }).querySelector("svg")).toBeNull();
    rerender(<PageHead title="Halo, Budi" highlight="Sari" />);
    expect(screen.getByRole("heading", { level: 1 }).querySelector("svg")).toBeNull();
  });
});
