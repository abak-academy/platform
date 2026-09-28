import { describe, it, expect } from "vitest";
import { usedFontCssVars, fontFaceCSS } from "./certificate-fonts";
import type { CertificateLayout } from "@/lib/types";

const layoutWithFonts = (fonts: (string | undefined)[]): CertificateLayout => ({
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: fonts.map((font, i) => ({
    id: `field_${i}`,
    kind: "text",
    x_mm: 10,
    y_mm: 10 + i * 10,
    w_mm: 100,
    align: "center",
    visible: true,
    font,
  })),
});

describe("usedFontCssVars", () => {
  it("always includes the source-serif fallback", () => {
    expect(usedFontCssVars(layoutWithFonts([]))).toEqual(new Set(["--font-certificate-source-serif"]));
  });

  it("maps field font keys to their CSS variables", () => {
    expect(usedFontCssVars(layoutWithFonts(["cinzel", "public_sans"]))).toEqual(
      new Set(["--font-certificate-source-serif", "--font-certificate-cinzel", "--font-certificate-public-sans"])
    );
  });

  it("ignores unknown font keys (Core falls back to source-serif)", () => {
    expect(usedFontCssVars(layoutWithFonts(["not_a_font"]))).toEqual(new Set(["--font-certificate-source-serif"]));
  });
});

describe("fontFaceCSS", () => {
  it("embeds @font-face only for used fonts", () => {
    const css = fontFaceCSS(usedFontCssVars(layoutWithFonts(["cinzel"])));
    expect(css).toContain('@font-face{font-family:"--font-certificate-cinzel"');
    expect(css).toContain('@font-face{font-family:"--font-certificate-source-serif"');
    expect(css).not.toContain('@font-face{font-family:"--font-certificate-playfair-display"');
  });

  it("embeds fonts as self-contained data URIs", () => {
    const css = fontFaceCSS(usedFontCssVars(layoutWithFonts(["poppins"])));
    expect(css).toMatch(/@font-face\{font-family:"--font-certificate-poppins";src:url\(data:font\/ttf;base64,[A-Za-z0-9+/=]+\)/);
  });

  it("keeps :root variable declarations for every font", () => {
    const css = fontFaceCSS(new Set(["--font-certificate-cinzel"]));
    expect(css).toContain(':root{--font-certificate-playfair-display:"--font-certificate-playfair-display";}');
    expect(css).toContain(':root{--font-certificate-parisienne:"--font-certificate-parisienne";}');
  });
});
