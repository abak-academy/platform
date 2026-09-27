# Web Tests

Implementation and test guidance for `web/` (Next.js, vitest, Playwright). Existing component style wins when it is more specific.

## Architecture facts

- `web/` has no server components; data fetching is client-side.
- `NEXT_PUBLIC_API_BASE_URL` is baked into the bundle at build time.
- A page file (`app/**/page.tsx`) may export only Next's allowed fields. Put helpers a test needs in a sibling file.

## Unit and component tests

- Assert user-visible behavior: rendered text, roles, enabled state, and calls that leave the component.
- Wait for the effect (`findBy*`, `waitFor` on the DOM), not for the mocked call.
- Fixtures must match the real server payload. Copy the shape from the Go handler's response struct; a Go pointer without `omitempty` is `null`, so type it `T | null`.
- Cover loading, empty, error, and retry states when the component fetches.
- Two tokens with the same value make a theme assertion vacuous; assert something that differs.
- Never paste real names, phone numbers, or emails into fixtures.

## What vitest cannot prove

- Types: run `npx tsc --noEmit` after the last edit.
- Layout: jsdom has no geometry. Overflow, clipping, and wrapping need a real browser, checked in the longest locale.
- A collapsed element passes overflow assertions; assert a visible width first.

## Browser tests

- `npm run test:e2e` runs Playwright specs in `web/e2e/`.
- When changing a unit test's expected behavior, search `web/e2e/*.spec.ts` for the same flow and update it too.

## Gates

See [`.agents/verification.md`](../../.agents/verification.md#frontend-gates).
