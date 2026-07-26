# Abak Academy — App

Monorepo for the Abak Academy platform: exam engine + ecommerce + video courses.
Requirements live one level up in `../requirements/` (`product-requirements.md`, `technical-requirements.md`, `schema.dbml`).

## What's built

| Phase | Scope | Status |
|---|---|---|
| 1 — Foundation & Auth | OTP registration, JWT auth, RBAC middleware, refresh tokens | ✅ |
| 2 — Store & E-Commerce | Product catalog, cart, Midtrans Snap checkout, transactional-outbox worker, promos, refunds, revenue | ✅ |
| 3 — Student Frontend | Dashboard, catalog, cart, orders, course viewer (YouTube), profile + photo upload, i18n (ID/EN) | ✅ |
| Admin — MD3 shell | Blue/teal MD3 palette, dark mode, AdminPageHeader, role-based nav, super admin system pages | ✅ |
| Admin — Super Admin | User management, audit log, system config (AES-256-GCM encrypted Midtrans keys), store dashboard | ✅ |
| 4 — Exam Engine | Question bank, tryouts, schedules, sessions, auto-grading, essay grading, leaderboard, UTBK/IELTS modes | ✅ |
| Certificates & cards | HTML → PDF via a Gotenberg sidecar, drag-and-drop certificate design editor | ✅ |
| Shipping | Biteship ongkir, physical products, order tracking | ✅ |

## Layout

```
app/
├── backend/            Go module — two binaries
│   ├── cmd/api/            REST API server (:8080); runs DB migrations at boot
│   ├── cmd/worker/         outbox, announcements, bulk-student jobs
│   ├── internal/
│   │   ├── adapter/        external service clients — midtrans, biteship, smtp, notify, fazpass
│   │   ├── handler/        HTTP layer (echo)
│   │   ├── infra/          postgres + redis wiring, JWT, migration runner
│   │   ├── model/          shared types
│   │   ├── repository/     pgxpool data access
│   │   ├── server/         echo instance, middleware, /api/v1 routes
│   │   ├── service/        business logic (incl. certificate/card HTML + the Gotenberg client)
│   │   └── worker/         poll loops
│   └── db/
│       └── migrations/     44 .up.sql / .down.sql pairs
├── web/                Next.js 15 (App Router, TS, Tailwind v4)
│   ├── app/(auth)/         login, register, OTP
│   ├── app/(student)/      dashboard, catalog, cart, orders, courses, profile
│   ├── app/(exam-session)/ chrome-free exam runner
│   ├── app/(admin)/        admin shell + all domain pages
│   └── public/fonts/       certificate typefaces for the design editor (mirrors the backend's embedded copy)
├── deploy/             BUILD + LOCAL DEV — docker-compose.yml, Dockerfiles, CI pipeline scripts
├── deployments/        RUNTIME MANIFESTS — per-environment compose files, nginx configs, secret templates
├── docs/               runbooks/ and backlog/
└── Makefile
```

Layering: `handler → service → repository / adapter`.

### `deploy/` vs `deployments/`

Two directories, two audiences — the names are unfortunately similar:

- **`deploy/`** is what *builds* the app and what you run *locally*: `docker-compose.yml` for the full
  local stack, the Dockerfiles, and `pipeline/*.sh`, which CI invokes directly from
  `.github/workflows/pipeline.yml`.
- **`deployments/`** is what *runs* the app on a server: `app-staging.yaml`, `app-production-app.yaml`,
  the per-environment nginx configs, `secrets/*.example.yaml`, and `storage-cors.json`. These files are
  **hand-placed on the VMs** — there is no git clone on the staging or production boxes — so editing
  them here does not update a running environment.

## Stack

| Layer | Choice |
|---|---|
| Router | echo v4 |
| DB | PostgreSQL 17 via pgx/v5 (raw, no ORM) |
| Migrations | custom runner in `internal/infra/migrate.go` — **not** golang-migrate |
| Cache / idempotency | Redis via go-redis/v9 |
| Logging | stdlib slog (JSON) |
| Payment | Midtrans Snap |
| Shipping | Biteship |
| PDF rendering | Gotenberg sidecar (Chromium HTML → PDF) |
| Object storage | MinIO locally, Google Cloud Storage on staging and production |
| Frontend | Next.js 15 + React 19 + Tailwind v4 |
| State | Zustand (auth, cart, UI/theme/lang) |
| Data fetching | TanStack Query v5 |
| i18n | Custom DICT hook (ID/EN, no external lib) |

### How migrations run

`cmd/api` calls `infra.RunMigrations` at startup. The runner globs `db/migrations/*.up.sql`, sorts by
filename, and applies anything whose filename is not already a row in `schema_migrations` — each file
in its own transaction. It compares **filenames, not version numbers**, so a migration numbered lower
than one already applied still runs. Nothing needs to be applied by hand.

The `migrate-up` / `migrate-down` Makefile targets predate this and shell out to a `golang-migrate`
CLI that is not a dependency of this module. Do not use them: golang-migrate keeps its own
`schema_migrations` table with an incompatible schema, so running it against an app-managed database
conflicts with the runner above.

## Quickstart (Docker)

```bash
# One-time: create your local dev config (gitignored)
cd backend/config/env
mkdir -p dev
cp config.example.yaml dev/config.yaml
cp secrets.example.yaml dev/secrets.yaml   # values match docker-compose as-is

cd ../../../deploy
docker compose up -d   # postgres + redis + minio + gotenberg + api + worker + web
```

- API: `http://localhost:8080/api/v1/health`
- App: `http://localhost:3000`
- Admin: `http://localhost:3000/admin` (login with a super_admin account)
- MinIO console: `http://localhost:9001`

The `web` container is a baked image with no source mount — run `npm run dev` for frontend iteration.

## Local development (without Docker)

**Prerequisites:** Go 1.26+ · Node 20+ · Docker (for infra only)

```bash
# 1. Start infra only
cd deploy && docker compose up -d postgres redis minio gotenberg

# 2. Backend — migrations apply automatically on boot
cd backend
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec   # if on this machine
make api      # :8080
make worker   # separate terminal

# 3. Frontend
cd web && npm install && npm run dev   # :3000
```

## Tests

```bash
# Backend — includes testcontainers-backed integration tests, so Docker must be running
cd backend && go test ./... -race

# The real-Gotenberg certificate render gate is behind a build tag and runs as its own CI job
cd backend && go test -tags gotenberg_integration ./internal/service/

# Frontend
cd web && npx vitest run
```

## Operations

- `docs/runbooks/` — PostgreSQL 16→17 data migration, repository migration to the client org
- `docs/backlog/` — accepted tech debt, with the reasoning for deferring it
