---
title: System Overview
updated: 2026-09-25
verified_against: e6cf227c
---

# System Overview

Single-tenant platform selling three product types — exam/competition, video course, physical book/merch — with a Go backend (`backend/`) and a Next.js frontend (`web/`). Not an LMS: no live class, no tutor classroom.

## Components

| Component | Role | Source |
|---|---|---|
| `nginx` | Sole public ingress; TLS termination, reverse-proxies `/api/` to `api` and everything else to `web` | `deploy/nginx`, `deploy/compose/production.yml` |
| `api` | Echo HTTP server, stateless, 2 replicas in prod | `backend/cmd/api/main.go` |
| `worker` | Single long-running process; outbox + job-queue + polling loops | `backend/cmd/worker/main.go` |
| `web` | Next.js app, 3 replicas in prod, no server components (`NEXT_PUBLIC_API_BASE_URL` baked into the client bundle) | `web/` |
| `postgres` + `pgbouncer` | Primary datastore; pooled connection (`DATABASE_URL`, `pgbouncer:6432`) for app traffic, direct connection (`MIGRATION_DATABASE_URL`, `postgres:5432`) for migrations, which the `api` process runs on boot (`backend/cmd/api/main.go:44`) | `deploy/compose/production.yml:33-36` (native on a separate DB host in prod, containerized in `deploy/compose/local.yml`) |
| `redis` | Session store (access/refresh JWT revocation sets, OTP, password-reset tokens) and job/queue support | `backend/internal/service/auth.go` |
| `gotenberg` | Headless-Chromium HTML→PDF sidecar for exam cards and certificates | `backend/internal/service/pdf_generator.go`; see [certificate-rendering.md](./certificate-rendering.md) |
| Object storage | S3-compatible client (`minio-go`); GCS in prod, MinIO in local dev — private bucket (bulk exports, credentials) and a public/avatar prefix served read-only through `GET /api/v1/files/*` | `backend/cmd/api/main.go` (`newStorageClient`), `backend/internal/handler/student_serve_test.go` |
| Biteship / Midtrans | External shipping and payment providers, resolved at runtime from DB or env config | `backend/internal/adapter/biteship.go`, `backend/internal/adapter/midtrans.go`; see [shipping-biteship.md](./shipping-biteship.md) |

Full deploy topology, replica counts, and memory budgets: [infra-and-deploy.md](./infra-and-deploy.md).

## Backend layering

Strict `handler → service → repository` chain; `model` is the shared struct package; `adapter` wraps external providers behind service-owned interfaces (`PaymentClient`, `LogisticsClient`, `EmailProvider`, `PDFGenerator`).

| Layer | Package | Owns |
|---|---|---|
| Handler | `backend/internal/handler` | HTTP request/response mapping, route-level middleware (`middleware.go`), error mapping (`errors.go`) |
| Service | `backend/internal/service` | Business rules, orchestration, RBAC capability table (`rbac.go`), ports (`ports_payment.go`, `ports_logistics.go`, `ports_notify.go`, `ports_storage.go`) |
| Repository | `backend/internal/repository` | SQL persistence via `pgxpool`, cursor-based pagination (`cursor.go`) |
| Model | `backend/internal/model` | Shared domain structs (`Exam`, `Order`, `Product`, `School`, `User`, `PromoCode`, …) |
| Adapter | `backend/internal/adapter` | Biteship, Midtrans, SMTP, Fazpass (OTP fallback) |

All three layers are flat packages (no per-domain subpackages) — a file per feature (e.g. `service/exam_grading.go`, `repository/exam.go`, `handler/exam.go`).

## Module areas

| Module | Handler | Service | Repository | Sibling page |
|---|---|---|---|---|
| Auth & accounts | `auth.go` | `auth.go`, `admin_users.go`, `rbac.go` | `admin_users.go`, `user.go` | [rbac-and-accounts.md](../domain/rbac-and-accounts.md) |
| Products / courses | `product.go`, `course.go` | `product_specs.go`, `course.go` | `product.go`, `course.go` | — |
| Orders & payments | `order.go`, `webhook.go` | `store.go` (cart/checkout), `item_qty.go` | `order.go`, `order_reporting.go` | [orders-and-payments.md](../domain/orders-and-payments.md) |
| Shipping | `order.go` (ship/label/tracking routes) | `shipping_order.go`, `shipping_rates.go`, `shipping_webhook.go`, `shipping_admin_actions.go` | `order_shipment.go` | [shipping-biteship.md](./shipping-biteship.md) |
| Exam engine | `exam.go`, `exam_session*.go`, `exam_grant.go` | `exam.go`, `exam_session.go`, `exam_grading.go`, `exam_leaderboard.go`, `exam_expiry.go` | `exam.go`, `exam_expiry.go` | [exam-lifecycle-and-scoring.md](../domain/exam-lifecycle-and-scoring.md), [exam-capacity.md](./exam-capacity.md) |
| Certificates / cards | `exam_certificate_handler_test.go`, `exam_card_handler_test.go` | `certificate.go`, `certificate_layout.go`, `card_generate.go` | `exam_certificate_test.go`, `exam_card_test.go` | [certificate-rendering.md](./certificate-rendering.md) |
| Schools | `admin_school.go`, `region.go` | `school.go`, `school_bulk.go`, `region.go` | `school.go`, `region.go` | [rbac-and-accounts.md](../domain/rbac-and-accounts.md) |
| Announcements / notifications | `admin_announcements.go` | `announcement.go`, `notification.go` | `announcement.go` | — |
| System config / audit | `admin_system.go` | `system_config.go`, `audit_read.go` | `system_config.go`, `audit_log.go` | — |
| Bulk jobs (import/export) | `admin_jobs.go`, `*_bulk*.go` | `bulk_credentials.go`, `bulk_csv.go`, `student_bulk.go`, `school_bulk.go`, `exam_grant_bulk.go` | — | worker jobs, below |

