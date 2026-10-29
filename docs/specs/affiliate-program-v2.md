---
title: Affiliate program v2
issue: none yet
status: accepted
created: 2026-10-27
sources: ["affiliate_program_v2.md", "backend/db/migrations/0002_identity.up.sql", "backend/db/migrations/0004_commerce.up.sql", "backend/internal/service/rbac.go", "backend/internal/service/store.go", "backend/internal/service/bulk_exam_order.go", "backend/internal/service/auth.go", "backend/internal/handler/school_scope.go", "backend/internal/handler/admin_promo.go", "backend/internal/handler/admin_students.go", "backend/internal/handler/bulk_exam_order.go", "backend/internal/repository/promo.go", "backend/internal/server/routes.go", "web/components/admin/ProductsToolbar.tsx", "web/components/admin/PromoModal.tsx", "docs/specs/admin-orders-toolbar-redesign.md", "docs/specs/exam-admin-student-management.md"]
---

# Affiliate program v2

## Problem and outcome

Super admin wants affiliators: accounts that register students of assigned schools, buy exam and course products for them with affiliator-only promo codes, and bill the students outside the system. Outcome: a new `affiliator` role with its own dashboard, a super-admin settings and monitoring area, and an extended promo code. No new tables.

The v1 concept (`feat/affiliate-program`, commission, referral links, payouts, about 13 tables) is not reused; v2 is cut from `main`.

## Verified facts (read from `main` 4332c418, not run)

- `promo_code` (`0004_commerce`, `0009`, `0054`): `discount_percent`, `discount_amount`, `min_order_amount`, `max_discount_amount`, `max_uses`, `used_count`, `expires_at`, `is_public`. It has no start date, no min quantity, no product scope and no owner. Usage is only the `used_count` counter; `orders.promo_code_id` links an order to its promo.
- `ValidatePromo` (`service/store.go`) takes the code and the subtotal only. `POST /promo-codes/validate` is anonymous; `GET /promo-codes/active` lists `is_public` promos. A non-public code still validates for anyone who knows it.
- The promo is applied in `PatchCart` and counted in `Checkout` (`IncrementPromoUsesTx`). `CreateBulkExamOrder` (`service/bulk_exam_order.go`) has no promo path, covers exam products only, and the buyer on the order is the admin. There is no single-student purchase by an admin.
- `users.role` has a CHECK of five roles; `users.school_id` is one nullable UUID, indexed (`idx_users_school_id`) and copied into the JWT (`infra.Claims.SchoolID`). `users.status` is `active|deactivated|deleted` and `deactivated` rejects login. `school.status` is `active|deactivated`.
- School scope is resolved in `handler/school_scope.go` (`resolveSchoolScope`, `resolveStudentSchoolScope`, `resolveResultsSchoolScope`). `admin_exam` already administers students of every school through the capability `student-scope:all` (`CanManageAllStudents`).
- Product types are `book`, `course`, `exam`, `merchandise`, `medal` (`product_type_check`, `0027`). `book`, `merchandise` and `medal` are physical (`isPhysicalType`, `store.go`). `adminExamProductTypes = [course, exam]` is the existing precedent for a restricted admin product set.
- `order_participant` (`0031`, extended by `0068` to `course` and `exam` lines) records the students an order grants access to.
- Latest migration on `main` is `0069`; migrations run at API start by filename order (`infra.RunMigrations`).

## Agreed behavior

Decisions (owner, 2026-10-27):

- **Role.** New role `affiliator`. A school admin is "promoted" by changing the role and assigning schools. Super admin can also create affiliator accounts, single and bulk. Login is the same flow as `admin_school`.
- **Schools.** An affiliator has a list of allowed schools stored on the user (`users.school_ids`). It sees and registers only students of those schools, never others, and never students without a school.
- **Settings.** The affiliator settings and monitoring pages are `super_admin` only.
- **Active/inactive.** Super admin can deactivate an affiliator. A deactivated affiliator can still log in but is read-only: every write endpoint is rejected and the UI shows a notice that the account is deactivated.
- **Students.** An affiliator registers students (single and bulk) into an allowed school and sets `registered_by` to itself. It lists students of its allowed schools and can tell which ones it registered.
- **Purchases.** An affiliator buys `exam` and `course` products for students of its allowed schools, single and bulk, and pays the whole order through Midtrans Snap as the buyer. Billing the students is outside the system. Physical product types are out of scope for this phase.
- **Promo.** The affiliator enters a code at checkout; it is validated like any promo plus the rules below. The super admin controls promos from the existing admin promo page.
- **Promo rules** (all optional unless stated):
  1. `affiliator_only`: valid only for the `affiliator` role, never listed in `/promo-codes/active`, rejected for every other role even when the code is known.
  2. `min_qty`: minimum total units in the order.
  3. `min_order_amount` (exists): minimum subtotal.
  4. `starts_at` and `expires_at`: the promo applies only inside the range.
  5. `product_ids`: empty means all products; otherwise only lines of those products count towards `min_qty`, `min_order_amount` and the discount.
  6. Percent or nominal discount (exists, with `max_discount_amount` cap).
