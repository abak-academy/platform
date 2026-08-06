# admin_exam authors digital products + lesson video stops leaking to YouTube

| | |
|---|---|
| **Raised** | 2026-08-06 |
| **Status** | ▶ design approved, implementation plan next |
| **Objective** | `admin_exam` can build and price a course end-to-end without `admin_store`, and a student watching a lesson has no route out to youtube.com. |
| **Surfaces** | `/admin/products`, `/admin/courses`, `/courses/[id]` |
| **Defects fixed** | — (both items are new capability, not regressions) |
| **Depends on** | — |
| **Verified against** | `main` @ `1b738b0`, 2026-08-06 |

Two unrelated asks, shipped on one branch at the user's request. They share no
files: item 1 is backend RBAC plus admin nav, item 2 is one frontend component.

---

## 1. Why

**Item 1.** `admin_exam` owns exam content — tests, questions, packages, session
monitoring — and can create products of type `exam`. It cannot create a product
of type `course`, and it cannot author a course at all. Selling a course today
requires `admin_store` to author the content and mint the product, even when the
course is exam-prep material the exam admin wrote. The client wants the exam
admin to own the whole digital catalogue: both digital product types (`exam`,
`course`) and the course content behind them.

**Item 2.** The student lesson player embeds YouTube in an iframe. It does not
link out — but YouTube's own player chrome does. Hovering the embed raises a
title bar carrying the video title and the YouTube logo, both of which navigate
to youtube.com in a new tab. A student one hover away from the open web is the
opposite of what a paid course should be.

### What the code actually enforces

Worth stating plainly, because the obvious place to look is wrong. Product and
course authorization does **not** run through `RBACMiddleware`. The
`/admin/products` and `/admin/courses` route groups carry only `JWTMiddleware`
(`routes.go:36`, `routes.go:45`). The real gates are:

- **Products** — `checkTypeRBAC(role, productType)` in `store.go:1793`, called
  from every create/update/publish/delete path.
- **Courses** — the literal expression `role != RoleAdminStore && role !=
  RoleSuperAdmin`, copy-pasted into **ten** service methods in `course.go`
  (create/update/delete course, create/update/delete/reorder section,
  create/update/delete/reorder lesson).

The capability strings in `rbac.go` are consumed by exactly two things: the
`RBACMiddleware` calls on *other* route groups, and the frontend mirror in
`use-capability.ts:13`, which decides what renders. For products they are
descriptive labels, not enforcement. Note also that
`products(book|course):write` is **not** a pattern — `HasCapability` does exact
and `:*`-prefix matching only, so the pipe is read as a literal character. It
has never matched anything. Both layers therefore need editing, and neither one
alone is sufficient.

---

## 2. Scope

**In:** `admin_exam` gains product types `exam` + `course` and full course
authoring (courses, sections, lessons); the `rbac.go` capability list and its
frontend mirror updated to match; Products and Courses added to the exam admin
nav; the products list and create modal restricted to the types the signed-in
role may write; `VideoPlayer` rewritten as a controlled player with a click
shield and our own control bar.

**Out:** `admin_exam` gets **no** `orders:*` and **no** `revenue:read` — it can
build and price a course, not fulfil or report on one. Self-hosted video. Any
change to `toYoutubeId`, the lesson-completion flow, or the admin course editor.
Auto-marking a lesson complete on video end.

---

## 3. Decisions

| # | Decision | Rationale |
|---|---|---|
| D1 | `admin_exam` gets digital products **and** full course authoring, not products alone | Chosen by the user from three options. Product-only would leave them able to link a course they cannot create — a dead end. |
| D2 | Extract `canAuthorCourses(role)` instead of editing ten inline checks | All ten lines change anyway; ten copies of a policy is how the eleventh method ships without one. |
| D3 | `admin_exam` does **not** get physical product types | The third option offered. Rejected: `book`/`merchandise`/`medal` carry stock, weight, and shipping, which is fulfilment work and belongs to `admin_store`. |
| D4 | Product type lists derive from role, in both the filter tabs and the create dropdown | Otherwise `admin_exam` sees rows it cannot edit and a dropdown that 403s on submit. |
| D5 | Click shield over the YouTube iframe, not self-hosted video | Chosen by the user. No infra change, no re-upload of existing lessons. Accepted trade in §6. |
| D6 | Fullscreen targets the wrapper `<div>`, not the iframe | Fullscreening the iframe would hand the viewport to YouTube's chrome and undo the shield. |
| D7 | Fall back to the current plain embed if the IFrame API script fails to load | A player with YouTube branding beats a dead black rectangle. |

---

## 4. Backend

### 4.1 `checkTypeRBAC` — admit `course` for the exam admin

`store.go:1793`. The `RoleAdminExam` arm currently returns `nil` only for
`exam`:

```go
case RoleAdminExam:
    if productType == "exam" || productType == "course" {
        return nil
    }
    return ErrForbidden
```

`RoleAdminStore` and `RoleSuperAdmin` arms are untouched. This one function
covers create, update, publish, and delete — every product write path calls it.

