import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import ProgramPage from "./page";
import { PROGRAMS } from "@/lib/programs";
import { t } from "@/lib/i18n";

const IDS = ["kompetisi", "olimpiade", "bimbel", "karya-tulis", "homeschooling", "study-tour"];

function blocks(container: HTMLElement) {
  return Array.from(container.querySelectorAll<HTMLElement>("section[id]"));
}

describe("ProgramPage", () => {
  it("renders one block per program with the program id, in order", () => {
    const { container } = render(<ProgramPage />);
    expect(blocks(container).map((s) => s.id)).toEqual(IDS);
  });

  it("alternates the art card left/right like the prototype, with the icon and number inside it", () => {
    const { container } = render(<ProgramPage />);
    blocks(container).forEach((s, i) => {
      const art = s.querySelector<HTMLElement>("[data-program-art]")!;
      expect(art.querySelector("svg")).not.toBeNull();
      expect(art).toHaveTextContent(String(i + 1).padStart(2, "0"));
      expect(art.className.includes("min-[901px]:order-2")).toBe(i % 2 === 1);
    });
  });

  it("fades the hero into the canvas like /event and /catalog", () => {
    const { container } = render(<ProgramPage />);
    expect(container.querySelector("h1")?.closest("section")?.querySelector("[aria-hidden].page-hero-fade")).not.toBeNull();
  });

  it("jump chips link to each block", () => {
    render(<ProgramPage />);
    const nav = screen.getByRole("navigation", { name: t("id", "prog_jump_label") });
    const hrefs = Array.from(nav.querySelectorAll("a")).map((a) => a.getAttribute("href"));
    expect(hrefs).toEqual(IDS.map((id) => `#${id}`));
  });

  it("kompetisi links to /event, the rest to WhatsApp naming the program", () => {
    const { container } = render(<ProgramPage />);
    for (const p of PROGRAMS) {
      const cta = container.querySelector<HTMLAnchorElement>(`#${p.id} a[data-cta]`)!;
      if (p.waKey === null) {
        expect(cta.getAttribute("href")).toBe("/event");
        continue;
      }
      const href = cta.getAttribute("href")!;
      expect(href.startsWith("https://wa.me/6282318910482?text=")).toBe(true);
      expect(decodeURIComponent(href.split("text=")[1])).toContain(t("id", p.titleKey));
      expect(cta.getAttribute("target")).toBe("_blank");
      expect(cta.getAttribute("rel")).toContain("noopener");
    }
  });

  it("consult band links to WhatsApp with the consult message", () => {
    render(<ProgramPage />);
    const link = screen.getByRole("link", { name: t("id", "prog_help_cta") });
    const href = link.getAttribute("href")!;
    expect(href.startsWith("https://wa.me/6282318910482?text=")).toBe(true);
    expect(decodeURIComponent(href.split("text=")[1])).toBe(t("id", "prog_help_wa"));
  });
});
