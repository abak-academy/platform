// Explicit React import: scripts/build-card-template.mjs renders this file
// through tsx, which compiles JSX with the classic runtime, so the automatic
// runtime Next relies on is not available there.
import React from "react";

/**
 * The Abak Academy mark — the one definition every surface draws from.
 *
 * Before this existed there were four hand-maintained copies and they had
 * already drifted into two different logos: the favicon and the app shell drew
 * rounded-square heads while the login panel and the exam-card PDF drew
 * circles. Circles are canonical (client, 2026-08-03).
 *
 * Derived artifacts — public/brand/*.png, public/brand/logo.svg and
 * app/icon.svg — are generated from this file by scripts/build-brand-assets.mjs,
 * and the exam card's Go-side fallback SVG by scripts/build-card-template.mjs.
 * Edit the geometry here and re-run both; never hand-edit an output.
 *
 * `primary` colours the parent figure only. The child's teal and the cap's
 * amber are fixed brand colours and do not vary by surface.
 */
export function AbakLogo({
  size,
  primary = "currentColor",
  className,
}: {
  /** Omit to let CSS size the svg — the exam card does that. */
  size?: number;
  primary?: string;
  className?: string;
}) {
  return (
    <svg
      {...(size ? { width: size, height: size } : null)}
      viewBox="0 0 120 120"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-label="abak academy"
      className={className}
    >
      <circle cx="44" cy="34" r="15" fill={primary} />
      <path d="M22 104 Q22 64 44 64 Q66 64 66 104 Z" fill={primary} />
      <path d="M62 104 Q62 78 80 78 Q98 78 98 104 Z" fill="#1E978A" />
      <path d="M80 44 L96 51 L80 58 L64 51 Z" fill="#D99A2B" />
      <circle cx="80" cy="62" r="11" fill="#1E978A" />
      <rect x="79" y="44" width="2.5" height="9" fill="#D99A2B" />
    </svg>
  );
}

/** Brand blue — the parent figure on light surfaces (favicon, PNG exports). */
export const ABAK_BRAND_BLUE = "#3D4DDB";
/** Ink navy — the parent figure on printed documents (exam card, PDFs). */
export const ABAK_PRINT_NAVY = "#22315B";
