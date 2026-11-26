import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const card = readFileSync(join(__dirname, "card.tsx"), "utf8");
const css = readFileSync(join(__dirname, "../../app/(admin)/admin-theme.css"), "utf8");

describe("shared card standard", () => {
  it("Card uses the 22px card radius, line border, shadow-card and 24px padding", () => {
    expect(card).toContain("rounded-[var(--radius-card)]");
    expect(card).toContain("border-line");
    expect(card).toContain("shadow-card");
    expect(card).toContain("py-6");
    expect(card).toContain('"px-6"');
    expect(card).not.toMatch(/rounded-(lg|xl|2xl)/);
  });

  it("md-card variants share radius, border, shadow and 24px padding; flush has none", () => {
    const base = css.match(/\.md-card-outlined \{([^}]*)\}/)![1];
    expect(css).toMatch(/\.md-card-elevated,\s*\.admin-shell \.md-card-filled,\s*\.admin-shell \.md-card-outlined/);
    expect(base).toContain("border: 1px solid var(--color-line)");
    expect(base).toContain("border-radius: var(--radius-card)");
    expect(base).toContain("box-shadow: var(--shadow-card)");
    expect(base).toContain("padding: 24px");
    const flush = css.match(/\.md-card-flush \{([^}]*)\}/)![1];
    expect(flush).toContain("border-radius: var(--radius-card)");
    expect(flush).toContain("box-shadow: var(--shadow-card)");
    expect(flush).toContain("padding: 0");
  });
});
