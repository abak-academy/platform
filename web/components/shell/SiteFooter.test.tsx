import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { SiteFooter } from "./SiteFooter";

const cfg = vi.hoisted(() => ({
  social: [{ key: "instagram", label: "Instagram", url: null as string | null }],
}));

vi.mock("@/lib/i18n", () => ({ useTranslation: () => ({ t: (k: string) => k, lang: "id" }) }));
vi.mock("@/lib/site-config", () => ({
  SITE_CONFIG: { whatsappDisplay: "+62 000", address: "Alamat Uji", get social() { return cfg.social; } },
  waHref: (s: string) => `https://wa.me/x?text=${s}`,
}));

beforeEach(() => {
  cfg.social = [{ key: "instagram", label: "Instagram", url: null }];
});

describe("SiteFooter", () => {
  it("renders contact, columns and only existing destinations", () => {
    render(<SiteFooter />);
    expect(screen.getByRole("link", { name: "+62 000" })).toHaveAttribute("href", expect.stringContaining("wa.me"));
    expect(screen.getByRole("link", { name: "pub_foot_link_events" })).toHaveAttribute("href", "/event");
    expect(screen.getByRole("link", { name: "pub_foot_link_signup" })).toHaveAttribute("href", "/register");
    expect(screen.queryByText("pub_foot_link_help")).not.toBeInTheDocument();
    expect(screen.queryByText("pub_foot_link_terms")).not.toBeInTheDocument();
  });

  it("shows the abak academy wordmark and a divider line above the columns", () => {
    const { container } = render(<SiteFooter />);
    const logo = container.querySelector("footer svg")!.closest("span")!.parentElement!;
    expect(logo).toHaveTextContent(/abak\s*academy/i);
    expect(container.querySelector("[data-footer-divider]")).toHaveClass("border-t");
  });

  it("renders no social links or heading when every url is null", () => {
    render(<SiteFooter />);
    expect(screen.queryByRole("link", { name: "Instagram" })).not.toBeInTheDocument();
    expect(screen.queryByText("pub_foot_social_label")).not.toBeInTheDocument();
  });

  it("renders a social link once its url is set", () => {
    cfg.social = [{ key: "instagram", label: "Instagram", url: "https://example.test/ig" }];
    render(<SiteFooter />);
    expect(screen.getByRole("link", { name: "Instagram" })).toHaveAttribute("href", "https://example.test/ig");
  });
});
