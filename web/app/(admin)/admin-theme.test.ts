import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, it, expect } from "vitest";

const adminCss = readFileSync(join(__dirname, "admin-theme.css"), "utf8");
const globalsCss = readFileSync(join(__dirname, "../globals.css"), "utf8");

function declsOf(css: string, selector: RegExp): Record<string, string> {
  const out: Record<string, string> = {};
  for (const m of css.replace(/\/\*[\s\S]*?\*\//g, "").matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    if (!selector.test(m[1].split(";").pop()!.trim())) continue;
    for (const d of m[2].matchAll(/(--[\w-]+)\s*:\s*([^;]+);/g)) out[d[1]] = d[2].trim();
  }
  return out;
}

function resolve(name: string, scope: Record<string, string>): string {
  let v = scope[name];
  for (let i = 0; i < 10 && v?.includes("var("); i++) {
    v = v.replace(/var\((--[\w-]+)(?:\s*,[^)]*)?\)/g, (_, n) => scope[n] ?? "");
  }
  return v;
}

const light = {
  ...declsOf(globalsCss, /^@theme static$/),
  ...declsOf(adminCss, /^\.admin-shell$/),
};
const dark = {
  ...light,
  ...declsOf(globalsCss, /^html\[data-theme="dark"\]$/),
  ...declsOf(adminCss, /^html\[data-theme="dark"\] \.admin-shell$/),
};

describe("admin MD3 bridge", () => {
  it("maps md primary to brand-600 in light and dark, with different resolved values", () => {
    expect(light["--md-sys-color-primary"]).toBe("var(--color-brand-600)");
    expect(resolve("--md-sys-color-primary", light)).toBe("#3d4ddb");
    expect(resolve("--md-sys-color-primary", dark)).toBe("#7d89f2");
    expect(resolve("--md-sys-color-on-primary", light)).not.toBe(
      resolve("--md-sys-color-on-primary", dark),
    );
  });

  it("flips surfaces and text with the theme", () => {
    for (const n of ["--md-sys-color-surface", "--md-sys-color-on-surface", "--md-sys-color-background"]) {
      expect(resolve(n, light), n).toBeTruthy();
      expect(resolve(n, light), n).not.toBe(resolve(n, dark));
    }
  });

  it("holds no hard-coded colour values", () => {
    const shell = declsOf(adminCss, /^(html\[data-theme="dark"\] )?\.admin-shell$/);
    for (const [k, v] of Object.entries(shell)) {
      if (k.startsWith("--md-sys-color")) expect(v, k).toMatch(/^var\(--color-/);
    }
  });

  it("drops the Google Fonts / Outfit / Roboto import", () => {
    expect(adminCss).not.toMatch(/@import/);
    expect(adminCss).not.toMatch(/fonts\.googleapis\.com|Outfit|Roboto/i);
  });

  it("uses the v2 font tokens", () => {
    expect(light["--md-sys-typescale-display"]).toContain("var(--font-display)");
    expect(light["--md-sys-typescale-body"]).toContain("var(--font-sans)");
  });
});
