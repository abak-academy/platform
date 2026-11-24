---
title: Design tokens v2 and UI primitives restyle
issue: none yet
status: proposed
created: 2026-11-17
sources: ["docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "web/app/globals.css", "web/app/layout.tsx", "web/components/ui/button.tsx", "web/components/ui/input.tsx", "web/components/ui/card.tsx", "web/components/ui/badge.tsx", "web/components/ui/tabs.tsx", "web/components/ui/select.tsx", "web/components/ui/popover.tsx", "web/components/ui/dialog.tsx"]
---

# Design tokens v2 and UI primitives restyle

## Problem and outcome

Every later redesign phase needs the same colors, type, radii, and component shapes. If each phase picks values from the prototype, pages will drift apart. That is the inconsistency we already have in admin.

Outcome: `globals.css` exposes the v2 tokens, `layout.tsx` loads the display font, and the primitives in `web/components/ui` render in the v2 style. Pages that already use the primitives change look without page-level edits.

## Agreed behavior

### Color

The existing `brand-*` indigo scale and `paper`/`surface`/`line`/`ink-*` neutrals stay as they are; they already match the prototype. Add an **accent** (terracotta) family and status families:

| Token | Light | Dark | Use |
|---|---|---|---|
| `accent-600` | `#c24e26` | `#f08a5d` | Primary CTA background, active dot, count badges |
| `accent-500` | `#e2683c` | `#e8743f` | Hover/bright, hatch pattern |
| `accent-50` | `#fbe6da` | `#3a2620` | Soft background (chips, referral box) |
| `accent-700` | `#a4401c` | `#f4a07a` | Accent text on soft background |
| `teal-600` / `-50` / `-700` | `#1e978a` / `#dcf0ec` / `#146b61` | `#2bb3a3` / `#173733` / `#5cc9b9` | Success, done, progress |
| `gold-600` / `-50` / `-700` | `#d99a2b` / `#fbefd4` / `#7a5410` | `#e2ac48` / `#3a3020` / `#ebc56e` | Pending payment, medals, warnings |

Map to the existing status tokens where they exist (`--color-success`, `--color-warn`, `--color-gold`) instead of duplicating them. If a value differs, keep the existing token and note the difference in the PR.

Usage rule (from the 50/30/20 decision): **primary buttons are accent**; **selection and navigation states are brand** (active nav, selected tab, selected option, focus ring, switches, checkboxes); everything else is neutral.

### Type

- Add Plus Jakarta Sans (500–800) via `next/font/google` as `--font-display`. Headings, buttons, chips, and nav use it.
- Body stays Public Sans (`--font-sans`). Overlines and labels use Spline Sans Mono (`--font-mono`) uppercase with 0.12em tracking.
- Display headings use weight 800 with -0.04em tracking.

### Shape and motion

- Radii: buttons, inputs-as-pills, chips, and tabs are fully round (`9999px`). Cards are 22px, large cards and panels 30px, modals 28px.
- Shadow: keep the existing `--sh-*` scale; cards use `sh-sm` plus a 1px `line` border.
- Motion: spring easing `cubic-bezier(.34,1.56,.64,1)` for hover lift (translateY -2 to -6px), standard easing `cubic-bezier(.22,1,.36,1)` for panels. All motion is disabled under `prefers-reduced-motion`.

### Primitives

| Primitive | v2 change |
|---|---|
| `Button` | Pill shape. Variants: `default` = accent, `secondary` = brand solid, `outline` (1.5px line, hover darkens border), `ghost`. Sizes sm 34px / default 42px / lg 52px. Optional trailing "CTA dot" (circle with arrow) for marketing CTAs. |
| `Input` | 46px, 14px radius (search inputs are pills), focus = brand border + 4px brand-50 ring. |
| `Card` | 22px radius, 1px line, sm shadow. |
| `Badge` | Pill chip with soft background + ink text per tone (brand/accent/teal/gold/danger/neutral). A `live` variant is accent solid with a pulsing dot. |
| `Tabs` | Segmented pill group; active = brand solid. Only for switching **views** of one record (for example exam detail tabs), never for filtering lists. |
| `Select` | Pill trigger in toolbars, 14px radius in forms. |
| `Popover` | 20px radius, pop shadow, spring entrance. Used by the list toolbar's Filter panel. |
| `Dialog` | 28px radius, spring entrance, header row with title + close icon button. |

Icons: the prototype's icon set is custom (24px grid, 1.75 stroke, optional duotone fill). Keep `lucide-react` for this phase. Swapping the icon set is a separate decision because it touches every file that imports icons.

## Technical contract

- Tokens live in `web/app/globals.css` under the existing `@theme` block plus the `html[data-theme="dark"]` block. No new CSS files.
- No new dependencies; the display font comes from `next/font/google`, which is already used.
- `components/ui/*` keep their public props. Variant names that already exist keep their meaning, except `Button` `default` becoming accent. List every call site whose meaning changes in the PR body.

## Acceptance

1. Light and dark mode screenshots of `/admin`, `/admin/orders`, `/`, and `/exam` show the new primitives with no layout breakage.
2. No hard-coded hex values are added outside `globals.css` (grep check).
3. Existing component tests pass; snapshot or class assertions that change are updated in the same PR.
4. Contrast: accent-600 on white and white on accent-600 meet WCAG AA for 14px bold text.

## Non-goals

- No page layout changes. Those belong to the later phases.
- No icon set swap.