### 4.2 `course.go` — one policy helper, ten call sites

Replace every `if role != RoleAdminStore && role != RoleSuperAdmin` with a call
to a single unexported helper in `course.go`:

```go
func canAuthorCourses(role string) bool {
    return role == RoleAdminExam || role == RoleAdminStore || role == RoleSuperAdmin
}
```

Applied at: `CreateCourse` (:18), `DeleteCourse` (:59), `UpdateCourse` (:77),
`CreateSection` (:129), `UpdateSection` (:153), `DeleteSection` (:166),
`ReorderSections` (:179), `CreateLesson` (:203), `UpdateLesson` (:229),
`DeleteLesson` (:247), `ReorderLessons` (:260).

Read paths (`ListCourses`, `GetCourse`, `ListSections`) take no role today and
stay as they are.

### 4.3 `rbac.go` — capability list

`admin_exam` becomes:

```go
RoleAdminExam: {"questions:*", "tests:*", "products(exam):*", "products(course):*", "sections:*", "sessions:*", "uploads:write"},
```

Two separate entries, not `products(exam|course):*`. `HasCapability` cannot read
a pipe, and the existing `products(exam):*` entry has to survive verbatim: the
`/admin/exams` read group is gated by
`RBACMiddleware("products(exam):read")` (`routes.go:293`), which resolves only
through the `:*` prefix match on `products(exam):`. Collapsing the two into one
piped entry would silently lock the exam admin out of its own read routes.

`uploads:write` is already granted, so product images and course thumbnails work
with no further change. `admin_store`'s dead
`products(book|course):write` entry is left alone — fixing it is a separate
concern and touching it would change what `admin_store` renders.

### 4.4 Routes

No change. `/admin/products` and `/admin/courses` carry no `RBACMiddleware`
today and gain none — adding one now would be a second, redundant policy source
that can drift from `checkTypeRBAC`. The service layer stays the single gate.

---

## 5. Frontend

### 5.1 `use-capability.ts` mirror

`ROLE_CAPABILITIES` at `use-capability.ts:13` is a hand-copied mirror of
`rbac.go`. Update the `admin_exam` row to match §4.3 exactly, character for
character. A test asserts the two stay in sync (§7).

### 5.2 `nav-config.ts` — exam admin nav

`ADMIN_EXAM_NAV` (`nav-config.ts:109`) today has one group holding
`EXAM_NAV_ITEMS`. Add a second group for the catalogue:

```ts
export const ADMIN_EXAM_NAV: RoleNavConfig = [
  { titleKey: "nav_group_exam", items: EXAM_NAV_ITEMS },
  {
    titleKey: "nav_group_catalog",
    items: [
      { labelKey: "admin_nav_products", href: "/admin/products", icon: Package },
      { labelKey: "admin_nav_courses", href: "/admin/courses", icon: Library },
    ],
  },
];
```

`admin_nav_products` and `admin_nav_courses` already exist in `i18n.ts` (:17,
:18 id / :1403, :1404 en). `nav_group_catalog` is new and needs an entry in
**both** locale blocks. Neither `/admin/products` nor
`/admin/courses` has a per-page role guard — `app/(admin)/layout.tsx` only picks
a nav config — so the nav entry is the whole routing change.

### 5.3 Product type lists follow the role

Two hardcoded arrays need to become role-derived:

- `FILTER_TYPES` in `app/(admin)/admin/products/page.tsx`, driving the filter
  tab row at :163.
- `PRODUCT_TYPES` in `components/admin/ProductModal.tsx:33`, driving the create
  dropdown.

Both read from one exported helper — `writableProductTypes(role)` — returning
`["exam", "course"]` for `admin_exam` and the full five for `admin_store` and
`super_admin`. The list itself is also filtered to those types for `admin_exam`,
so the table shows only rows the signed-in admin can act on.

`ProductModal`'s edit path already pins the type (`effectiveType = isEdit ?
product?.type : type`, :144) and does not offer the dropdown, so an
`admin_exam` opening a `book` it should not have seen cannot change its type —
and with the list filtered, cannot reach it at all.

### 5.4 `VideoPlayer` — controlled player

`components/courses/VideoPlayer.tsx`. `toYoutubeId` and the "Video belum
tersedia" empty state are unchanged. What changes is everything after the id
resolves.

**Structure.**

```
<div ref={wrapper}>                  ← fullscreen target, 16/9
  <iframe … enablejsapi=1 />         ← youtube-nocookie.com/embed/{id}
  <div className="absolute inset-0"/> ← click shield, swallows all pointer events
  <ControlBar … />                    ← ours, above the shield
</div>
```

The shield covers the iframe completely. Because pointer events never reach the
iframe, YouTube's title bar and logo **never render** — the shield suppresses
the chrome visually, not merely the click on it. That is the mechanism; it is
worth knowing, because it also means the shield cannot have gaps.

**Player control.** Load the IFrame API (`https://www.youtube.com/iframe_api`)
once per page, resolve the `onYouTubeIframeAPIReady` global into a promise, and
construct a `YT.Player` bound to the iframe. Our control bar drives it:

