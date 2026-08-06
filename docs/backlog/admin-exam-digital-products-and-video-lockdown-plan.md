# admin_exam digital products + lesson video lockdown — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `admin_exam` can author courses and create digital products end-to-end, and a student watching a lesson has no route out to youtube.com.

**Architecture:** Four fully independent tasks — one Go package, three disjoint frontend file sets, zero shared files — plus a final integration task that runs the whole-project gates once. T1–T4 can be fanned out in parallel to separate agents. T5 must run last, alone.

**Tech Stack:** Go 1.26 (Echo, pgx), Next.js 15 App Router, React 19, TanStack Query, Tailwind, vitest + @testing-library/react, Playwright.

**Spec:** [`admin-exam-digital-products-and-video-lockdown.md`](./admin-exam-digital-products-and-video-lockdown.md)

**Branch:** `feat/admin-exam-digital-products-video-lockdown` (already created, spec committed at `acf598a`).

---

## Global Constraints

- **Go commands must be prefixed:** `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && <cmd>`. Without it you get `package unsafe is not in std`.
- **Never `git add -A`.** Stage the exact paths listed in each task. The working directory contains unrelated scratch files and sibling agents' work.
- **No Claude co-author trailer** in any commit message. Commits are authored by the user alone.
- **Do not run `npm run build` or `tsc --noEmit` inside T1–T4.** Both typecheck the entire project and will surface sibling agents' in-flight errors as if they were yours. T5 owns them.
- **Do not push.** T5 ends at a clean local branch; the user pushes.
- `admin_exam` gains **no** `orders:*` and **no** `revenue:read`. If a step seems to imply otherwise, it is wrong — stop and report.
- Product types are exactly `"book" | "course" | "exam" | "merchandise" | "medal"`. Digital = `course`, `exam`. Physical = `book`, `merchandise`, `medal`.
- Locale strings live in `web/lib/i18n.ts` under two blocks: `id` (Indonesian, first) and `en`. Every new key needs an entry in **both**.

### Parallelism contract

| Task | Owns these files exclusively | Package / area |
|---|---|---|
| T1 | `backend/internal/service/{store.go, course.go, rbac.go, store_test.go, course_test.go, rbac_test.go}` | Go `internal/service` |
| T2 | `web/lib/hooks/use-capability.ts`, `web/lib/nav-config.ts`, `web/lib/i18n.ts`, `web/lib/hooks/use-capability.test.ts` | TS |
| T3 | `web/lib/product-types.ts`, `web/lib/product-types.test.ts`, `web/app/(admin)/admin/products/{page.tsx, page.test.tsx}`, `web/components/admin/{ProductModal.tsx, ProductModal.test.tsx}` | TS |
| T4 | `web/components/courses/VideoPlayer.tsx`, `web/components/courses/VideoPlayer.test.tsx` | TS |
| T5 | none — runs gates, fixes only what the gates surface | all |

