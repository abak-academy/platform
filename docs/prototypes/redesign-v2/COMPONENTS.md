# Redesign v2 — component inventory

Maps every recurring pattern in `index.html` to the React component that should own it. When a phase needs a pattern, use or extend the component in this list. Do not restyle the pattern inside a page.

**Status:** *exists* = the component exists, restyle per `design-tokens-v2.md`; *new* = build it once, in the folder shown.

## Rules

1. **One pattern, one component.** If a page needs a variation, add a prop to the component; do not fork it.
2. **Tokens only.** No hex values or arbitrary pixel radii in pages. If a value is missing, add a token.
3. **Accent is for action.** The terracotta accent appears only on primary CTAs, highlights, live badges, and counts. Selection, navigation, and focus states use brand indigo.
4. **Tabs switch views, filters filter lists.** Never use tabs to filter a list.
5. **Every list = ListToolbar + table or grid + the shared empty state.**
6. **Every create/edit flow with more than three fields = Dialog or a dedicated page.** Small actions (password, avatar) use a Dialog with an explicit Save.
7. **Motion is decoration only.** It is disabled under `prefers-reduced-motion`.

## Primitives (`web/components/ui`)

| Prototype class | Component | Status |
|---|---|---|
| `.ab-btn` (primary / dark / outline / ghost / cta-dot) | `Button` | exists |
| `.ab-input`, `.ab-field.pill` | `Input` | exists |
| `.chip.c-*`, `.chip.live` | `Badge` | exists |
| `.card` | `Card` | exists |
| `.seg-tabs` | `Tabs` | exists |
| `select.ab-input`, `.lsort` | `Select` | exists |
| `.fpop-panel` | `Popover` | exists |
| `#modal .modal-card` | `Dialog` | exists |
| `.switch` | `Switch` | **new** (`ui/switch.tsx`, radix) |
| `.av` sprite avatar / uploaded photo | `Avatar` + `AvatarSprite` | exists / **new** |
| `.empty` | `EmptyState` | **new** (`ui/empty-state.tsx`) |
| `.ab-progress` (+ `hatch`) | `Progress` | exists |
| `.donut`, `.score-ring` | `Ring` | **new** (`ui/ring.tsx`) |

## Shared blocks

| Prototype class | Component | Folder | Status |
|---|---|---|---|
| `.lbar` (search · Filter · sort · chips · count) | `ListToolbar` | `components/admin` | **new** (spec `admin-list-toolbar.md`) |
| `.side`, `.side-grp` accordion | `AppSidebar` | `components/shell` | exists (spec `admin-nav-accordion.md`) |
| `.pubnav` top nav (guest + student) | `SiteNav` | `components/shell` | **new** (phase 5) |
| `.umenu` avatar dropdown | `AccountMenu` | `components/shell` | **new** (phase 5) |
| `.site-foot` + `.stats-band.at-end` | `SiteFooter` | `components/shell` | **new** (phase 4) |
| `.app-head` | `AdminPageHeader` | `components/admin` | exists |
| `.kpi` | `StatCard` | `components/admin` | exists |
| `.poster` (edition card) | `EditionCard` | `components/catalog` | **new** |
| `.pkg` (registered exam card) | existing exam card | `components/exam` | exists (restyle) |
| `.s-tile` + `.s-story` | `EventSeriesPicker` + `EventStoryCard` | `components/events` | **new** (phase 3/4) |
| `.ss-sched` timeline | `EventSchedule` | `components/events` | **new** |
| `.floaters` math symbols | `MathFloaters` | `components/brand` | **new** |
| `.lang-btn` | `LanguageToggle` | `components/shell` | **new** (wraps existing `useTranslation`) |

## Page templates

| Template | Used by |
|---|---|
| Admin list = `AdminPageHeader` + `ListToolbar` + table/grid + `EmptyState` | all admin lists |
| Admin detail = back link + `AdminPageHeader` + `Tabs` (views) + cards | exam detail, order detail, event detail |
| Admin editor = two columns: form cards left, sticky "Tampilan/Publikasi" card right with the Save button | course editor, event editor |
| Site page = `SiteNav` + content + `SiteFooter` | landing, event, catalog, all student pages |
| Focus page = minimal header, no nav | exam room |
