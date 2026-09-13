# Repo Map

`app/` is the real git repository. The parent directory contains mockups, backups, and local workspace material.

## Backend

- `backend/cmd/api`: API entrypoint.
- `backend/cmd/worker`: background worker entrypoint.
- `backend/internal/server`: Echo setup, middleware, route registration.
- `backend/internal/handler`: HTTP request/response mapping.
- `backend/internal/service`: business rules and orchestration.
- `backend/internal/repository`: SQL persistence and query shaping.
- `backend/internal/model`: shared domain models.
- `backend/internal/adapter`: external providers such as Midtrans, Biteship, SMTP, storage.
- `backend/db/migrations`: database migrations.
- `backend/integration`: API-level integration tests.

## Frontend

- `web/app`: Next.js route tree.
- `web/components`: reusable UI and feature components.
- `web/lib/hooks`: API hooks and React Query integration.
- `web/lib/api.ts`: shared API client.
- `web/lib/types.ts`: frontend wire/domain types.
- `web/e2e`: Playwright specs.

## Deploy

- `deploy/pipeline`: CI pipeline scripts.
- `deploy/compose`: local, staging, and production compose manifests.
- `deploy/nginx`: staging and production front door configs.
- `deploy/alloy` and `deploy/grafana`: observability assets.

## Current Important Shape

- Production and staging serve web and API through one host; `/api/` proxies to Go.
- Web builds still inline `NEXT_PUBLIC_API_BASE_URL`.
- Backend tests use many Postgres testcontainers and can take several minutes.
- Some frontend tests spawn Go sanitizer bridge tests, so Go env matters even in `web/`.