- **Dashboard** (affiliator): students per allowed school; purchases and total promo discount; available affiliator promos with their conditions; students with no purchase yet; payment status of its orders; monthly purchase trend; best-selling products per school.
- **Monitoring** (super admin): list and detail of affiliators with their schools, student counts, orders and active state.

## Technical contract

### Migration `0070`

- `users.role` CHECK gains `affiliator`.
- `users.school_ids UUID[] NOT NULL DEFAULT '{}'` and `users.registered_by UUID REFERENCES users (id)`.
- `promo_code`: `starts_at TIMESTAMPTZ`, `min_qty INT`, `affiliator_only BOOLEAN NOT NULL DEFAULT false`, `product_ids UUID[] NOT NULL DEFAULT '{}'`.
- No new tables. `school_ids` has no foreign key; the service validates every ID against `school` when super admin assigns it. A GIN index on `users.school_ids` is added only if the monitoring query needs the reverse lookup.

### Authorization and scope

- `service/rbac.go`: role `affiliator` with capabilities for scoped students, the affiliator purchase routes, promo read and its own dashboard. A new capability (`student-scope:affiliator`) is the single marker for "scope comes from `school_ids`"; it must not start with `students:`. Role comparisons stay in `roleCapabilities`, not in handlers.
- `handler/school_scope.go`: a new resolver returns the affiliator's `school_ids`; a requested `school_id` outside the list is 403. The JWT school claim is not used for affiliators. Student list, register, bulk import and credentials handlers accept the list scope.
- Read-only middleware (`AffiliatorWriteGuard` on the `/admin` group): for a user with `status = deactivated` and role `affiliator`, login succeeds and every non-GET affiliator route returns 403 with a stable error code the UI maps to the notice. All other roles keep today's behavior (`deactivated` rejects login).
- Super admin routes for affiliator accounts: promote, create single, create bulk (CSV), set schools, set active state, list, detail. Each is recorded in `audit_log` like `student.set_password`.

### Promo

- `ValidatePromo` receives the actor role, the order lines (product id and qty) and the subtotal. It applies the rules above and returns the discount computed over matching lines only.
- `GET /promo-codes/active` excludes `affiliator_only`; a new affiliator-only list returns active, in-range, unexhausted affiliator promos.
- Admin create and update accept the new fields. `PUT /admin/promo-codes/:id` now replaces every editable field (the code and `used_count` never change), so an omitted key clears it. Create validates percent versus nominal exclusivity, `starts_at < expires_at`, `min_qty >= 0`, and that every `product_ids` entry exists.
- The affiliator order has no cart, so the promo is validated at order creation and again at checkout; the counter increments in the existing pre-gateway transaction.

### Purchase

- Separate affiliator flow keyed by product, not by exam: `GET /affiliates/me/products`, `POST /affiliates/me/orders/preview`, `POST /affiliates/me/orders`, `POST /affiliates/me/orders/:id/checkout` (`Idempotency-Key` required). The existing `/admin/bulk-exam-orders` routes are unchanged and still exam-only for school admins.
- Body: `school_id` (one of the allowed schools), `product_id`, one selector (`student_ids`, `grade` or `all`) and an optional `promo_code`. A single student is a one-element `student_ids`. Only `exam` and `course` products visible to students are accepted (`GetProduct` with the student role); any other type is 422 `affiliator_product_type`.
- Students who already hold every target of the product (live exam registration, or active course enrollment) are excluded as `already_owned`; zero left is 409. The preview and the create both re-resolve; a cached preview is never trusted.
- Promo: validated with role `affiliator` over a line of the net-new quantity, stored on the order (`promo_code_id`, `discount`, `total`), and re-validated against the live promo (rules and unchanged discount) right before checkout while the order is still a cart. A replay of a finished checkout skips that check. The counter increments in the existing pre-gateway transaction.
- The buyer on the order is the affiliator. One open cart per buyer exists (`idx_orders_student_cart`), so creating a new order cancels the affiliator's previous unfinished participant order (`cancellation_reason = 'superseded'`); an unfinished order without participants is a 409 `open_cart_exists`.
- `Checkout` no longer applies the digital qty-1 rule to orders that have participant rows (their qty is the seat count). This also unblocks multi-student checkout of the existing school-admin bulk exam order.
- Fulfilment: the `OrderPaid` handler enrolls each participant in each linked course, mirroring the existing exam fan-out. The stock decrement at checkout counts the seat quantity.
- A deactivated affiliator is read-only on these routes and on `/orders/*` (same `AffiliatorWriteGuard`).

