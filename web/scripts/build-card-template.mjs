#!/usr/bin/env node
// Build-time artifact (redesign 2026-08-02, orchestrator decision on the exam
// card's "no design studio" question): the exam card has exactly one
// template, so instead of a per-exam admin-authored design there is exactly
// one static self-contained HTML file, generated from ExamCardPrintable.tsx
// — the SAME component the on-screen student card
// (web/app/(print)/exam/[id]/card/page.tsx) renders — so the two can never
// drift on markup. Run with `node scripts/build-card-template.mjs` (or via
// `npm run build:card-template`) whenever ExamCardPrintable.tsx or its CSS
// module changes; the output is committed, not generated at deploy time,
// because the Go backend embeds it with go:embed.
//
// This is a plain Node script, not a Next.js build step: ExamCardPrintable
// imports a CSS Module, which only Next's webpack loader can normally
// resolve. Rather than pull in webpack, the script writes a throwaway copy of
// the component with that import swapped for an identity Proxy (so
// `styles.card` reads back as the literal string "card") and inlines the
// *raw* CSS file text unchanged — the two agree by construction, since
// nothing here ever hashes a class name.
//
// Every value that varies per participant becomes a {{token}} the worker
// substitutes at generation time (same contract as the certificate
// template): participant_no, student_name, school, exam_title,
// exam_schedule, check_in_code, tenant_name, tenant_logo_url, photo_url,
// footer_note. grade/dob/subject/time_range/duration/mode/platform are not
// part of CardPrintData today (see web/lib/server/print-api.ts's historical
// comment) and render as the same literal "—" the old print route used.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const componentPath = path.join(__dirname, "../components/exam/ExamCardPrintable.tsx");
const cssPath = path.join(__dirname, "../components/exam/ExamCardPrintable.module.css");
const outPath = path.join(__dirname, "../../backend/internal/service/assets/exam_card_template.html");

let src = fs.readFileSync(componentPath, "utf8");
src = 'import React from "react";\n' + src;
src = src.replace(
  'import styles from "./ExamCardPrintable.module.css";',
  'const styles = new Proxy({}, { get: (_t, p) => String(p) });'
);
// Expose the two inline placeholder icons (AbakMarkFull, PhotoPlaceholder) so
// the worker's fallback-icon assets can be generated from the exact same
// markup instead of hand-transcribed duplicates.
src += "\nexport { AbakMarkFull, PhotoPlaceholder };\n";

// Beside the component, not in scripts/, so its relative imports (the shared
// brand mark) resolve from the same directory the original sits in. Only the
// CSS-module import is rewritten above; everything else must resolve for real.
const tmpPath = path.join(__dirname, "../components/exam/.exam-card-printable.generated.tsx");
fs.writeFileSync(tmpPath, src);

try {
  const mod = await import(`${tmpPath}?t=${Date.now()}`);
  const { ExamCardPrintable, AbakMarkFull, PhotoPlaceholder } = mod;

  const DASH = "—";
  const props = {
    fullName: "{{student_name}}",
    participantNumber: "{{participant_no}}",
    school: "{{school}}",
    grade: DASH,
    dob: DASH,
    photoUrl: "{{photo_url}}",
    examName: "{{exam_title}}",
    subject: DASH,
    date: "{{exam_schedule}}",
    timeRange: DASH,
    duration: DASH,
    mode: DASH,
    platform: DASH,
    checkInCode: "{{check_in_code}}",
    tenantName: "{{tenant_name}}",
    tenantLogoUrl: "{{tenant_logo_url}}",
    footerNote: "{{footer_note}}",
  };

  const markup = renderToStaticMarkup(React.createElement(ExamCardPrintable, props));
  const css = fs.readFileSync(cssPath, "utf8");

  // ExamCardPrintable.module.css only sets fonts via next/font CSS vars
  // (--font-inter, --font-cinzel), which the on-screen app supplies globally.
  // This standalone document has no next/font pipeline, so define system-font
  // fallbacks for the same var names rather than leaving them unresolved
  // (an undefined var() invalidates the whole font-family declaration).
  const html = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
:root { --font-inter: system-ui, sans-serif; --font-cinzel: Georgia, serif; }
body { margin: 0; font-family: system-ui, sans-serif; }
${css}
@page { margin: 0; }
</style>
</head>
<body>
${markup}
</body>
</html>
`;
  fs.writeFileSync(outPath, html);

  // Fallback icons for {{photo_url}}/{{tenant_logo_url}} when the worker has
  // no real photo/logo to presign — the same SVGs the on-screen card shows
  // for the same missing-value case, so the PDF's placeholder matches.
  const logoSvg = renderToStaticMarkup(AbakMarkFull);
  const photoSvg = renderToStaticMarkup(PhotoPlaceholder);
  fs.writeFileSync(path.join(__dirname, "../../backend/internal/service/assets/exam_card_logo_fallback.svg"), logoSvg);
  fs.writeFileSync(path.join(__dirname, "../../backend/internal/service/assets/exam_card_photo_fallback.svg"), photoSvg);

  console.log(`wrote ${outPath} (${html.length} bytes)`);
  console.log("wrote exam_card_logo_fallback.svg, exam_card_photo_fallback.svg");
} finally {
  fs.unlinkSync(tmpPath);
}