| Control | API |
|---|---|
| Play / pause | `playVideo()` / `pauseVideo()`, state from `onStateChange` |
| Scrubber | `seekTo(s, true)`, position polled at ~4 Hz while playing |
| Buffered range | `getVideoLoadedFraction()` |
| Elapsed / total | `getCurrentTime()` / `getDuration()` |
| Volume + mute | `setVolume()` / `mute()` / `unMute()` |
| Fullscreen | `wrapper.requestFullscreen()` — **wrapper, not iframe** (D6) |

Embed params: `enablejsapi=1`, `rel=0`, `disablekb=1`, `controls=0`,
`playsinline=1`, `origin=<window.location.origin>`. `controls=0` hides YouTube's
own bar so ours is the only one; the shield stays regardless, since `controls=0`
does not remove the title-bar link.

**Fallback (D7).** If the API script fails to load or `YT.Player` construction
throws, render the current markup — plain embed, no shield, no control bar —
and log once. A student who cannot reach the API still gets their lesson.

**Keyboard.** The control bar is real `<button>`s and an `<input type="range">`,
so it is tabbable and screen-reader reachable without extra work. `disablekb=1`
stops YouTube from also consuming arrow keys.

---

## 6. Accepted risk

Covering the player conflicts with YouTube's embedded-player terms, which ask
that the player not be obstructed. This was raised before the approach was
chosen and accepted knowingly. Practically it is not enforced at this scale, and
the exit is clean if it ever matters: self-host the MP4s to GCS and swap the
iframe for a `<video>` tag. Nothing else in the design would need to move — the
control bar is already ours, and `toYoutubeId` is the only YouTube-shaped thing
left, called from one place.

---

## 7. Testing

**Backend.**

- `checkTypeRBAC` — table test over the full role × type matrix (5 roles × 5
  types), asserting `admin_exam` passes `exam` and `course` and is forbidden
  `book`, `merchandise`, `medal`. `store_test.go` already has this shape.
- `canAuthorCourses` — table test over all five roles, plus one service-level
  test per *category* (course, section, lesson) confirming `admin_exam` now
  succeeds and `admin_school` still gets `ErrForbidden`. `course_test.go` has
  existing per-method forbidden tests to extend.
- `HasCapability(RoleAdminExam, "products(exam):read")` still true after the
  `rbac.go` edit — this is the one that the `/admin/exams` read group depends
  on, and the one a careless single-entry rewrite would break.

**Frontend.**

- A test asserting `ROLE_CAPABILITIES` in `use-capability.ts` matches `rbac.go`.
  Since the mirror is hand-copied and cannot import Go, the check is a literal
  expected-value assertion in the TS test *and* a matching one in Go, both
  spelling out the same capability list — so either side drifting fails a build.
  This copies the `physical_type_test.go` ↔ `web/lib/shipping.test.ts` pair,
  which pins `isPhysicalType` in both languages the same way. Note it is two
  independent suites asserting the same literal, **not** a vitest process
  shelling out to `go test` — that bridge shape is a known CI trap here.
- `writableProductTypes` — per role.
- Products page renders only digital filter tabs and digital rows as
  `admin_exam`; all five as `admin_store`.
- `VideoPlayer` — asserts the shield element renders after the iframe in DOM
  order, that no `<a>` with a `youtube.com` href exists anywhere in the tree,
  and that the fallback branch renders when the API script rejects.

**What jsdom cannot prove.** jsdom has no layout, so no unit test can show the
shield actually covers the iframe or that the title bar is suppressed. That
check is a Playwright run against a lesson with a real video: hover the player,
screenshot, confirm no YouTube title bar and no logo; then click where the title
bar would be and confirm no navigation and no new tab. Also exercise fullscreen
and confirm our control bar survives it. Until that screenshot exists, item 2 is
unverified regardless of what the unit tests say.

**Gates.** `deploy/pipeline/backend.sh` in full — `go vet ./internal/...` alone
misses `integration/`. `npm run build` in `web/`, not just `tsc --noEmit`.

---

## 8. Rollout

Single branch, single PR, no migration, no config change, no feature flag. The
RBAC widening is additive: no role loses anything, so there is no window where
an admin is locked out mid-deploy. `VideoPlayer` is one component with one
caller (`app/(student)/courses/[id]/page.tsx:153`).

Rollback is `git revert`. If only the player misbehaves in production, deleting
the shield and control bar restores today's behaviour without touching the RBAC
half.

PR title: `feat: admin_exam digital products + course authoring; lesson player
click-shield`.

---

## 9. Open

- `nav_group_catalog` needs an Indonesian label. "Katalog" unless the client
  prefers otherwise.
- `admin_store`'s `products(book|course):write` capability string has never
  matched anything (`HasCapability` cannot read the pipe). It is inert rather
  than harmful — `checkTypeRBAC` is what actually gates `admin_store` — so it is
  left alone here. Worth its own cleanup ticket.
