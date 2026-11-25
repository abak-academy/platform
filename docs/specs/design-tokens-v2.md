---
title: Design tokens v2 and UI primitives restyle
issue: none yet
status: agreed
created: 2026-11-17
updated: 2026-11-26
sources: ["Claude artifact 'Abak Academy Design System' https://claude.ai/artifact/7vi4s85XFJ3BZfK1cpeiyp (private; tokens.json, README.md, components/*/README.md)", "docs/prototypes/redesign-v2/index.html", "docs/prototypes/redesign-v2/COMPONENTS.md", "docs/specs/admin-dashboard-v2.md", "web/app/globals.css", "web/app/layout.tsx", "web/components/shell/AppShell.tsx", "web/components/ui/button.tsx", "web/components/ui/input.tsx", "web/components/ui/card.tsx", "web/components/ui/badge.tsx", "web/components/ui/tabs.tsx", "web/components/ui/select.tsx", "web/components/ui/popover.tsx", "web/components/ui/dialog.tsx"]
---

# Design tokens v2 and UI primitives restyle

**This file is the in-repo source of truth for v2 tokens.** It matches `tokens.json` of the design system built from the prototype on 2026-11-25 ("Abak Academy Design System", Claude artifact https://claude.ai/artifact/7vi4s85XFJ3BZfK1cpeiyp, private). Use the artifact as the **visual reference** for components (21 component pages, logos, cover); when the artifact and this file disagree, fix one of them in the same PR, never silently pick.

Shipped by PR 1 (`admin-dashboard-v2.md`, task T1); D3 (`ember-*`) and D14 (full token set, radius repoint) of that spec are settled (Panca, 2026-11-26).

## Problem and outcome

Every later redesign phase needs the same colors, type, spacing, radii, and component shapes. If each phase picks values from the prototype (about 35 font sizes, a dozen radii), pages drift apart. That is the inconsistency admin already has.

Outcome: `globals.css` exposes the v2 tokens below, `layout.tsx` loads the display font, and `web/components/ui` primitives render in the v2 style. Pages that use the primitives change look without page-level edits. New code uses only named tokens: **if a value is missing, add a token here instead of typing a number.**

## Agreed behavior

### Principles (from the DS README)

1. One pattern, one component; a variation is a prop, never a restyled copy in a page.
2. Tokens only: no hex, pixel sizes, or radii in pages. Type from the 12 styles, spacing from `space-*`, corners from `radius-*`.
3. Accent is for action: `ember-600` = primary CTA, live badges, counts, highlights. Selection, navigation, and focus are always `brand-600`.
4. Tabs switch views of one record; filters filter lists.
5. Every list = ListToolbar + Table or grid + EmptyState.
6. Create/edit with more than three fields = Dialog or a dedicated page.
7. Motion is decoration and is off under `prefers-reduced-motion`.

### Naming: prototype vs production

| Prototype CSS (`index.html`) | Production / this spec |
|---|---|
| `--canvas` (page background) | `paper` |
| `--paper`, `--surface` (white card) | `surface` |
| `--ink`, `--ink-2`, `--ink-muted` | `ink-900`, `ink-700`, `ink-600` |
| `--brand`, `--brand-hover`, `--brand-bright`, `--brand-soft`, `--brand-deep` | `brand-600`, `brand-700`, `brand-500`, `brand-50`, `brand-900` |
| `--navy-soft`, `--navy-ink` | `brand-100`, `brand-800` |
| `--ember`, `--ember-bright`, `--ember-soft`, `--ember-ink` | `ember-600`, `ember-500`, `ember-50`, `ember-700` |
| any `--<tone>-soft` / `--<tone>-ink` | `<tone>-50` / `<tone>-700` |

Watch the swap: the prototype's `--paper` is **white**; production `paper` is the **lavender canvas**. Reading prototype CSS literally puts cards on the wrong colour.

**Terracotta is `ember-*`, not `accent-*`** (settled 2026-11-26): `--color-accent` / `bg-accent` is already shadcn's neutral hover (`--accent: var(--color-brand-50)`), used by six primitives. Every earlier mention of `accent-50/500/600/700` in specs means `ember-50/500/600/700`, plus `on-ember` for text on it.

### Color

Proportion on any screen: ~50% neutral (`paper`, `surface`, `ink-*`), ~30% brand (`brand-*`), ~20% ember (`ember-*`). Light first; dark is the same structure, not an inversion (brand and ember lighten, `on-brand`/`on-ember` turn dark, `deep` flips from `brand-900` to `brand-600`).

"Status" = change vs `globals.css` on `feat/public-landing` @ 34ea5d32.

| Token | Light | Dark | Status | Use |
|---|---|---|---|---|
| `paper` | `#f4f5fd` | `#161225` | unchanged | Page canvas behind every card and the app shell. Never a card fill |
| `surface` | `#ffffff` | `#211b34` | **repoint** dark `#27213b`→`#211b34` | Cards, sidebar, inputs, popovers, dialogs, table bodies |
| `surface-raised` | `#ffffff` | `#27213b` | new | A surface on a surface in dark mode (nested panel, hovered menu) |
| `surface-sunken` | `#f0f1fa` | `#1b162d` | new | Wells: table head, progress track, row hover, date tile, icon tiles |
| `line` | `#e7e9f5` | `#393f60` | unchanged | Hairlines: card border, dividers, control border at rest. Decorative only |
| `line-strong` | `#9da3c2` | `#848bb0` | new | Control border on hover, switch track off. Pair with a label |
| `ink-900` | `#15183a` | `#eeeffc` | unchanged | Headings, values, primary text (≥12:1) |
| `ink-700` | `#353a60` | `#d6daef` | unchanged | Body copy in tables and lists, nav rows at rest (≥9:1) |
| `ink-600` | `#565c84` | `#9fa6c6` | **repoint** dark `#bac0dc`→`#9fa6c6` | Muted text: meta, helper, placeholders, column heads (≥5.7:1) |
| `brand-50` | `#eef0fe` | `rgba(81,97,232,.22)` | unchanged | Selected background: active nav row, active chip, hovered row, input focus halo |
| `brand-100` | `#e0e4fd` | `rgba(81,97,232,.22)` | **repoint** dark alpha `.34`→`.22` | Navy chip background |
| `brand-400` | `#6e7cee` | `#6e7cee` | unchanged | Light stop of the brand hero gradient |
| `brand-500` | `#5161e8` | `#9ca6f6` | **repoint** dark (was the light value) | Decorative blobs, chart highlight |
| `brand-600` | `#3d4ddb` | `#7d89f2` | unchanged | THE brand colour: selection, navigation, focus ring, secondary button, bars |
| `brand-700` | `#2e3bb8` | `#9ca6f6` | **repoint** dark `#aab2f8`→`#9ca6f6` | Hover fill of a brand-600 button |
| `brand-ink` | `#2e3bb8` | `#b3bbf8` | new | Brand text and links on surface and brand-50 (≥7.1:1) |
| `brand-800` | `#232c8c` | `#c4cafb` | **repoint** dark (was the light value) | Navy chip text on brand-100 |
| `brand-900` | `#1a2168` | `#0f0c1f` | **repoint** dark (was the light value) | Hero gradient dark stop, footer/CTA bands, wordmark in light |
| `on-brand` | `#ffffff` | `#10132e` | new | Text/icons on a brand-600 fill |
| `deep` | `#1a2168` | `#7d89f2` | new | Small inverse marks: timer pill, chart tooltip, code chip |
| `on-deep` | `#ffffff` | `#10132e` | new | Text on `deep` |
| `ember-50` | `#fbe6da` | `#3a2620` | new | Accent tint: ember chip, EmptyState tile, KPI tone ember |
| `ember-500` | `#e2683c` | `#e8743f` | new | Notification dot, hatch stripe. Never text |
| `ember-600` | `#c24e26` | `#f08a5d` | new | THE accent: primary CTA fill, live badge, count pills. Never selection/navigation |
| `ember-700` | `#a4401c` | `#f4a07a` | new | Accent text on ember-50 and surface, eyebrows |
| `on-ember` | `#ffffff` | `#1b0e07` | new | Text on an ember-600 fill |
| `teal-50` / `-600` / `-700` | `#dcf0ec` / `#1e978a` / `#146b61` | `#173733` / `#2bb3a3` / `#5cc9b9` | new | Success: tint / fill (bars, done step, ring arc) / text |
| `gold-50` / `-600` | `#fbefd4` / `#d99a2b` | `#3a3020` / `#e2ac48` | new | Pending/warning tint / fill (medals, pending banner) |
| `gold-700` | `#7a5410` | `#ebc56e` | **repoint** `#6f5209`/`#ddc46e` → these | Gold text on gold-50 |
| `danger-50` / `-600` | `#fbe7e4` / `#b8352a` | `#3d2224` / `#f08c80` | new | Error tint / error text, destructive fill, down arrow |
| `on-danger` | `#ffffff` | `#1b0e07` | new | Text on danger-600 |
| `sky-50` / `-600` / `-700` | `#e6eefc` / `#2f6fde` / `#2153b0` | `#172d4a` / `#6fa2f2` / `#9cc0f7` | new | Info / in progress: tint / fill / text |
| `berry-50` / `-600` / `-700` | `#fbe6ee` / `#c23b6b` / `#a02a56` | `#3a1f2c` / `#ec7ea4` / `#f3a6c0` | new | Second category (physical goods, 2nd chart series) |
| `mute-50` / `-700` | `#f0f1fa` / `#565c84` | `#2a2442` / `#bac0dc` | new | Neutral chip (Menunggu hasil, Terlewat, Draft) |
| `hero-a` / `-b` / `-c` | `#fde6d8` / `#e0e4fd` / `#eef0fe` | `#2c1a1f` / `#1f1d4f` / `#1c1838` | new | Radial glows in landing/page heroes |
| `scrim` | `rgba(10,24,32,.5)` | same | new | Backdrop behind dialogs and the drawer (6px blur) |

Usage rules:
- Page `paper`; cards, sidebar, inputs, popovers, dialogs `surface`; wells and table heads `surface-sunken`. Never a card on a card.
- Text `ink-900` headings/values, `ink-700` body in lists/tables, `ink-600` muted.
- Brand fills (`brand-600` + `on-brand`): active tab, active nav tile, switch on, selected chip, secondary button. `brand-50` behind selected rows; `brand-ink` for links.
- Ember fill (`ember-600` + `on-ember`): primary button, live/count pills. `ember-50` + `ember-700`: soft accent chips.
- Status always carries a word, never colour alone: success `teal`, pending `gold`, error `danger`, info/in progress `sky`, neutral `mute`; categories `brand`, `navy` (`brand-100`/`brand-800`), `berry`. Each tone = `-50` background + `-700` text; `-600` is for bars, dots, arcs.
- Student hero band (HeroCarousel): fixed gradient `brand-400` → `#3d4ddb` → `#1a2168` in both themes (the literal stops are the light `brand-600`/`brand-900`; define them as one `--gradient-hero` token in `globals.css`, not in components).

**Legacy tokens not in the DS** (`surface-2`, `ink-500`, `ink-400`, `line-2`, `--color-info|warn|danger|success|violet|gold(-bg)`, `--color-submit*`, `--sh-sm|md|lg`): kept with their current values so pre-v2 pages do not change; v2 components never use them. `ink-500`/`ink-400` fail AA as text (`ink-500` is used in ~110 files, `ink-400` in ~50); replacing them is a later per-page cleanup, not this phase.

### Typography

Families: **Plus Jakarta Sans** 500–800 (`display`: headings, buttons, chips, nav, numbers; new, via `next/font/google` as `--font-display`), **Public Sans** 400–700 (`body`, existing `--font-sans`), **Spline Sans Mono** 500–600 (`mono`, existing `--font-mono`; overlines and column heads only). Numbers in tables and KPIs use tabular figures.

Twelve styles; nothing else. Ten sizes: 64, 44, 30, 20, 16, 15, 14, 13, 12, 11.

| Style | Family | Size / line-height | Weight | Tracking | Phones | Use |
|---|---|---|---|---|---|---|
| `display-xl` | display | 64 / 1.02 | 800 | -0.045em | `clamp(42px, 5.4vw, 64px)` | Landing hero headline only (see D1) |
| `display` | display | 44 / 1.08 | 800 | -0.04em | 30 | Landing section titles, CTA band, story headings |
| `h1` | display | 30 / 1.15 | 800 | -0.035em | 24 | One per page: page title; KPI values; ring centres |
| `h2` | display | 20 / 1.3 | 800 | -0.02em | — | Section and dialog titles |
| `h3` | display | 16 / 1.35 | 800 | -0.01em | — | Card titles, empty-state title, large button |
| `label` | display | 14 / 20px | 700 | -0.005em | — | Default button, nav row, menu item, row titles |
| `label-sm` | display | 13 / 18px | 700 | 0 | — | Small button, tab, filter chip, field label |
| `label-xs` | display | 12 / 16px | 700 | 0 | — | Badge, count pill, step labels |
| `body` | body | 15 / 1.55 | 400 | 0 | — | Default copy, inputs, dialog body |
| `body-sm` | body | 13 / 1.5 | 400 | 0 | — | Table cells, secondary lines, page subtitle |
| `caption` | body | 12 / 1.4 | 400 | 0 | — | Meta, helper/error text, comparisons, axes |
| `overline` | mono | 11 / 1.4 | 600 | 0.12em, UPPERCASE | — | Eyebrows, column heads, group labels, counts |

Only the three display-family sizes shrink on phones: `display-xl` with the clamp above; `display` and `h1` step down below 680px. Sentence case everywhere except `overline` and the invoice stamp.

### Spacing

4px base; only these steps for padding, gaps, margins: `space-1` 4 · `space-2` 8 · `space-3` 12 · `space-4` 16 · `space-5` 20 (card padding, section gap) · `space-6` 24 (dialog padding) · `space-8` 32 (desktop gutter) · `space-10` 40 · `space-14` 56 (landing section gap) · `space-20` 80 (landing section padding, main-column bottom).

### Radius

| Token | Value | Use |
|---|---|---|
| `radius-sm` | 10px | Small tiles: date tile, course cover, icon tiles |
| `radius-md` | 14px | Form inputs and selects, nav rows, menu items |
| `radius-lg` | 20px | Popovers, user menu, banners, shipping strip |
| `radius-card` | 22px | Every card, toolbar card, table container |
| `radius-modal` | 28px | Dialogs |
| `radius-panel` | 30px | Hero carousel, large landing panels |
| `radius-pill` | 9999px | Buttons, chips, badges, tabs, search, progress, counts |

**Radius repoint (settled 2026-11-26):** production `--r-sm/--r-md/--r-lg` change from 9/13/18 to **10/14/20**, so every existing `rounded-sm|md|lg` and the shadcn `--radius` (= `--r-md`) follow. `--r-xl` (26) becomes 28 per the map below (26–28 → `radius-modal`).

### Elevation and borders

| Token | Light | Dark | Use |
|---|---|---|---|
| `shadow-card` | `0 1px 2px rgba(40,46,110,.05), 0 6px 18px rgba(40,46,110,.07)` | `0 1px 2px rgba(0,0,0,.3), 0 8px 24px rgba(0,0,0,.28)` | Resting cards, always with a 1px `line` border |
| `shadow-pop` | `0 18px 50px rgba(26,33,104,.18)` | `0 20px 60px rgba(0,0,0,.55)` | Popover, menu, dialog, drawer, chart tooltip |
| `shadow-cta` / `-hover` | `0 6px 18px -6px rgba(194,78,38,.55)` / `0 12px 26px -8px rgba(194,78,38,.6)` | same | Primary (ember) button at rest / lifted |
| `shadow-brand` | `0 6px 18px -6px rgba(61,77,219,.55)` | same | Brand button, active nav tile |

No other shadows. Borders: 1px `line` for cards, rows, dividers; 1.5px for interactive control outlines; dashed 1px `line` only for in-popover separators and the waybill chip.

### Layout and sizes

| Token | Value | Use |
|---|---|---|
| `container-max` | 1480px | Max content width, centred in the main column. **Replaces `max-w-[1340px]` in `AppShell`** (settled 2026-11-26, admin and student) |
| `gutter` / `gutter-mobile` | 32px / 16px | Main-column horizontal padding; 16px below 680px |
| `sidebar-width` | 264px | App sidebar ≥1024px; same-width off-canvas drawer below (replaces today's 252px + 64px rail) |
| `dialog-width` / `dialog-wide-width` | 480px / 820px | Default / wide Dialog (invoice preview; body scrolls inside) |
| `popover-width` | 420px | Filter popover (min with 90vw) |
| `menu-width` | 280px | User menu, dropdown menus |
| `control-sm` / `-md` / `-lg` | 34 / 42 / 52px | Button and pill heights. Nothing in between |
| `input-height` | 46px | Form inputs and selects |
| `nav-row` | 44px | Sidebar row and group header (38px for nested rows) |
| `icon` | 20px | Default icon; 16 small, 26 large, 48 empty state |
| `z-popover` < `z-menu` < `z-drawer` < `z-dialog` | 30 < 40 < 50 < 70 | Stacking. The Dialog moves to 70 (shadcn default is 50, which would tie with the drawer) |

Breakpoints: ≥1200 full grids · <1200 KPI rows drop to 3 and two-column dashboard rows stack · <1024 sidebar becomes the drawer (scrim, `z-drawer`, close button) · <680 single column, toolbar search full width, Filter popover docks to the bottom, display/h1 shrink.

Page templates: admin list = page header + ListToolbar + Table/grid + EmptyState; admin detail = back link + header + Tabs (views) + cards; admin editor = form cards left, sticky publish card right; student home per `student-dashboard-v2.md`. Page header: title `h1`, subtitle `body-sm` `ink-600`, at most one primary action.

### States, focus, motion

- Focus ring: 2.5px solid `brand-600`, 2px offset. Inputs: `brand-600` border + 4px `brand-50` halo instead.
- Hover: buttons lift -2px, KPI cards -3px; rows tint `surface-sunken` (neutral) or `brand-50` (navigable). Disabled: 45% opacity, no shadow, `not-allowed`. Busy: label changes ("Memproses…"), skeletons on `surface-sunken`.
- Motion: spring `cubic-bezier(.34,1.56,.64,1)` for hover lifts, switch thumbs, popover/dialog entrance (.3–.4s); standard `cubic-bezier(.22,1,.36,1)` for panels, accordions (.35s), carousel slides (.5s), progress (.5s); colour/border .15s; live pulse 1.3s on `live` badges only. All off under `prefers-reduced-motion` (carousels stop autoplay).

### Primitives

Component pages in the artifact are the visual contract; this table is the API delta for `web/components/ui`.

| Primitive | v2 |
|---|---|
| `Button` | Pill. Variants: `default` = **ember** primary (+ `shadow-cta`; one per view), `secondary` = brand solid (+ `shadow-brand`), `outline` (1.5px `line`, hover border `ink-900`), `ghost` (hover `surface-sunken`), `destructive` = `danger-600` (Dialog confirmations only), `light`/`line-light` (only on the hero gradient). Sizes `sm` 34 / default 42 / `lg` 52. Optional trailing CTA dot (marketing only). Icon button: 42px circle (32 `sm`), `aria-label` required, optional ember count pill or `ember-500` dot. No ember button below 14px bold. |
| `Input` | `input-height` 46, `radius-md` (search inputs are pills), focus = brand border + `brand-50` halo. |
| `Card` | `radius-card`, 1px `line`, `shadow-card`, padding `space-5`. |
| `Badge` | 26px pill, `label-xs`, tone = `-50` bg + `-700` text: teal, gold, danger, sky, mute, brand (`brand-50`/`brand-ink`), navy, berry, ember; `live` = ember solid + pulsing dot; `count` = 20px ember pill. |
| `Tabs` | Segmented pill group; active = brand solid. Views of one record only. |
| `Select` | Pill trigger (`control-md`) in toolbars, `radius-md` + `input-height` in forms. |
| `Popover` | `radius-lg`, `shadow-pop`, `popover-width`, spring entrance, `z-popover`. |
| `Dialog` | `radius-modal`, padding `space-6`, `shadow-pop`, `scrim` + blur, `z-dialog`. `size: "default" \| "wide"` (480 / 820, wide body scrolls inside, header/footer fixed). Header: `h2` title + 42px close icon button (`aria-label="Tutup"`). |

Icons: the artifact documents the custom 96-icon set; keep `lucide-react` for now. An icon-set swap is a separate decision (touches every icon import).

### Mapping stray values to tokens

When porting from the prototype, snap every value; never copy it.

| Prototype value | → Token |
|---|---|
| Font 9.5–12.5 in mono | `overline` |
| 10.5–12 bold display (counts, chips, tab counts) | `label-xs` |
| 11–12.5 regular (meta, helper, legend, axis) | `caption` |
| 13–13.5 bold display (tabs, field labels, small buttons) | `label-sm` |
| 13–14.5 regular (tables, `.small`, subtitles) | `body-sm` |
| 14–14.5 bold display (buttons, nav rows, menu items) | `label` |
| 15–15.5 regular | `body` |
| 15 bold, 16, 17 (card heads, banners, lg button) | `h3` |
| 18–22 | `h2` |
| 24–34 (page titles, KPI values, ring centres) | `h1` |
| 36–46 | `display` |
| clamp(42–72), 58, 64, 66 | `display-xl` |
| 8.5 mono (ACADEMY in the wordmark) | part of the logo lockup |
| 150–300 (giant numerals, watermark) | illustration, drawn as SVG |
| Spacing 2–5 / 6–9 / 10–13 / 14–17 / 18–21 / 22–27 / 28–35 / 36–47 / 48–64 / 72+ | `space-1` / `2` / `3` / `4` / `5` / `6` / `8` / `10` / `14` / `20` |
| Radius 6–12 / 13–16 / 18–20 / 22–24 / 26–28 / 30–34 / 999px or 50% | `radius-sm` / `md` / `lg` / `card` / `modal` / `panel` / `pill` (circles stay 50%) |

A control's own geometry (a 5px tab-track inset, 18px button side padding) stays as the component defines it. Organic blob shapes on marketing pages are illustration, not radii.

### Contrast flags

Known limits, by design; reviewers do not re-raise them, implementers must respect them:
- `line` (1.2:1 light) is decorative; a control must have a visible label or a 1.5px border, never `line` alone as its boundary.
- `line-strong` is 2.5:1 on surface (under the 3:1 non-text floor); always paired with a label.
- `gold-600` (2.4:1) and `teal-600` (3.6:1) are fills only, never text on white; text on gold fill is `ink-900`.
- White on `ember-600` is 4.8:1: AA (4.5:1) at any size, with little margin, so the DS rule keeps ember **buttons** at ≥14px bold; 12px/700 count pills are still AA. Never put ember-600 text on `ember-50` (use `ember-700`).
- Hero gradient: white text 6.4:1 at the middle stop, 3.6:1 at the light top-right corner; body copy left/bottom, title large.
- Dark repoints change `brand-800`/`brand-900` from the light values to `#c4cafb`/`#0f0c1f`: the 3 files using `*-brand-800` and the 1 using `*-brand-900` must be checked in dark mode in PR 1.

## Technical contract

- Colours, radii, shadows: `web/app/globals.css` `@theme` block + the `html[data-theme="dark"]` block. No new CSS file. `--gradient-hero` and `--z-*` live in `:root` (Tailwind has no theme namespace for them; use `z-(--z-dialog)`).
- Type: one `@utility type-<style>` per style (family, size, line-height, weight, tracking, phone step) so a page writes `type-h1`, not five utilities. The `type-` prefix avoids colliding with Tailwind's `text-*` scale.
- Spacing: Tailwind v4's numeric scale already equals `space-N` (`p-4` = 16px); the rule is "only steps 1, 2, 3, 4, 5, 6, 8, 10, 14, 20". No new spacing tokens.
- Sizes and widths: `--spacing-control-sm|md|lg`, `--spacing-input`, `--spacing-nav-row`, `--spacing-sidebar`; `--container-page` (1480px; not `container-max`, because `max-w-max` is a built-in Tailwind utility), `--container-dialog`, `--container-dialog-wide`, `--container-popover`, `--container-menu`. Breakpoints 1200 and 680 are added as named `--breakpoint-*` entries (1024 = existing `lg`).
- `layout.tsx`: Plus Jakarta Sans 500–800 via `next/font/google` as `--font-display`. No new dependency.
- `components/ui/*` keep their public props; `Dialog` gains `size`. `Button` `default` becoming ember changes meaning on every call site: list them in the PR body.
- Admin and affiliate pages use these tokens through the `--md-sys-*` bridge in `admin-theme.css` (`admin-dashboard-v2.md` D2 = a).

## Acceptance

1. Light and dark screenshots of `/admin`, `/admin/orders`, `/`, and `/exam` show the new primitives with no layout breakage.
2. No hex, px font size, or px radius added outside `globals.css` in the diff (grep), except the documented component geometry.
3. Every token in the colour, radius, shadow, layout, size, and z tables exists in `globals.css` with the listed light/dark value (checked against this file in review).
4. Existing component tests pass; class assertions that change are updated in the same PR.
5. Contrast table in the PR body for every `-50`/`-700` badge pair, `on-brand`/`on-ember`/`on-danger` on their fills, `ink-600` on `paper`/`surface`/`surface-sunken`, both themes; values match the flags above.
6. `AppShell` content is capped at 1480px and centred; no `max-w-[1340px]` remains.

## Decisions needed

Recommendation first.

- **D1 — Landing hero title size.** The 2026-11-26 landing hero title is `clamp(44px, 6.4vw, 92px)` (up to ~92px, line-height 1, tracking -0.05em), larger than `display-xl` (64).
  - a) **Add `display-hero`** (display family, 800, `clamp(44px, 6.4vw, 92px)` / 1.0, -0.05em), landing hero only; `display-xl` stays 64 for other marketing headlines. Keeps the approved hero; one extra style.
  - b) Cap at `display-xl` 64 and ask Panca to re-approve the smaller hero. Keeps the twelve-style rule intact.
- ~~D2 — Terracotta name.~~ **Settled (Panca, 2026-11-26):** `ember-*` + `on-ember`.
- ~~D3 — Scope in PR 1.~~ **Settled (Panca, 2026-11-26):** the full token set incl. the radius repoint ships in PR 1.

## Non-goals

- Page layout changes; those belong to the phase specs.
- Icon-set swap; replacing legacy `ink-500`/`ink-400`/status tokens on existing pages.
