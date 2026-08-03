#!/usr/bin/env node
// Build-time artifacts derived from components/brand/AbakLogo.tsx, the single
// definition of the Abak Academy mark. Run `npm run build:brand-assets`
// whenever that component's geometry or colours change; the outputs are
// committed, not generated at deploy time.
//
// Why these exist: the mark used to live in four hand-maintained copies and had
// already drifted into two different logos (rounded-square heads in the favicon
// and app shell, circles on the login panel and exam card). Circles are
// canonical. Generating every raster and standalone-SVG output from one
// component is what stops that happening again.
//
// The PNGs are for surfaces that cannot take markup — print vendors, Canva,
// Word, social, and any image-upload field. In-app documents (certificate,
// exam card) should keep using the inline SVG: it stays sharp at print DPI and
// needs no fetch at render time, whereas a missing PNG degrades to a blank box
// rather than an error.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { chromium } from "playwright";

import { AbakLogo, ABAK_BRAND_BLUE } from "../components/brand/AbakLogo.tsx";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const brandDir = path.join(__dirname, "../public/brand");
const PNG_SIZES = [256, 512, 1024];

fs.mkdirSync(brandDir, { recursive: true });

const svg = renderToStaticMarkup(
  React.createElement(AbakLogo, { primary: ABAK_BRAND_BLUE }),
);

// public/brand/logo.svg — the shareable vector, served at /brand/logo.svg.
fs.writeFileSync(path.join(brandDir, "logo.svg"), svg + "\n");

// app/icon.svg — Next's file-based favicon convention. Sized explicitly
// because a favicon is consumed as a standalone document, not laid out by CSS.
const favicon = svg.replace(
  "<svg ",
  '<svg width="120" height="120" ',
);
fs.writeFileSync(path.join(__dirname, "../app/icon.svg"), favicon + "\n");

const browser = await chromium.launch();
try {
  for (const size of PNG_SIZES) {
    const page = await browser.newPage({
      viewport: { width: size, height: size },
    });
    // omitBackground keeps the PNG transparent; the mark has to sit on dark
    // panels and light documents alike.
    await page.setContent(
      `<body style="margin:0">${svg.replace('<svg ', `<svg width="${size}" height="${size}" `)}</body>`,
    );
    await page.screenshot({
      path: path.join(brandDir, `logo-${size}.png`),
      omitBackground: true,
    });
    await page.close();
  }
} finally {
  await browser.close();
}

console.log(`wrote public/brand/logo.svg, app/icon.svg`);
console.log(`wrote ${PNG_SIZES.map((s) => `logo-${s}.png`).join(", ")}`);