### API (shape; final names settle in phase 1 and 4)

- `/api/admin/system/affiliators` (super admin, `system:admin`): `GET` list (`status`, `q` on name/email, `cursor`, `limit`; returns `total`), `GET /promotable` (active `admin_school` accounts with their school; `q` also matches the school name; same paging), `POST` create (an empty `password` is generated and returned once as `temp_password`), `POST /bulk/presign` and `POST /bulk {file_key}` (async `affiliator_bulk` job, see below), `POST /:id/promote` (admin_school only, one-way), `PUT /:id/schools`. Active state reuses `PATCH /admin/system/accounts/:id/status`.
- Dashboard (phase 5): `GET /api/affiliates/me/dashboard?from&to` (WIB day range on `paid_at`) returns allowed schools with student counts, totals (paid orders, units, amount, promo discount, pending orders), the monthly trend, best sellers per school and the number of active students no live order covers. `GET /api/affiliates/me/orders?status&cursor` lists non-cart orders with product, seats, school and promo. `GET /admin/students?not_purchased=true` lists the students behind that last number. Paid means `paid_at` set and not refunded.
- Monitoring (phase 5): the affiliator list rows carry `student_count`, `order_count` and `paid_amount`; `GET /api/admin/system/affiliators/:id` returns profile, schools, lifetime totals and the ten latest orders.
- Bulk accounts (review R2, reshaped in the second review): CSV columns `name,email,password,school_npsn,school_code`. **One line is one school**, by NPSN or by code (the NPSN wins when both are filled); an affiliator with several schools is several lines with the same email, and no cell is a list. Lines are grouped by email: name and password come from the first line (later lines may leave them blank or repeat them, a different value fails the account), the schools are collected in line order without repeats. An account is all or nothing: if any of its lines is wrong (unknown or inactive school, bad NPSN, conflicting name) none of its lines is created and each reports why, so a fixed file can be uploaded again. A blank `password` is generated and written on the account's first line only. The worker creates each account through the single-create path, uploads `affiliator-bulk/results/<job>.csv` (`row,name,email,school,status,temp_password,error`, the only place generated passwords appear) and deletes the input CSV because it may hold passwords. Up to 1000 lines. The earlier synchronous JSON endpoint was removed.
- Reports (review R3), all `system:admin`, period `from`/`to` (WIB days; `from` and `to` must be ordered): `GET /api/admin/system/affiliate-reports/{affiliators,schools,products,promos}` paged by `limit` (max 100) and `offset` with `total`, `GET .../trend` (latest 24 months, unpaged), and `GET .../:report/export` (CSV with BOM, whole period, up to 10,000 rows, formula-safe cells). `affiliators`: sales in the period (paid, not refunded), the current pending backlog, expired and total orders created in the period, last order ever, filters `q` and `inactive=true` (no order created in the period). `schools` (schools assigned to some affiliator): students, students covered by a live order of any affiliator (all time), not covered, and the period's sales. `products`: units per school and product. `promos`: affiliator-only promos plus any promo used on an affiliator order, with lifetime `used_count`/`max_uses` and the period's orders and discount. `trend`: orders, units, amount, discount and distinct active affiliators per month.
- `GET /api/affiliates/me` (affiliator, `affiliates:self`): profile, schools, active state. Purchase routes are listed under Purchase. `GET /api/affiliates/me/promos` lists the affiliator-only promos usable now. Dashboard widgets and students follow under `/api/affiliates/me/*`.
- Student and purchase routes reuse `/admin/students*` and `/admin/bulk-exam-orders*` with the new scope and capability.
- Students (phase 3): the affiliator holds `students:*`. List (`GET /admin/students`) covers every allowed school, narrowable with `?school_id=` (must be allowed, else 403 `school_not_assigned`) and `?registered_by_me=true`. Single-school actions (register) require `?school_id=` of an allowed school. Bulk CSV uses the cross-school format (school per row, like `super_admin`); each row's school is checked against the allowed list when the row is registered, and a manual password column is refused. Status change, credential reissue, bulk credential reissue and set password are not available to affiliators. The allowed list is read from the database on every request, not from the token.

### Web

