---
title: Block browser machine translation site-wide
updated: 2026-10-01
verified_against: bb588028
---

**Status:** accepted (issue #199).

## Context

Google Translate rewrites a page by replacing each text node with a `<font>` element, so React's references point at detached nodes. Updating a detached node is harmless. Removing one throws `NotFoundError` (`removeChild`). That happens whenever a host element's children change shape.

The exam nav button did exactly that on the last question. Next (`[label, <ChevronRight/>]`) became Submit (a bare label) on `isLastQuestion` (`app/(exam-session)/exam/sessions/[id]/page.tsx`). Every translated student crashed into the root error boundary (`app/error.tsx`, "Gagal memuat data") and could not reach Submit. This was reproduced locally in real Chrome: with Translate active on the pre-fix code, the console showed `NotFoundError … The above error occurred in the <button> component`.

## Decision

1. **Opt the whole site out of browser translation.** `web/app/layout.tsx` sets `<html lang="id" translate="no">` and emits `<meta name="google" content="notranslate">` via `metadata.other`. Chrome then does not offer Translate at all.
2. **Keep conditional children shape-stable in exam controls.** The nav button wraps its label in a `<span>` in both branches, so the swap no longer removes a text node. This is defence in depth for clients that ignore the opt-out. The regression test, `page.test.tsx` "survives Google Translate rewriting text nodes…", fails without the span.

## Trade-offs

- Students lose machine translation on every route, not only the exam. The app ships its own ID/EN locales (`web/lib/i18n.ts`, `id` / `en`; switcher in `components/shell/AppHeader.tsx`), and an exam answered through machine translation is no longer the exam that was authored.
- The opt-out is honoured by Chrome. Other translators (browser extensions, other browsers' built-in translation) were not tested and may ignore it. For those clients, rule 2 is the only protection.

## Alternatives rejected

- **Opt out on the exam-session routes only.** This keeps Translate for catalog pages, but any other page with a conditional text swap can crash the same way, and the scope boundary must be maintained by hand.
- **Span-wrapping alone.** This fixes one known button but leaves every other conditional text swap, and the imperative DOM in `MultiBlankInput`, open to the same failure.
- **Rely on the error boundary.** `app/error.tsx` is recovery, not prevention: the student still loses the exam screen mid-attempt and sees a generic "failed to load" message that misdescribes the cause.
