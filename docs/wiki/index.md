# Platform Wiki

Current truth about this system: how it is built, the business rules it enforces, and why key choices were made. It is not a changelog; history lives in Git and PRs.

## Rules

- One topic per page, kebab-case filename, short and dense.
- Every page starts with frontmatter: `title`, `updated: YYYY-MM-DD`, `verified_against: <commit SHA>`.
- A claim about code must name the file or symbol it comes from. Re-check it against the code when editing the page and bump `verified_against`.
- Update an existing page before creating a new one; add every new page below.
- This repo is client-visible: no billing, contract, pricing, credentials, or student PII.

## Where things go

| Kind | Location |
|---|---|
| Decision with lasting trade-offs | `decisions/` |
| Component boundaries, contracts, data ownership, runtime | `architecture/` |
| Business rules, user process, invariants | `domain/` |
| Operating or migration procedure | [`../runbooks/`](../runbooks/) |
| Accepted implementation contract for one change | [`../specs/`](../specs/) |
| Coding and test standards | [`../standards/`](../standards/) |

## Decisions

_None yet._

## Architecture

- [System overview](architecture/system-overview.md) — components, backend layers, route groups, worker loops
- [Infra and deploy](architecture/infra-and-deploy.md) — topology, image build and release flow, CI gates, TLS edge, observability
- [Exam capacity](architecture/exam-capacity.md) — pooling, bcrypt, autosave path, nginx, replica counts, known measurements
- [Certificate rendering](architecture/certificate-rendering.md) — issuance, numbering, direct-HTML Gotenberg render, render gate
- [Shipping and Biteship](architecture/shipping-biteship.md) — quotes, stale-quote voiding, booking, webhooks, fallbacks

## Domain

- [Exam lifecycle and scoring](domain/exam-lifecycle-and-scoring.md) — question formats, sanitisation, access, session lifecycle, grading, proctoring
- [RBAC and accounts](domain/rbac-and-accounts.md) — roles, capabilities, school scope, auth, sessions, passwords, login rate limit
- [Orders and payments](domain/orders-and-payments.md) — cart, checkout, digital purchase claims, order statuses, payment window, payment webhook, refunds, revenue ledger
