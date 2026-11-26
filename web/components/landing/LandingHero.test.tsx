import { readFileSync } from "node:fs";
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
  });

  it("puts the stats above a decorative fade that comes after the content", () => {
    const { container } = render(<LandingHero eventNames={["IKMC"]} />);
    const content = container.querySelector("[data-hero-content]") as HTMLElement;
    const fade = container.querySelector("[data-hero-fade]") as HTMLElement;
    expect(fade.getAttribute("aria-hidden")).toBe("true");
    expect(content.compareDocumentPosition(fade) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(fade.contains(content)).toBe(false);
    for (const s of SITE_CONFIG.stats) expect(content.contains(screen.getByText(s.value))).toBe(true);
    expect(fade.textContent).toBe("");
  });

  it("preloads the hero photo and reserves three headline lines so the rotating word cannot shift the layout", () => {
    const { container } = render(<LandingHero eventNames={["IKMC"]} />);
    const link = document.querySelector('link[rel="preload"][as="image"]') as HTMLLinkElement;
    expect(link.getAttribute("href")).toBe("/landing/hero-award.webp");
    expect(link.getAttribute("fetchpriority")).toBe("high");
    expect(container.querySelector("[data-hero-content] h1")!.className).toContain("min-h-[3em]");
  });
});

describe("landing contrast", () => {
  it("keeps small light-background text on ink-600, not ink-500/400", () => {
    for (const f of ["landing/AboutLines", "landing/StorySlider", "events/EventStoryCard", "events/EventSeriesPicker"]) {
      const src = readFileSync(`components/${f}.tsx`, "utf8");
      expect(src, f).not.toMatch(/type-(overline|caption)[^"]*text-ink-(500|400)/);
    }
  });
});