## API surface

Everything is served under `/api/v1` (`backend/internal/server/routes.go`). Route groups by auth requirement:

| Group | Auth | Examples |
|---|---|---|
| Public | none | `POST /auth/register\|login\|google\|otp/*`, `GET /products`, `GET /schools`, `GET /provinces`, `GET /files/*`, `POST /webhooks/payment\|shipping`, `GET /config/payment-client-key` |
| Authenticated (any role) | JWT bearer | `GET /auth/me`, `/orders/*`, `/uploads/presign`, `/schools/:id`, `/promo-codes/active`, `GET /admin/jobs/:id` (any authenticated user polls their own job) |
| Student-only | JWT + role check (`StudentOnlyMiddleware`, role == `student`) | `/students/*` (dashboard, profile) |
| Exam (student-facing) | JWT bearer | `/exam/registrations`, `/exam/sessions/*` (start, save answers, submit, advance section, violations, result, leaderboard) |
| Admin | JWT + capability (`RBACMiddleware("<capability>")`) | `/admin/products`, `/admin/courses`, `/admin/orders`, `/admin/sessions` (proctoring), `/admin/results`, `/admin/tests`, `/admin/questions`, `/admin/exams`, `/admin/exam-grants`, `/admin/bulk-exam-orders`, `/admin/system`, `/admin/dashboard*` |

Route groups are frequently split into read/write **sibling groups on the same path prefix** rather than nested, so a broader read capability isn't implied by a narrower write one (e.g. `adminSessions` vs `adminSessionsRead` in `routes.go:159-171`, `adminExams` vs `adminExamsRead` vs `adminResultsWorkspace` in `routes.go:285-318`).

### Auth mechanism

JWT access + refresh tokens (`backend/internal/infra/jwt.go`). Every access token's `jti` is mirrored in Redis (`session:access:<jti>`); `JWTMiddleware` (`backend/internal/handler/middleware.go:17`) parses the token *and* calls `svc.SessionActive`, so logout / force-logout-all-sessions revokes tokens server-side before expiry, not just client-side. Refresh tokens are similarly tracked (`session:refresh:<token>`), single-use (rotated + deleted on `/auth/refresh`).

RBAC is capability-string based, not per-route role checks: `backend/internal/service/rbac.go` maps five roles (`student`, `admin_school`, `admin_exam`, `admin_store`, `super_admin`) to capability lists (`"*"`, `"orders:*"`, `"products(exam):read"`, …), and `RBACMiddleware(required)` calls `service.HasCapability(role, required)` with prefix/wildcard matching. Detail: [rbac-and-accounts.md](../domain/rbac-and-accounts.md).

## Background worker

One process (`backend/cmd/worker/main.go`), one `Worker` (`backend/internal/worker/outbox.go:46`) running six independent loops from `Run(ctx)`:

| Loop | Trigger | Does |
|---|---|---|
| Outbox poll | `WorkerPollInterval` ticker | `pollOutbox` → `handleOrderPaid`: fans a paid order out into `CourseSession` / `ExamRegistration` rows per item, and `handleCertificateNeeded` → renders via Gotenberg |
| Stale-payment sweeper | 5 min ticker | `sweepStalePayments`: expires orders stuck in `payment_pending` |
| Announcement dispatch | 5 min ticker | `pollAnnouncements`: sends due scheduled announcements |
| Generic job queue | `WorkerPollInterval` ticker | `pollJobs` claims one row from a shared `jobs` table and dispatches by `job.Type`: `student_bulk`, `school_bulk`, `exam_grant_bulk`, `exam_revoke_bulk` |
| Active exam session refresh | goroutine, own loop | `pollActiveExamSessions` → `refreshActiveExamSessions` |
| Exam expiry | goroutine, own loop | `pollExamExpiryLoop` → `pollExamExpiry`: force-submits sessions past their time limit |

Bulk jobs write progress/result to the same `jobs` table the `GET /admin/jobs/:id` route polls, and read/write the private object-storage bucket for input CSVs and result files (`objectStore` interface, `backend/internal/worker/job.go:24`).

## Observability

Both `api` and `worker` expose Prometheus metrics on `:9102` (`METRICS_ADDR`), bound to the internal compose network only — nginx never proxies it. Request logging is structured JSON (`slog`) with route template, path params, latency, and request ID per request (`backend/internal/server/server.go:33-60`). Full metrics/dashboard/alert detail: [infra-and-deploy.md](./infra-and-deploy.md).

## See also

- [infra-and-deploy.md](./infra-and-deploy.md) — deploy topology, replicas, resource limits
- [exam-capacity.md](./exam-capacity.md) — exam-day load characteristics
- [certificate-rendering.md](./certificate-rendering.md) — PDF/certificate render pipeline
- [shipping-biteship.md](./shipping-biteship.md) — logistics integration
- [exam-lifecycle-and-scoring.md](../domain/exam-lifecycle-and-scoring.md) — exam/session/scoring domain model
- [rbac-and-accounts.md](../domain/rbac-and-accounts.md) — roles, capabilities, account model
- [orders-and-payments.md](../domain/orders-and-payments.md) — order state machine, payment flow
