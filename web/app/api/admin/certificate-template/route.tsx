import fs from "node:fs";
import path from "node:path";
import { NextResponse } from "next/server";
import { CertificateDocumentStatic } from "@/components/certificate/CertificateDocumentStatic";
import { normalizeCertificateLayout } from "@/lib/certificate-studio";
import type { CertificateLayout } from "@/lib/types";

// This route exists so the certificate design editor (a client component)
// can produce the FE-serialized self-contained HTML the redesign stores in
// exam.certificate_template_html — react-dom/server only runs in a Node
// context, not the browser bundle. It runs once per design save, never per
// student: the worker (backend/internal/service/certificate_generate.go)
// substitutes {{token}} placeholders from verified DB values at generation
// time, so nothing rendered here ever needs a real student's data.
//
// {{score}}, {{student_name}}, etc. are left literal because
// CertificateDocument's previewContent (lib/certificate-studio.ts) replaces
// only tokens present in the `values` map and falls back to the original
// `{{token}}` text otherwise (`values[token] ?? match`) — passing `{}` here
// is what keeps every text token un-substituted in the output. Image src
// attributes don't go through that path (they're plain props), so
// backgroundUrl/assetUrls are set to their own {{certificate_*}} tokens
// below instead.
export const runtime = "nodejs";

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

// fontFaceCSS embeds every certificate font as a base64 data: URI so the
// stored template is genuinely self-contained — the worker's Gotenberg call
// makes no network request for typography (only for the {{certificate_*}}
// image tokens it substitutes with presigned object-storage URLs).
function fontFaceCSS(): string {
  const publicDir = path.join(process.cwd(), "public", "fonts");
  const rules: string[] = [];
  for (const [cssVar, relPath] of Object.entries(FONT_FILES)) {
    const abs = path.join(publicDir, relPath);
    const bytes = fs.readFileSync(abs);
    const base64 = bytes.toString("base64");
    const family = cssVar;
    rules.push(
      `@font-face{font-family:"${family}";src:url(data:font/ttf;base64,${base64}) format("truetype");font-display:swap;}`
    );
    rules.push(`:root{${cssVar}:"${family}";}`);
  }
  return rules.join("\n");
}

export async function POST(req: Request) {
  let body: { layout?: CertificateLayout };
  try {
    body = await req.json();
  } catch {
    return NextResponse.json({ error: "invalid request body" }, { status: 400 });
  }
  if (!body.layout) {
    return NextResponse.json({ error: "layout is required" }, { status: 400 });
  }

  const layout = normalizeCertificateLayout(body.layout);
  const assetUrls: Record<string, string> = {};
  for (const field of layout.fields) {
    if (field.kind === "image") {
      assetUrls[field.id] = `{{certificate_asset_${field.id}}}`;
    }
  }

  // Next's App Router build refuses a *static* import of react-dom/server
  // from a route handler ("You're importing a component that imports
  // react-dom/server... render as a Server Component instead") — a guard
  // aimed at accidental double-bundling in page rendering, which doesn't
  // apply here: this route never renders a page, it serializes a document to
  // a string once per admin save. `webpackIgnore` opts this one import out
  // of that static analysis so Node's own module resolution handles it.
  const ReactDOMServer = await import(/* webpackIgnore: true */ "react-dom/server");
  const markup = ReactDOMServer.renderToStaticMarkup(
    <CertificateDocumentStatic
      layout={body.layout}
      values={{}}
      assetUrls={assetUrls}
      backgroundUrl="{{certificate_background_url}}"
    />
  );

  const html = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
${fontFaceCSS()}
</style>
</head>
<body style="margin:0">
${markup}
</body>
</html>
`;

  return NextResponse.json({ html });
}
