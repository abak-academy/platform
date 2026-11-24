# Abak Academy Hub verification map

The maintained source for proving the Hub's user-facing behavior. Read this index, then use the feature file as the recipe.

## Baseline preconditions

- `abak-verify up` ran from this checkout and `abak-verify doctor` prints `OK` with the revision you mean to test.
- Seeded accounts only: `verify-student@example.test` (student) and `verify-admin@example.test` (super_admin). Passwords are in `~/.cache/abak-verify/state/creds.env`.
- The database starts empty apart from migrations and the dev affiliate defaults: no products, courses, tests, or exams.
- Never drive an instance this run did not start (`:3000`/`:8080` belong to the user's dev stack).

## Driving conventions

- One spec per feature in `journeys/<feature>.spec.ts`, run with `abak-verify journey <feature>`.
- Match UI copy in both languages (`/^(Masuk|Sign in)$/`). Indonesian renders by default.
- Prefer roles and accessible names, then `data-testid`. Avoid CSS classes and DOM position.
- Build the data a journey needs through the admin UI or admin API in the journey itself. Do not edit SQL by hand.

## Proof and skip reporting

- Before and after screenshots, an ARIA snapshot of the end state, and the `*.browser.log` for each test.
- Mutations get a second read-only view (an API GET or psql) saved next to the screenshots.
- Report the evidence dir and the revision. An entry point you could not reach is reported with the unmet precondition, never as verified through another path.

## Features

| Feature | Status |
|---|---|
| [Login](./login.md) | Proven 2026-11-21 (`journeys/login.spec.ts`) |
| [Store catalog and cart](./store-catalog-cart.md) | Mapped, no journey yet |
| [Admin test authoring](./admin-test-authoring.md) | Mapped, no journey yet |
| [Student competitions and exams](./student-exam.md) | Mapped, no journey yet |
