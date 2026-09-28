import { describe, it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { CertificateDocumentStatic } from "./CertificateDocumentStatic";
import type { CertificateLayout } from "@/lib/types";

// These tests guard the SERIALIZED markup (what Gotenberg renders to PDF):
// the stored template ships no stylesheet, so every layout-critical style
// must be inline. A Tailwind class here silently vanishes in the PDF — that
// was the "centered text renders left + 2-page PDF" production bug.

const layout: CertificateLayout = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [
    {
      id: "title",
      kind: "text",
      content: "CERTIFICATE OF COMPLETION",
      x_mm: 40,
      y_mm: 60,
      w_mm: 200,
      align: "center",
      size_pt: 26,
      visible: true,
    },
    {
      id: "logo",
      kind: "image",
      x_mm: 130,
      y_mm: 30,
      w_mm: 30,
      h_mm: 30,
      align: "center",
      visible: true,
    },
  ],
};

function serialize(backgroundUrl?: string | null, assetUrls: Record<string, string> = {}) {
  return renderToStaticMarkup(
    <CertificateDocumentStatic layout={layout} values={{}} backgroundUrl={backgroundUrl} assetUrls={assetUrls} />
  );
}

describe("CertificateDocumentStatic markup", () => {
  it("inlines position:relative + overflow:hidden on the page (single-page PDF)", () => {
    const html = serialize();
    const page = /<div[^>]*data-testid="certificate-document-page"[^>]*>/.exec(html)?.[0] ?? "";
    expect(page).toContain("position:relative");
    expect(page).toContain("overflow:hidden");
  });

  it("inlines absolute cover positioning on the background image", () => {
    const html = serialize("{{certificate_background_url}}");
    const img = /<img[^>]*data-testid="certificate-field-editor-background"[^>]*>/.exec(html)?.[0] ?? "";
    expect(img).toContain('src="{{certificate_background_url}}"');
    expect(img).toContain("position:absolute");
    expect(img).toContain("object-fit:cover");
    expect(img).toContain("width:100%");
  });

  it("inlines display:block so text-align:center takes effect in the PDF", () => {
    const html = serialize();
    const span = /<span[^>]*data-testid="certificate-field-value-title"[^>]*>/.exec(html)?.[0] ?? "";
    expect(span).toContain("display:block");
    expect(span).toContain("text-align:center");
  });

  it("inlines nowrap on the field box", () => {
    const html = serialize();
    const box = /<div[^>]*data-testid="certificate-field-box-title"[^>]*>/.exec(html)?.[0] ?? "";
    expect(box).toContain("white-space:nowrap");
  });

  it("inlines contain sizing on image-field assets", () => {
    const html = serialize(null, { logo: "{{certificate_asset_logo}}" });
    const img = /<img[^>]*src="{{certificate_asset_logo}}"[^>]*>/.exec(html)?.[0] ?? "";
    expect(img).toContain("object-fit:contain");
    expect(img).toContain("width:100%");
    expect(img).toContain("height:100%");
  });
});
