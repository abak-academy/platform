import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LandingHero } from "./LandingHero";
import { SITE_CONFIG } from "@/lib/site-config";

describe("LandingHero", () => {
  it("renders the headline, CTAs and the three configured stats with no data", () => {
    const { container } = render(<LandingHero eventNames={[]} />);
    expect(screen.getByRole("heading", { level: 1, name: "Dari ruang kelas ke panggung dunia" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Cari kompetisimu" })).toHaveAttribute("href", "/event");
    expect(screen.getByRole("link", { name: "Konsultasi gratis" }).getAttribute("href")).toMatch(/^https:\/\/wa\.me\/\d+\?text=/);
    expect(SITE_CONFIG.stats).toHaveLength(3);
    for (const s of SITE_CONFIG.stats) expect(screen.getByText(s.value)).toBeInTheDocument();
    expect(screen.getByText("peserta sejak 2021")).toBeInTheDocument();
    expect(screen.queryByText(/baru/i)).not.toBeInTheDocument();
    expect(container.querySelector("[data-hero][data-swarm-light] [data-hero-content]")).not.toBeNull();
  });

  it("uses the photo as an eager, high-priority, decorative cover image", () => {
    const { container } = render(<LandingHero eventNames={["IKMC"]} />);
    const img = container.querySelector("[data-hero-photo] img") as HTMLImageElement;
    expect(img.getAttribute("src")).toBe("/landing/hero-award.webp");
    expect(img.getAttribute("fetchpriority")).toBe("high");
    expect(img.getAttribute("loading")).toBe("eager");
    expect(img.getAttribute("alt")).toBe("");
    expect(img.className).toContain("object-cover");
    expect(img.className).toContain("object-bottom");
  });

  it("sits under the transparent navbar: white text, dark scrim ending in the surface colour, ember word", () => {
    const { container } = render(<LandingHero eventNames={["IKMC"]} />);
    expect(container.querySelector("h1")?.className).toContain("text-white");
    expect(container.querySelector("[data-hero]")?.className).toContain("min-h-[min(980px,100svh)]");
    expect(container.querySelector("[data-hero] [aria-hidden].bg-linear-to-b")?.className).toContain("to-surface");
    expect(container.querySelector("h1 > span")?.className).toContain("text-ember-500");
    expect(screen.getByRole("link", { name: "Konsultasi gratis" }).className).toContain("border-white");
  });
});