No file appears twice. T3 reads the `UserRole` **type** from `nav-config.ts` (T2's file) but does not edit it; type-only imports are safe to read mid-flight because T2 does not change `UserRole`.

**Why T1 is one task and not three.** `store.go`, `course.go`, and `rbac.go` are the same Go package. Two agents editing one package in one working directory will see each other's half-finished state through every `go build` and `go test` — a red run that isn't yours is indistinguishable from one that is. `course.go` also has a test-shim coupling (see T1) that has to move with it.

**Committing in parallel.** Each task commits its own exact paths. If `git commit` fails with `index.lock exists`, a sibling is mid-commit — wait two seconds and retry. Do not delete the lock file.

---

## File Structure

**Created**

| Path | Responsibility |
|---|---|
| `backend/internal/service/rbac_test.go` | Pins the capability list per role, and the `products(exam):read` resolution that `/admin/exams` depends on. |
| `web/lib/product-types.ts` | Single source for "which product types may this role write" on the frontend. Mirrors `checkTypeRBAC`. |
| `web/lib/product-types.test.ts` | Pins that map per role. |
| `web/lib/hooks/use-capability.test.ts` | Pins `ROLE_CAPABILITIES` against the literal list in `rbac.go`. |
| `web/components/courses/VideoPlayer.test.tsx` | Shield presence, no youtube.com anchor, fallback branch. |

**Modified**

| Path | Change |
|---|---|
| `backend/internal/service/store.go` | `checkTypeRBAC` admits `course` for `admin_exam`. |
| `backend/internal/service/course.go` | Eleven inline guards → one `canAuthorCourses` helper that admits `admin_exam`. |
| `backend/internal/service/rbac.go` | `admin_exam` capability list. |
| `backend/internal/service/store_test.go` | Extend `TestCreateProduct_TypeRBAC` to the full role × type matrix. |
| `backend/internal/service/course_test.go` | Shim calls the real helper; invert three `admin_exam`-is-forbidden assertions. |
| `web/lib/hooks/use-capability.ts` | Mirror the new `admin_exam` list. |
| `web/lib/nav-config.ts` | `ADMIN_EXAM_NAV` gains a Katalog group. |
| `web/lib/i18n.ts` | `nav_group_catalog` in both locales. |
| `web/app/(admin)/admin/products/page.tsx` | Filter tabs and rows scoped to writable types. |
| `web/components/admin/ProductModal.tsx` | Create dropdown scoped to writable types. |
| `web/components/courses/VideoPlayer.tsx` | Controlled player: shield + own control bar. |

---

# Task 1: Backend — `admin_exam` may write courses and course products

**Files:**
- Modify: `backend/internal/service/store.go:1793-1813`
- Modify: `backend/internal/service/course.go` (eleven guards, listed in Step 3)
- Modify: `backend/internal/service/rbac.go:13-22`
- Modify: `backend/internal/service/course_test.go` (shim at `:256`, `:541`; assertions at `:671`, `:701`, `:1201`)
- Modify: `backend/internal/service/store_test.go:392-425`
- Create: `backend/internal/service/rbac_test.go`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `canAuthorCourses(role string) bool` — unexported, `internal/service` only. No other task imports it. The capability list this task writes into `rbac.go` is duplicated by hand in T2; both are spelled out verbatim in their own task, so neither has to read the other.

### ⚠️ Read this before you start

`course_test.go` does **not** test `course.go`. It defines `shimCourseService` (`:256`) and `shimGetDeleteCourse` (`:541`), which re-implement every role guard inline as `if role != RoleAdminStore && role != RoleSuperAdmin`. If you edit `course.go` and run the suite, it will pass without ever executing your change. Step 3 changes the real file; Step 5 makes the shim delegate to the real helper so it can never drift again.

- [ ] **Step 1: Write the failing test for the product-type matrix**

Replace the body of `TestCreateProduct_TypeRBAC` in `backend/internal/service/store_test.go:392` with a full matrix. Keep the function name.

```go
func TestCreateProduct_TypeRBAC(t *testing.T) {
	ctx := context.Background()

	allTypes := []string{"book", "merchandise", "medal", "course", "exam"}

	// want[role] = set of types that role may create.
	want := map[string]map[string]bool{
		RoleSuperAdmin:  {"book": true, "merchandise": true, "medal": true, "course": true, "exam": true},
		RoleAdminStore:  {"book": true, "merchandise": true, "medal": true, "course": true, "exam": true},
		RoleAdminExam:   {"course": true, "exam": true},
		RoleAdminSchool: {},
		RoleStudent:     {},
	}

	for role, allowed := range want {
		for _, pt := range allTypes {
			t.Run(role+"/"+pt, func(t *testing.T) {
				fake := newFakeStoreRepo()
				svc := newShim(fake)
				_, err := svc.CreateProduct(ctx, model.Product{Type: pt, Name: "P"}, role)
				if allowed[pt] {
					if err != nil {
						t.Errorf("%s creating %s: want nil, got %v", role, pt, err)
					}
					return
				}
				if !errors.Is(err, ErrForbidden) {
					t.Errorf("%s creating %s: want ErrForbidden, got %v", role, pt, err)
				}
			})
		}
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestCreateProduct_TypeRBAC -v
```

Expected: FAIL on `admin_exam/course` — `want nil, got forbidden`. Every other subtest passes. If `admin_exam/course` passes already, stop: someone else has edited `store.go`.

- [ ] **Step 3: Admit `course` in `checkTypeRBAC`**

`backend/internal/service/store.go:1805`. Change only the `RoleAdminExam` arm:

```go
	case RoleAdminExam:
		// The exam admin owns the whole digital catalogue — exam products and
		// the courses behind them. Physical types stay with admin_store because
		// they carry stock, weight and shipping, which is fulfilment work.
		if productType == "exam" || productType == "course" {
			return nil
		}
		return ErrForbidden
```

- [ ] **Step 4: Run it and watch it pass**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestCreateProduct_TypeRBAC -v
```

Expected: PASS, 25 subtests.

- [ ] **Step 5: Write the failing test for `canAuthorCourses`**

Append to `backend/internal/service/course_test.go`. This calls the real function, not a shim:

```go
func TestCanAuthorCourses(t *testing.T) {
	want := map[string]bool{
		RoleSuperAdmin:  true,
		RoleAdminStore:  true,
		RoleAdminExam:   true,
		RoleAdminSchool: false,
		RoleStudent:     false,
		"":              false,
		"nonsense":      false,
	}
	for role, expected := range want {
		if got := canAuthorCourses(role); got != expected {
			t.Errorf("canAuthorCourses(%q) = %v, want %v", role, got, expected)
		}
	}
}
```

- [ ] **Step 6: Run it and watch it fail to compile**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestCanAuthorCourses
```

Expected: FAIL — `undefined: canAuthorCourses`.

- [ ] **Step 7: Add the helper and replace all eleven guards**

In `backend/internal/service/course.go`, add near the top of the file, below the imports:

```go
// canAuthorCourses gates every course, section and lesson write. It is one
// function rather than eleven inline comparisons because the eleventh method
// is how a role check goes missing.
func canAuthorCourses(role string) bool {
	return role == RoleAdminExam || role == RoleAdminStore || role == RoleSuperAdmin
}
```

Then at each of these eleven line numbers, replace `if role != RoleAdminStore && role != RoleSuperAdmin {` with `if !canAuthorCourses(role) {`. Leave the body (`return ..., ErrForbidden`) untouched.

`:18` CreateCourse · `:59` DeleteCourse · `:77` UpdateCourse · `:129` CreateSection · `:153` UpdateSection · `:166` DeleteSection · `:179` ReorderSections · `:203` CreateLesson · `:229` UpdateLesson · `:247` DeleteLesson · `:260` ReorderLessons

Verify you got all of them — this must print nothing:

```bash
grep -n "RoleAdminStore && role != RoleSuperAdmin" backend/internal/service/course.go
```

- [ ] **Step 8: Make the test shims delegate instead of duplicating**

In `backend/internal/service/course_test.go`, the two shim types re-implement the guard. Replace the guard in **every** shim method with the same helper call, so the shim can never disagree with the policy again:

```bash
grep -n "RoleAdminStore && role != RoleSuperAdmin" backend/internal/service/course_test.go
```

For each hit, replace `if role != RoleAdminStore && role != RoleSuperAdmin {` with `if !canAuthorCourses(role) {`. Expected hits: eleven in `shimCourseService`, one in `shimGetDeleteCourse` (`:569`). Re-run the grep afterwards; it must print nothing.

- [ ] **Step 9: Invert the three assertions that now state the opposite of policy**

These tests assert `admin_exam` is rejected. That is the behaviour we just removed.

`course_test.go:666` — rename `TestCreateCourse_RejectsNonStoreRole` → `TestCreateCourse_RoleGate` and replace its body:

```go
func TestCreateCourse_RoleGate(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	// admin_school is still rejected.
	_, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminSchool)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for admin_school, got %v", err)
	}

	// admin_exam now authors courses.
	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminExam)
	if err != nil {
		t.Fatalf("admin_exam CreateCourse: %v", err)
	}
	if course.Title != "Math" {
		t.Errorf("want title Math, got %s", course.Title)
	}

	// admin_store is unaffected.
	if _, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore); err != nil {
		t.Fatalf("admin_store CreateCourse: %v", err)
	}
}
```

`course_test.go:691` — rename `TestUpdateCourse_RejectsNonStoreRole` → `TestUpdateCourse_RoleGate` and replace its body:

```go
func TestUpdateCourse_RoleGate(t *testing.T) {
	ctx := context.Background()
	fake := newFakeCourseRepo()
	svc := &shimCourseService{fake: fake}

	course, err := svc.CreateCourse(ctx, "Math", "beginner", "math", "Mr. A", RoleAdminStore)
	if err != nil {
		t.Fatalf("CreateCourse: %v", err)
	}

	_, err = svc.UpdateCourse(ctx, course.ID.String(), "Updated", "advanced", "science", "Mr. B", RoleAdminSchool)
	if !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for admin_school, got %v", err)
	}

	updated, err := svc.UpdateCourse(ctx, course.ID.String(), "Updated", "advanced", "science", "Mr. B", RoleAdminExam)
	if err != nil {
		t.Fatalf("admin_exam UpdateCourse: %v", err)
	}
	if updated.Title != "Updated" {
		t.Errorf("want title Updated, got %s", updated.Title)
	}
}
```

`course_test.go:1201` — inside `TestDeleteCourse_RBACAndDelete`, replace the rejection block:

```go
	// admin_school is still rejected.
	if err := gdSvc.DeleteCourse(ctx, course.ID.String(), RoleAdminSchool); !errors.Is(err, ErrForbidden) {
		t.Errorf("want ErrForbidden for admin_school, got %v", err)
	}
```

Leave the rest of that test (the `RoleAdminStore` delete and the gone-check) alone.

- [ ] **Step 10: Run the course suite**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run 'TestCanAuthorCourses|TestCreateCourse|TestUpdateCourse|TestDeleteCourse|TestSection|TestLesson' -v
```

Expected: PASS. `TestSection_RejectsNonStoreRole` and the lesson equivalent use `RoleStudent`, which is still forbidden, so they need no edit.

- [ ] **Step 11: Write the failing capability test**

Create `backend/internal/service/rbac_test.go`:

```go
package service

import "testing"

// TestAdminExamCapabilities pins the admin_exam capability list. Its TypeScript
// counterpart is web/lib/hooks/use-capability.test.ts, which asserts the same
// literal strings. The two suites share no definition — this only makes a
// divergence between the two languages fail loudly.
func TestAdminExamCapabilities(t *testing.T) {
	want := []string{
		"questions:*",
		"tests:*",
		"products(exam):*",
		"products(course):*",
		"sections:*",
		"sessions:*",
		"uploads:write",
	}
	got := Capabilities(RoleAdminExam)
	if len(got) != len(want) {
		t.Fatalf("admin_exam caps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cap[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAdminExamRetainsExamReadCapability guards the collapse that looks tidy and
// is not: HasCapability matches exact strings and ":*" prefixes only, so a
// merged "products(exam|course):*" entry would stop matching this and lock the
// exam admin out of the /admin/exams read group (routes.go:293).
func TestAdminExamRetainsExamReadCapability(t *testing.T) {
	for _, required := range []string{"products(exam):read", "products(exam):write", "products(course):write"} {
		if !HasCapability(RoleAdminExam, required) {
			t.Errorf("HasCapability(admin_exam, %q) = false, want true", required)
		}
	}
	for _, required := range []string{"orders:write", "revenue:read", "schools:write"} {
		if HasCapability(RoleAdminExam, required) {
			t.Errorf("HasCapability(admin_exam, %q) = true, want false", required)
		}
	}
}
```

- [ ] **Step 12: Run it and watch it fail**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run 'TestAdminExam' -v
```

Expected: FAIL — `admin_exam caps = [questions:* tests:* products(exam):* sessions:* uploads:write], want [...]`.

- [ ] **Step 13: Update the capability list**

`backend/internal/service/rbac.go:15`. Replace the `RoleAdminExam` line:

```go
	RoleAdminExam:   {"questions:*", "tests:*", "products(exam):*", "products(course):*", "sections:*", "sessions:*", "uploads:write"},
```

Two entries, not `products(exam|course):*` — `HasCapability` reads the pipe as a literal character, and `products(exam):*` must survive verbatim for `routes.go:293`. Leave `RoleAdminStore` alone, including its inert `products(book|course):write`.

- [ ] **Step 14: Run it and watch it pass**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run 'TestAdminExam' -v
```

Expected: PASS.

- [ ] **Step 15: Run the whole service package**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go vet ./... && go test -race -shuffle=on -timeout 25m ./internal/service/
```

Expected: PASS. `-timeout 25m` because this package runs testcontainers and exceeds Go's 10-minute default. If anything outside `course`/`store`/`rbac` fails, it is not yours — report it, do not fix it.

- [ ] **Step 16: Commit**

```bash
git add backend/internal/service/store.go backend/internal/service/course.go backend/internal/service/rbac.go backend/internal/service/store_test.go backend/internal/service/course_test.go backend/internal/service/rbac_test.go && git commit -m "feat: admin_exam may author courses and create course products"
```

---

# Task 2: Frontend — capability mirror and Katalog nav

**Files:**
- Modify: `web/lib/hooks/use-capability.ts:13-17`
- Modify: `web/lib/nav-config.ts:109-114`
- Modify: `web/lib/i18n.ts` (both locale blocks)
- Create: `web/lib/hooks/use-capability.test.ts`

**Interfaces:**
- Consumes: nothing. The capability list below is spelled out verbatim; do not go read `rbac.go` for it.
- Produces: the i18n key `nav_group_catalog`. No other task uses it.

- [ ] **Step 1: Write the failing test**

Create `web/lib/hooks/use-capability.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { hasCapability } from "./use-capability";

// Go counterpart: backend/internal/service/rbac_test.go
describe("admin_exam capabilities", () => {
  it.each([
    "questions:*",
    "tests:*",
    "products(exam):*",
    "products(exam):read",
    "products(course):*",
    "sections:*",
    "sessions:*",
    "uploads:write",
  ])("grants '%s'", (cap) => {
    expect(hasCapability("admin_exam", cap)).toBe(true);
  });

  it.each(["orders:write", "revenue:read", "schools:write", "students:*"])(
    "withholds '%s'",
    (cap) => {
      expect(hasCapability("admin_exam", cap)).toBe(false);
    },
  );
});

describe("other roles are unchanged", () => {
  it("super_admin keeps the wildcard", () => {
    expect(hasCapability("super_admin", "anything:at:all")).toBe(true);
  });

  it("admin_store keeps orders and loses revenue", () => {
    expect(hasCapability("admin_store", "orders:*")).toBe(true);
    expect(hasCapability("admin_store", "revenue:read")).toBe(false);
  });

  it("an unknown role gets nothing", () => {
    expect(hasCapability("nonsense", "orders:*")).toBe(false);
    expect(hasCapability(undefined, "orders:*")).toBe(false);
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd web && npx vitest run lib/hooks/use-capability.test.ts
```

Expected: FAIL on `products(course):*` and `products(course)` variants — `admin_exam` does not have them yet. The `products(exam):read` case should already pass.

- [ ] **Step 3: Update the mirror**

`web/lib/hooks/use-capability.ts:13`. Replace the `admin_exam` line:

```ts
  admin_exam: ["questions:*", "tests:*", "products(exam):*", "products(course):*", "sections:*", "sessions:*", "uploads:write"],
```

Leave every other row alone — including `admin_store`'s `products(book|course):write`, which has never matched anything but is not this task's problem.

- [ ] **Step 4: Run it and watch it pass**

```bash
cd web && npx vitest run lib/hooks/use-capability.test.ts
```

Expected: PASS.

- [ ] **Step 5: Add the Katalog locale strings**

`web/lib/i18n.ts`. In the **id** block, beside `nav_group_exam` at `:26`:

```ts
    nav_group_catalog: "Katalog",
```

In the **en** block, beside `nav_group_exam` at `:1412`:

```ts
    nav_group_catalog: "Catalog",
```

Both are required — `NavGroup.titleKey` is typed as `keyof DICT["id"]`, and a key missing from `en` is a runtime blank in English.

- [ ] **Step 6: Add the nav group**

`web/lib/nav-config.ts:109`. Replace `ADMIN_EXAM_NAV` in full:

```ts
export const ADMIN_EXAM_NAV: RoleNavConfig = [
  {
    titleKey: "nav_group_exam",
    items: EXAM_NAV_ITEMS,
  },
  {
    titleKey: "nav_group_catalog",
    items: [
      { labelKey: "admin_nav_products", href: "/admin/products", icon: Package },
      { labelKey: "admin_nav_courses", href: "/admin/courses", icon: Library },
    ],
  },
];
```

`Package` and `Library` are already imported at `:8` and `:9`. `admin_nav_products` and `admin_nav_courses` already exist in both locales (`:17`, `:18`, `:1403`, `:1404`). Neither route has a per-page role guard — `app/(admin)/layout.tsx` only selects a nav config — so this is the whole routing change.

- [ ] **Step 7: Verify the nav renders**

```bash
cd web && npx vitest run lib/hooks/use-capability.test.ts lib/
```

Expected: PASS. If an existing test snapshots the exam-admin sidebar it will fail on the new group — update the expectation to include Katalog, Produk, Kursus. Do not delete the assertion.

- [ ] **Step 8: Commit**

```bash
git add web/lib/hooks/use-capability.ts web/lib/hooks/use-capability.test.ts web/lib/nav-config.ts web/lib/i18n.ts && git commit -m "feat: exam admin sidebar gains Katalog group; capability mirror follows rbac.go"
```

---

# Task 3: Frontend — product type lists follow the signed-in role

**Files:**
- Create: `web/lib/product-types.ts`
- Create: `web/lib/product-types.test.ts`
- Modify: `web/app/(admin)/admin/products/page.tsx:22` and `:56-70`, `:163`
- Modify: `web/components/admin/ProductModal.tsx:33`

**Interfaces:**
- Consumes: `UserRole` (type only) from `@/lib/nav-config`; `ProductType` from `@/lib/types`. Both already exist and neither is modified by any other task.
- Produces: `writableProductTypes(role: UserRole | undefined): ProductType[]`.

This is the frontend mirror of `checkTypeRBAC`, deliberately separate from `use-capability.ts`'s `ROLE_CAPABILITIES` — the backend splits the same way (`checkTypeRBAC` gates products, `rbac.go` describes them), so the frontend matching that split keeps the two mirrors readable against their own source.

- [ ] **Step 1: Write the failing test**

Create `web/lib/product-types.test.ts`:

```ts
import { describe, it, expect } from "vitest";
import { writableProductTypes } from "./product-types";

// Go counterpart: checkTypeRBAC in backend/internal/service/store.go,
// pinned by TestCreateProduct_TypeRBAC.
describe("writableProductTypes", () => {
  it("gives super_admin every type", () => {
    expect(writableProductTypes("super_admin")).toEqual([
      "book",
      "course",
      "exam",
      "merchandise",
      "medal",
    ]);
  });

  it("gives admin_store every type", () => {
    expect(writableProductTypes("admin_store")).toEqual([
      "book",
      "course",
      "exam",
      "merchandise",
      "medal",
    ]);
  });

  it("gives admin_exam only the digital types", () => {
    expect(writableProductTypes("admin_exam")).toEqual(["course", "exam"]);
  });

  it.each(["admin_school", "student", undefined] as const)(
    "gives %s nothing",
    (role) => {
      expect(writableProductTypes(role)).toEqual([]);
    },
  );
});
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd web && npx vitest run lib/product-types.test.ts
```

Expected: FAIL — cannot resolve `./product-types`.

- [ ] **Step 3: Create the helper**

Create `web/lib/product-types.ts`:

```ts
import type { UserRole } from "@/lib/nav-config";
import type { ProductType } from "@/lib/types";

const ALL_TYPES: ProductType[] = ["book", "course", "exam", "merchandise", "medal"];
const DIGITAL_TYPES: ProductType[] = ["course", "exam"];

// Mirrors checkTypeRBAC in backend/internal/service/store.go. The server is the
// real boundary — this only decides what to render, so drift here is a cosmetic
// bug, not a hole.
export function writableProductTypes(role: UserRole | undefined): ProductType[] {
  switch (role) {
    case "super_admin":
    case "admin_store":
      return ALL_TYPES;
    case "admin_exam":
      return DIGITAL_TYPES;
    default:
      return [];
  }
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd web && npx vitest run lib/product-types.test.ts
```

Expected: PASS.

- [ ] **Step 5: Scope the products page**

`web/app/(admin)/admin/products/page.tsx`.

Delete the module-level constant at `:22`:

```ts
const FILTER_TYPES: (ProductType | "all")[] = ["all", "book", "course", "exam", "merchandise", "medal"];
```

Add to the imports:

```ts
import { useResolvedAdminRole } from "@/lib/hooks/use-capability";
import { writableProductTypes } from "@/lib/product-types";
```

Inside `ProductsPage`, after `const { data: products, isLoading, isError, error } = useAdminProducts();`:

```ts
  const { role } = useResolvedAdminRole();
  const writableTypes = useMemo(() => writableProductTypes(role), [role]);
  const filterTypes = useMemo<(ProductType | "all")[]>(
    () => ["all", ...writableTypes],
    [writableTypes],
  );
```

Replace the filter body at `:68` so rows outside the role's remit never render — an exam admin should not see a `book` row it cannot open:

```ts
    const visible = products.filter((p) => writableTypes.includes(p.type));
    if (filter === "all") return visible;
    return visible.filter((p) => p.type === filter);
```

At `:163`, change `FILTER_TYPES.map((ft) => (` to `filterTypes.map((ft) => (`.

`useMemo` is already imported at `:3`.

- [ ] **Step 6: Scope the create dropdown**

`web/components/admin/ProductModal.tsx`.

Delete the module-level constant at `:33`:

```ts
const PRODUCT_TYPES: ProductType[] = ["book", "course", "exam", "merchandise", "medal"];
```

Add to the imports:

```ts
import { useResolvedAdminRole } from "@/lib/hooks/use-capability";
import { writableProductTypes } from "@/lib/product-types";
```

Inside the component, beside the other hooks (near `:81`):

```ts
  const { role } = useResolvedAdminRole();
  const productTypes = writableProductTypes(role);
```

Then replace every `PRODUCT_TYPES` reference in the JSX with `productTypes`. Find them with:

```bash
grep -n "PRODUCT_TYPES" web/components/admin/ProductModal.tsx
```

Leave `TYPE_LABELS` alone — it is a full `Record<ProductType, string>` and stays complete, because the edit path still renders the label of a pinned type the current role may not create.

The edit path already pins the type (`effectiveType = isEdit ? product?.type : type` at `:144`) and does not show the dropdown, so nothing here can change an existing product's type.

- [ ] **Step 7: Give the two existing suites a role**

Both suites render with an empty auth store, so `useResolvedAdminRole()` returns `role: undefined`, `writableProductTypes` returns `[]`, and the table and dropdown both go empty — every existing assertion fails.

Do **not** fix this by making `writableProductTypes(undefined)` return every type. An unauthenticated fallback that grants everything is the exact shape of a real bug.

Do **not** mock `@/stores/auth` the way `admin-orders.test.tsx` does either — that mock supplies only `getState()`, while `useResolvedAdminRole` calls `useAuthStore((s) => s.token)` as a hook with a selector. It would throw.

Mock the resolver directly. Add to both `web/app/(admin)/admin/products/page.test.tsx` and `web/components/admin/ProductModal.test.tsx`, beside the other `vi.mock` calls at the top:

```ts
let mockRole = "admin_store";

vi.mock("@/lib/hooks/use-capability", () => ({
  useResolvedAdminRole: () => ({ role: mockRole, hydrated: true, meIsError: false }),
}));
```

and reset it in each suite's existing `beforeEach`:

```ts
    mockRole = "admin_store";
```

- [ ] **Step 8: Run them and watch them pass**

```bash
cd web && npx vitest run lib/product-types.test.ts "app/(admin)/admin/products/page.test.tsx" components/admin/ProductModal.test.tsx
```

Expected: PASS, with every pre-existing assertion intact. If you had to weaken an existing assertion to get green, stop and report — that is the change breaking something, not the test being wrong.

- [ ] **Step 9: Write the role-scoping test**

The suites now pass as `admin_store`, which proves nothing about the actual feature. Append to `web/app/(admin)/admin/products/page.test.tsx`, inside the existing `describe("ProductsPage", ...)`. The fixture at `:49` is one `book` (Buku Matematika), one `course` (Kursus Fisika), one `exam` (Paket UTBK):

```tsx
  it("shows an exam admin only the digital products", async () => {
    mockRole = "admin_exam";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    });
    expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
    expect(screen.queryByText("Buku Matematika")).not.toBeInTheDocument();
  });

  it("offers an exam admin only the digital filter tabs", async () => {
    mockRole = "admin_exam";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Kursus" })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Ujian" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Buku" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Merchandise" })).not.toBeInTheDocument();
  });

  it("shows a store admin every product", async () => {
    mockRole = "admin_store";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText("Buku Matematika")).toBeInTheDocument();
    });
    expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
  });
```

- [ ] **Step 10: Run it and watch it pass**

```bash
cd web && npx vitest run "app/(admin)/admin/products/page.test.tsx"
```

Expected: PASS. Before trusting the two `queryByText(...).not` assertions, flip `mockRole` to `"admin_store"` in the first new test and confirm it **fails** — an assertion that something is absent proves nothing until you have seen it fire. Then flip it back.

- [ ] **Step 11: Commit**

```bash
git add web/lib/product-types.ts web/lib/product-types.test.ts "web/app/(admin)/admin/products/page.tsx" web/components/admin/ProductModal.tsx "web/app/(admin)/admin/products/page.test.tsx" web/components/admin/ProductModal.test.tsx && git commit -m "feat: product type filters and create dropdown follow the signed-in role"
```

**Note for T3's implementer:** `web/app/(admin)/admin/products/page.test.tsx` and `web/components/admin/ProductModal.test.tsx` are yours exclusively — no other task touches them. They are omitted from the file table at the top only because they are modified rather than created; stage them.

---

# Task 4: Frontend — lesson player stops leaking to YouTube

**Files:**
- Modify: `web/components/courses/VideoPlayer.tsx` (full rewrite below `toYoutubeId`)
- Create: `web/components/courses/VideoPlayer.test.tsx`

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `VideoPlayer({ videoRef, title })` — the props are unchanged, so its one caller (`web/app/(student)/courses/[id]/page.tsx:153`) needs no edit.

### What you are actually fixing

The player already embeds rather than links out. The leak is YouTube's own chrome: hovering the iframe raises a title bar carrying the video title and the YouTube logo, both of which navigate to youtube.com. **No iframe parameter removes this** — `modestbranding` was retired and `rel=0` only limits related videos. The fix is an overlay that absorbs pointer events, which means hover never reaches the iframe, which means the title bar never renders at all. That is why the shield must have no gaps.

- [ ] **Step 1: Write the failing test**

Create `web/components/courses/VideoPlayer.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { VideoPlayer } from "./VideoPlayer";

describe("VideoPlayer", () => {
  beforeEach(() => {
    vi.stubGlobal("YT", undefined);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("renders the empty state when there is no video", () => {
    render(<VideoPlayer title="Pelajaran 1" />);
    expect(screen.getByText(/Video belum tersedia/i)).toBeInTheDocument();
  });

  it("renders a click shield over the player", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    expect(container.querySelector('[data-testid="video-shield"]')).not.toBeNull();
  });

  it("exposes no link to youtube.com anywhere in the tree", () => {
    const { container } = render(
      <VideoPlayer videoRef="https://www.youtube.com/watch?v=abc123" title="L1" />,
    );
    const hrefs = Array.from(container.querySelectorAll("a")).map((a) => a.getAttribute("href") ?? "");
    expect(hrefs.some((h) => h.includes("youtube.com") || h.includes("youtu.be"))).toBe(false);
  });

  it("renders our own control bar, not YouTube's", () => {
    render(<VideoPlayer videoRef="abc123" title="L1" />);
    expect(screen.getByRole("button", { name: /putar|play/i })).toBeInTheDocument();
    expect(screen.getByRole("slider", { name: /posisi|progress/i })).toBeInTheDocument();
  });

  it("falls back to a plain embed when the IFrame API fails to load", async () => {
    const { container } = render(
      <VideoPlayer videoRef="abc123" title="L1" forceFallback />,
    );
    const iframe = container.querySelector("iframe");
    expect(iframe).not.toBeNull();
    expect(container.querySelector('[data-testid="video-shield"]')).toBeNull();
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd web && npx vitest run components/courses/VideoPlayer.test.tsx
```

Expected: FAIL — the empty-state test passes, the other four fail (no shield, no control bar, no `forceFallback` prop).

- [ ] **Step 3: Rewrite the component**

Replace `web/components/courses/VideoPlayer.tsx` in full. `toYoutubeId` is carried over **unchanged** — do not touch its logic.

```tsx
"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Maximize, Pause, PlayCircle, Play, Volume2, VolumeX } from "lucide-react";

interface VideoPlayerProps {
  videoRef?: string;
  title?: string;
  /** Test seam: render the no-JS-API fallback without stubbing the network. */
  forceFallback?: boolean;
}

function toYoutubeId(value?: string): string | null {
  if (!value) return null;
  const trimmed = value.trim();
  if (!trimmed) return null;
  if (/^https?:\/\//i.test(trimmed)) {
    try {
      const url = new URL(trimmed);
      if (/youtube\.com$/i.test(url.hostname)) {
        // /watch?v=VIDEO_ID
        const v = url.searchParams.get("v");
        if (v) return v;
        // /shorts/VIDEO_ID  or  /embed/VIDEO_ID
        const match = url.pathname.match(/\/(shorts|embed)\/([^/?]+)/);
        if (match?.[2]) return match[2];
      }
      if (/youtu\.be$/i.test(url.hostname)) {
        const id = url.pathname.replace(/^\//, "").split("?")[0];
        if (id) return id;
      }
      return null;
    } catch {
      return null;
    }
  }
  return trimmed;
}

interface YTPlayer {
  playVideo(): void;
  pauseVideo(): void;
  seekTo(seconds: number, allowSeekAhead: boolean): void;
  getCurrentTime(): number;
  getDuration(): number;
  getVideoLoadedFraction(): number;
  setVolume(volume: number): void;
  mute(): void;
  unMute(): void;
  destroy(): void;
}

type YTGlobal = {
  YT?: { Player: new (el: HTMLElement, opts: unknown) => YTPlayer };
  onYouTubeIframeAPIReady?: () => void;
};

// One script tag per page, shared by every player instance on it.
let apiPromise: Promise<void> | null = null;

function loadYoutubeApi(): Promise<void> {
  if (apiPromise) return apiPromise;
  apiPromise = new Promise<void>((resolve, reject) => {
    const w = window as unknown as YTGlobal;
    if (w.YT?.Player) {
      resolve();
      return;
    }
    const previous = w.onYouTubeIframeAPIReady;
    w.onYouTubeIframeAPIReady = () => {
      previous?.();
      resolve();
    };
    const script = document.createElement("script");
    script.src = "https://www.youtube.com/iframe_api";
    script.async = true;
    script.onerror = () => reject(new Error("youtube iframe_api failed to load"));
    document.head.appendChild(script);
  });
  return apiPromise;
}

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const m = Math.floor(seconds / 60);
  const s = Math.floor(seconds % 60);
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function VideoPlayer({ videoRef, title, forceFallback }: VideoPlayerProps) {
  const id = toYoutubeId(videoRef);

  const wrapperEl = useRef<HTMLDivElement | null>(null);
  const mountEl = useRef<HTMLDivElement | null>(null);
  const player = useRef<YTPlayer | null>(null);

  const [fallback, setFallback] = useState(Boolean(forceFallback));
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(false);
  const [position, setPosition] = useState(0);
  const [duration, setDuration] = useState(0);
  const [loaded, setLoaded] = useState(0);

  useEffect(() => {
    if (!id || forceFallback) return;
    let cancelled = false;

    loadYoutubeApi()
      .then(() => {
        if (cancelled || !mountEl.current) return;
        const w = window as unknown as YTGlobal;
        if (!w.YT?.Player) throw new Error("YT.Player missing after ready");
        player.current = new w.YT.Player(mountEl.current, {
          videoId: id,
          playerVars: {
            enablejsapi: 1,
            controls: 0,
            rel: 0,
            disablekb: 1,
            playsinline: 1,
            // No modestbranding — YouTube retired it. The shield is what
            // removes the branding; a dead parameter here would only suggest
            // otherwise to the next reader.
            origin: window.location.origin,
          },
          host: "https://www.youtube-nocookie.com",
          events: {
            onReady: () => {
              if (cancelled) return;
              setDuration(player.current?.getDuration() ?? 0);
            },
            // YT.PlayerState.PLAYING === 1
            onStateChange: (e: { data: number }) => {
              if (cancelled) return;
              setPlaying(e.data === 1);
              setDuration(player.current?.getDuration() ?? 0);
            },
            onError: () => {
              if (!cancelled) setFallback(true);
            },
          },
        });
      })
      .catch(() => {
        if (!cancelled) setFallback(true);
      });

    return () => {
      cancelled = true;
      player.current?.destroy();
      player.current = null;
    };
  }, [id, forceFallback]);

  useEffect(() => {
    if (!playing) return;
    const tick = setInterval(() => {
      const p = player.current;
      if (!p) return;
      setPosition(p.getCurrentTime());
      setLoaded(p.getVideoLoadedFraction());
    }, 250);
    return () => clearInterval(tick);
  }, [playing]);

  const togglePlay = useCallback(() => {
    const p = player.current;
    if (!p) return;
    if (playing) p.pauseVideo();
    else p.playVideo();
  }, [playing]);

  const toggleMute = useCallback(() => {
    const p = player.current;
    if (!p) return;
    if (muted) p.unMute();
    else p.mute();
    setMuted(!muted);
  }, [muted]);

  const seek = useCallback((next: number) => {
    player.current?.seekTo(next, true);
    setPosition(next);
  }, []);

  // Fullscreen targets the wrapper, never the iframe: fullscreening the iframe
  // would hand the viewport to YouTube's chrome and undo the shield.
  const goFullscreen = useCallback(() => {
    wrapperEl.current?.requestFullscreen?.();
  }, []);

  if (!id) {
    return (
      <div
        className="overflow-hidden rounded-lg border border-line bg-ink-900"
        style={{ aspectRatio: "16 / 9" }}
      >
        <div className="flex size-full flex-col items-center justify-center gap-3 text-ink-400">
          <PlayCircle size={48} strokeWidth={1.5} />
          <div className="text-center">
            <p className="text-sm font-medium text-ink-300">
              {title ? `${title}` : "Video pelajaran"}
            </p>
            <p className="mt-1 text-xs text-ink-500">
              Video belum tersedia. Hubungi admin untuk informasi lebih lanjut.
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (fallback) {
    return (
      <div
        className="overflow-hidden rounded-lg border border-line bg-ink-900"
        style={{ aspectRatio: "16 / 9" }}
      >
        <iframe
          title={title ?? "Lesson video"}
          src={`https://www.youtube-nocookie.com/embed/${encodeURIComponent(id)}?rel=0`}
          allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
          allowFullScreen
          className="block size-full border-0"
        />
      </div>
    );
  }

  return (
    <div
      ref={wrapperEl}
      className="relative overflow-hidden rounded-lg border border-line bg-ink-900"
      style={{ aspectRatio: "16 / 9" }}
    >
      <div ref={mountEl} className="block size-full" />

      {/* Absorbs every pointer event so YouTube's title bar and logo never
          render, let alone become clickable. Must have no gaps. */}
      <div
        data-testid="video-shield"
        className="absolute inset-0 z-10"
        onContextMenu={(e) => e.preventDefault()}
      />

      <div className="absolute inset-x-0 bottom-0 z-20 flex items-center gap-3 bg-gradient-to-t from-ink-900/90 to-transparent px-3 pb-2 pt-6">
        <button
          type="button"
          onClick={togglePlay}
          aria-label={playing ? "Jeda" : "Putar"}
          className="shrink-0 text-white"
        >
          {playing ? <Pause className="size-5" /> : <Play className="size-5" />}
        </button>

        <span className="shrink-0 font-mono text-[11px] text-white/80">
          {formatTime(position)}
        </span>

        <div className="relative flex-1">
          <div
            className="pointer-events-none absolute inset-y-1/2 h-1 -translate-y-1/2 rounded bg-white/40"
            style={{ width: `${loaded * 100}%` }}
          />
          <input
            type="range"
            aria-label="Posisi video"
            min={0}
            max={Math.max(duration, 1)}
            step={1}
            value={position}
            onChange={(e) => seek(Number(e.target.value))}
            className="relative w-full accent-brand-600"
          />
        </div>

        <span className="shrink-0 font-mono text-[11px] text-white/80">
          {formatTime(duration)}
        </span>

        <button
          type="button"
          onClick={toggleMute}
          aria-label={muted ? "Bunyikan" : "Bisukan"}
          className="shrink-0 text-white"
        >
          {muted ? <VolumeX className="size-5" /> : <Volume2 className="size-5" />}
        </button>

        <button
          type="button"
          onClick={goFullscreen}
          aria-label="Layar penuh"
          className="shrink-0 text-white"
        >
          <Maximize className="size-5" />
        </button>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd web && npx vitest run components/courses/VideoPlayer.test.tsx
```

Expected: PASS, five tests. If the control-bar test cannot find the play button, check that `aria-label` is `"Putar"` — the test regex is `/putar|play/i`.

- [ ] **Step 5: Confirm the one caller still typechecks against these props**

`web/app/(student)/courses/[id]/page.tsx:153` passes `videoRef` and `title` only. `forceFallback` is optional, so the call site needs no change. Verify by reading it — do **not** run `tsc --noEmit`, which typechecks the whole project and will show you other agents' in-flight errors:

```bash
grep -n -A 3 "<VideoPlayer" "web/app/(student)/courses/[id]/page.tsx"
```

Expected: `videoRef={activeLesson?.video_url}` and `title={activeLesson?.title}`, nothing else.

- [ ] **Step 6: Commit**

```bash
git add web/components/courses/VideoPlayer.tsx web/components/courses/VideoPlayer.test.tsx && git commit -m "feat: lesson player shields YouTube chrome behind our own controls"
```

---

# Task 5: Integration — whole-project gates

**Run this alone, after T1–T4 have all committed.** Everything here typechecks or builds the entire project; running it while a sibling is mid-edit produces failures that belong to nobody.

**Files:** none owned. Fix only what the gates surface, and only in files T1–T4 touched.

- [ ] **Step 1: Confirm all four tasks landed**

```bash
git log --oneline -5
```

Expected: four feature commits above `acf598a`. If one is missing, stop and report which.

- [ ] **Step 2: Backend gate — the full pipeline script, not a subset**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && ./deploy/pipeline/backend.sh
```

Expected: PASS. This runs `go build ./...`, `go vet ./...` and `go test -race -shuffle=on -timeout 20m ./...`. Run the script rather than a hand-rolled `go vet ./internal/...`, which misses `integration/`. Takes roughly 10 minutes.

- [ ] **Step 3: Frontend typecheck**

```bash
cd web && npx tsc --noEmit
```

Expected: no output. vitest does not typecheck, so this is the first time the TypeScript in T2–T4 is checked as a whole.

- [ ] **Step 4: Full frontend suite**

```bash
cd web && npm run test:run
```

Expected: PASS.

- [ ] **Step 5: Next build**

```bash
cd web && npm run build
```

Expected: PASS. `tsc --noEmit` is not a substitute — Next rejects things tsc accepts, such as stray exports from a page module, and the images-web CI job only runs on `main`, so a branch that skips this can still break the deploy.

- [ ] **Step 6: Commit any gate fixes**

Only if steps 2–5 required edits. Stage exact paths; do not `git add -A`.

```bash
git add <exact paths> && git commit -m "fix: gate fixes for admin_exam catalogue + player"
```

- [ ] **Step 7: Report, do not push**

Report to the user: the five commits, the gate results with actual output, and the two verifications below that remain outstanding. Do not push and do not open a PR — the user does both.

---

## Verification this plan does not cover

Two things stay unproven when every step above is green. Say so plainly when reporting; do not describe the work as verified.

**1. The shield is untested by jsdom.** jsdom has no layout engine. `VideoPlayer.test.tsx` asserts the shield element exists and that no youtube.com anchor is in the tree — it cannot show the shield actually covers the iframe, or that the title bar is suppressed. That needs Playwright against a lesson with a real video: hover the player, screenshot to a real PNG file, confirm no YouTube title bar and no logo; then click where the title bar would be and confirm no navigation and no new tab; then enter fullscreen and confirm our control bar survives. Until that screenshot exists, item 2 of the spec is unverified.

**2. No end-to-end proof that an `admin_exam` session can actually author a course.** Every backend test in T1 runs against fakes and shims. The real path — log in as `admin_exam`, open `/admin/courses`, create a course, add a section and a lesson, then create a `course` product linked to it — is untested. Worth one manual pass against local Docker before this goes near production.

---

## Open

- `admin_store`'s `products(book|course):write` capability string has never matched anything: `HasCapability` reads the pipe as a literal. It is inert rather than harmful, since `checkTypeRBAC` is what actually gates `admin_store`. Deliberately untouched here; worth its own cleanup ticket.
- Covering the YouTube player conflicts with YouTube's embedded-player terms, which ask that the player not be obstructed. Accepted knowingly (spec §6). The exit if it ever matters is self-hosting the MP4s to GCS and swapping the iframe for a `<video>` tag — the control bar is already ours, so little else would move.
