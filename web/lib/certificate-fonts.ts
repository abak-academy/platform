import fs from "node:fs";
import path from "node:path";
import { fonts } from "@/components/certificate/CertificateDocumentCore";
import type { CertificateLayout } from "@/lib/types";

// Font file paths for every certificate font, keyed by the CSS variable each
// maps to. The @font-face family name is the CSS variable itself (quirky but
// consistent with CertificateDocumentCore's var() references).
const FONT_FILES: Record<string, string> = {
  "--font-certificate-public-sans": "public_sans/PublicSans-Regular.ttf",
  "--font-certificate-source-serif": "source_serif_4/SourceSerif4-SemiBold.ttf",
  "--font-certificate-cinzel": "cinzel/Cinzel-Bold.ttf",
  "--font-certificate-playfair-display": "playfair_display/PlayfairDisplay-Bold.ttf",
  "--font-certificate-cormorant-garamond": "cormorant_garamond/CormorantGaramond-SemiBold.ttf",
  "--font-certificate-great-vibes": "great_vibes/GreatVibes-Regular.ttf",
  "--font-certificate-poppins": "poppins/Poppins-SemiBold.ttf",
  "--font-certificate-libre-baskerville": "libre_baskerville/LibreBaskerville-Variable.ttf",
  "--font-certificate-allura": "allura/Allura-Regular.ttf",
  "--font-certificate-parisienne": "parisienne/Parisienne-Regular.ttf",
};

// The fallback CertificateDocumentCore applies when a field's font key is
// empty or unknown — always embedded so the serialized template can render.
const DEFAULT_FONT_VAR = "--font-certificate-source-serif";

// usedFontCssVars collects the fonts a layout actually renders. Embedding all
// ten fonts unconditionally pushed the design-save PUT body past nginx's
// request-size limit (~1.6MB of base64); gating keeps it to a few hundred KB.
export function usedFontCssVars(layout: CertificateLayout): Set<string> {
  const used = new Set<string>([DEFAULT_FONT_VAR]);
  for (const field of layout.fields) {
    const match = /^var\((--.+)\)$/.exec(fonts[field.font || ""] || "");
    if (match) used.add(match[1]);
  }
  return used;
}

// fontFaceCSS embeds the used fonts as base64 data: URIs so the stored
// template is genuinely self-contained — the worker's Gotenberg call makes no
// network request for typography (only for the {{certificate_*}} image tokens
// it substitutes with presigned object-storage URLs). :root declarations stay
// for every font: they're bytes, and Core's var() references stay resolvable.
export function fontFaceCSS(used: Set<string>): string {
  const publicDir = path.join(process.cwd(), "public", "fonts");
  const rules: string[] = [];
  for (const [cssVar, relPath] of Object.entries(FONT_FILES)) {
    if (!used.has(cssVar)) continue;
    const bytes = fs.readFileSync(path.join(publicDir, relPath));
    const base64 = bytes.toString("base64");
    rules.push(
      `@font-face{font-family:"${cssVar}";src:url(data:font/ttf;base64,${base64}) format("truetype");font-display:swap;}`
    );
  }
  for (const cssVar of Object.keys(FONT_FILES)) {
    rules.push(`:root{${cssVar}:"${cssVar}";}`);
  }
  return rules.join("\n");
}