- Follows the admin concept in `docs/specs/admin-orders-toolbar-redesign.md`: the primary row has only search (with clear), a Filters button showing the active count, and one sort select; filters live in a `Popover` with Reset and a Show button; active filters are removable `md-chip md-chip-primary` chips outside the panel; rows per page and Prev/Next sit in the table footer.
- Reuse `AdminPageHeader`, `data-table`, `StatCard`, `PeriodBar`, `TopProductsTable`, `UnitSalesBlock`, `PromoModal`, `BulkImportModal`, `BulkFormatGuide`, `SchoolPicker`, `SchoolFilterPicker` and `ParticipantPicker`; no new visual language.
- Every string goes through `useTranslation` with real `id` and `en` entries in `web/lib/i18n.ts`.
- The affiliator is an admin-shell role (`ADMIN_ROLES`, own nav, home `/admin/affiliate`, same `/admin/login`), so its pages live under `(admin)/admin/affiliate/*` rather than a separate route group: `/admin/affiliate` (dashboard), `/students`, `/orders`, `/orders/new`, `/promos`. The super-admin menu is its own group, apart from System (review): `/admin/affiliators` with two tabs (Daftar affiliator: search, status filter, pagination, create one or many by CSV, row action only activate/deactivate; Promote: paged admin school list with the school prefilled, a warning that promotion cannot be undone and that the school admin access stops, plus the affiliators to change their schools) and `/admin/affiliators/reports` (period bar and five report tabs with pagination and CSV export; per-affiliator rows open the detail dialog). Labels (second review): money columns are **Total dibayar** (paid orders, after discount, refunds excluded) everywhere except the affiliator's own order list, where the column is **Total order** (each order's value after discount, whatever its status) with a note saying so; every report tab opens with a short text saying what it counts and over which time. Downgrading an affiliator to school admin is not allowed (decision, review): the Promote tab and dialog say so, and deactivation is the way to stop access. Students use the existing hooks with the affiliator filters; the order flow pays through `SnapCheckout` on the `/affiliates/me/orders` base path.
- Bulk student import for an affiliator reuses `BulkImportModal` with `schoolPerRow`: the template and guide carry `school_npsn`/`school_code` and no password column.
- The nav (`web/lib/nav-config.ts`) and `use-capability.ts` mirror the new capabilities.

## Acceptance

1. An affiliator cannot list, register, import or buy for a student outside `school_ids`; a `school_id` outside the list is 403.
2. A promo with `affiliator_only` is rejected for `student`, `admin_school` and anonymous callers, and absent from `/promo-codes/active`.
3. `min_qty`, `min_order_amount`, `starts_at`/`expires_at` and `product_ids` each reject an order that misses the rule and compute the discount only over matching lines.
4. A deactivated affiliator logs in, reads data, and gets 403 with the stable code on every write; other deactivated roles still cannot log in.
5. Buying for students creates an order with `order_participant` rows, the promo applied once, and Snap payment as the affiliator.
6. A physical product type is rejected in the affiliator purchase path.
7. Only `super_admin` reaches the affiliator settings and monitoring routes.
8. Migration `0070` applies on a database at `0069` and existing roles, students and promos are unchanged.
9. Dashboard figures match the underlying `orders` and `users` rows.
10. Backend unit tests (`docs/standards/go-style-and-tests.md`) and web tests (`docs/standards/web-tests.md`) pass per phase; the end-to-end run happens once after phase 5.

## Delivery

Phases, each committed separately and each with unit tests only:

0. This TRD and the branch.
1. Migration `0070`, role and capabilities, account management (create, bulk, promote, set schools), active/inactive read-only.
2. Promo columns, `ValidatePromo`, admin promo CRUD and `PromoModal`.
3. Affiliator students: scope resolver over `school_ids`, list, single and bulk registration, `registered_by`.
4. Purchase: single and bulk for `exam` and `course` with promo.
5. Affiliator dashboard and super-admin monitoring pages.
6. One end-to-end run (`web/e2e/affiliate-program.spec.ts`, real backend and worker: sign-in, registration, scope refusals, free purchase with fulfilment, dashboard, super-admin deactivation, read-only session), fixes, wiki updates (`docs/wiki/domain/rbac-and-accounts.md`, `docs/wiki/domain/orders-and-payments.md`).

## Open points

- The "access about to end" widget is dropped from the dashboard: course enrollments have no expiry and exam registrations follow the exam window, so there is no single end date to report.
- Abandoned checkouts still consume a promo use (existing behavior, accepted).
- The v1 worktree uses migration numbers `0070` to `0073`; they collide if v1 is ever merged.

## Out of scope

Commission, referral links, payouts, physical products, in-system billing of students, per-user promo redemption limits, and a redemption table.
