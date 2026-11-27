import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const css = readFileSync(join(__dirname, "globals.css"), "utf8");

describe("public shell css", () => {
  it("hides only guest-only shells for a signed-in session", () => {
    expect(css).toContain("html[data-session] [data-guest-only] { display: none; }");
    expect(css).not.toMatch(/\[data-public-shell\]\s*\{\s*display:\s*none/);
  });

  it("clips horizontal overflow on the shell without breaking sticky", () => {
    expect(css).toMatch(/\[data-public-shell\]\s*\{\s*overflow-x:\s*clip;/);
  });

  it("tilts any [data-tilt] card, not only ones inside a finished reveal", () => {
    expect(css).toMatch(/\[data-tilt\]\.pm-tilting\s*\{/);
  });
});

describe("base body", () => {
  it("uses the prototype ink, canvas and line height so unstyled text is not pure black", () => {
    const body = css.match(/\nbody \{([^}]*)\}/)![1];
    expect(body).toContain("color: var(--color-ink-900)");
    expect(body).toContain("background: var(--color-paper)");
    expect(body).toContain("line-height: 1.55");
  });
});
