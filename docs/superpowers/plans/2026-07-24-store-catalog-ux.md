# Store & Checkout UX Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework the student store catalog and checkout so products are browsable and legible, product specifications are visible, digital products can't be over-purchased, and shipping figures shown to buyers are never fabricated.

**Architecture:** Six independent slices on one branch. Slices 1, 4 and 5 are frontend-only. Slice 2 adds one JSONB column and threads it through repository → service → handler → admin form → detail page. Slices 3 and 6 close two live money bugs in the Go service layer. Each slice ends green and is independently shippable.

**Tech Stack:** Go 1.26 + Echo + pgx v5 + golang-migrate; Next.js App Router + React Query + Tailwind; Vitest + Testing Library (frontend); Go stdlib testing + testcontainers Postgres (backend).

## Global Constraints

- **Worktree:** all work happens in `.claude/worktrees/store-catalog-ux` on branch `feat/store-catalog-ux`. Never touch the main working directory — a concurrent session is preparing production CD there.
- **Base:** `origin/main` @ `dc53214`. The availability-window feature (`available_from` / `available_until`) does **not** exist on this base — it lives in unmerged PR #44. Do not reference those columns.
- **Migration number:** `0045`. Base is at `0036`; PR #44 holds `0037`–`0044`. Any other number collides.
- **Go commands must override GOROOT:** prefix every Go command with `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec &&`. Without it you get `package unsafe is not in std`.
- **Commits are authored by the user alone.** Never add a `Co-Authored-By: Claude` trailer or any Claude/Anthropic attribution.
- **Never test business rules through `shimService` / `shimOrderService`.** These are hand-written reimplementations in `store_test.go` — `shimService.GetShippingRates` (line 361) merely calls `s.logistics.GetRates` and contains none of the real fallback logic. That is precisely why the fabricated-rate bug survived. Rules must be proven against real code: either a pure function in the non-test package, or a DB-backed handler test using the testcontainers harness.
- **i18n:** every user-facing string goes in `web/lib/i18n.ts`, added to **both** the `id` block (starts line 8) and the `en` block (starts line 1027). Never hardcode copy in components.
- **Frontend tests:** `cd web && npx vitest run <path>`
- **Backend tests:** `export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/... -run <Name> -v`
- **Final gate before any slice is done:** run the full suite, not a narrow `-run` filter. A narrow filter has twice hidden real regressions in this repo.

---

## File Structure

**Slice 1 — catalog layout (frontend only)**
- Create `web/components/catalog/CategoryRail.tsx` — sticky category selector; desktop rail, mobile chip row. Owns nothing but selection.
- Modify `web/components/catalog/ProductCard.tsx` — 3:4 contain-fit cover; badge + title + price only.
- Modify `web/components/catalog/ProductCard.test.tsx` — existing assertions read `style.backgroundImage` on a div and will break once the cover becomes an `<img>`.
- Modify `web/app/(student)/catalog/page.tsx` — rail replaces tabs; grid to 5 columns; skeleton matches new card.
- Modify `web/lib/hooks/products.ts` — follow `next_cursor`.

**Slice 2 — product specs**
- Create `backend/db/migrations/0045_product_specs.up.sql` / `.down.sql`
- Create `backend/internal/service/product_specs.go` — pure validation, no DB.
- Create `backend/internal/service/product_specs_test.go`
- Modify `backend/internal/model/product.go`, `backend/internal/repository/product.go`, `backend/internal/handler/product.go`
- Create `backend/internal/handler/product_specs_handler_test.go` — DB-backed round trip.
- Create `web/lib/product-specs.ts` — canonical field list per product type.
- Create `web/components/admin/ProductSpecsEditor.tsx` + test
- Modify `web/components/admin/ProductModal.tsx`
- Create `web/components/catalog/ProductSpecTable.tsx` + test
- Modify `web/app/(student)/catalog/[id]/page.tsx`

**Slice 3 — digital qty guard**
- Create `backend/internal/service/item_qty.go` — pure rule.
- Create `backend/internal/service/item_qty_test.go`
- Modify `backend/internal/service/store.go` (`AddItem`, `UpdateItemQty`)
- Modify `backend/internal/handler/errors.go` — map the new sentinel error.
- Modify `web/app/(student)/catalog/[id]/page.tsx`, `web/components/cart/CartLineItem.tsx`

**Slice 4 — order shipping block (frontend only)**
- Create `web/components/orders/ShippingInfo.tsx` + test
- Modify `web/app/(student)/orders/[id]/page.tsx`

**Slice 5 — checkout address (frontend only)**
- Create `web/components/cart/ShippingAddressSummary.tsx` + test
- Modify `web/components/cart/ShippingAddressForm.tsx`, `web/app/(student)/cart/page.tsx`

**Slice 6 — stop fabricated rates**
- Create `backend/internal/service/shipping_rates.go` — pure resolver.
- Create `backend/internal/service/shipping_rates_test.go`
- Modify `backend/internal/service/ports_logistics.go`, `backend/internal/service/store.go`, `backend/internal/service/store_test.go`
- Modify `web/components/cart/CourierRateList.tsx`, `web/app/(student)/cart/page.tsx`, `web/components/orders/ShippingInfo.tsx`

---

# Slice 1 — Catalog layout

### Task 1: ProductCard — 3:4 contain-fit cover, title and price only

**Files:**
- Modify: `web/components/catalog/ProductCard.tsx`
- Test: `web/components/catalog/ProductCard.test.tsx`

**Interfaces:**
- Consumes: `Product` from `@/lib/types`, `fileUrl` from `@/lib/api`
- Produces: `ProductCard({ product, className })` — unchanged props. The cover element becomes an `<img>` with `alt={product.name}` when `product.image_url` is set, and a gradient `<div>` with an icon otherwise. Task 3 relies on the card filling its grid cell.

- [ ] **Step 1: Rewrite the existing test for the new cover contract**

The current test reads `style.backgroundImage` off a `<div>`. That contract is being replaced. Replace the whole file:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProductCard } from "./ProductCard";

describe("ProductCard", () => {
  it("renders the cover as a contain-fitted image, resolving object keys and preserving absolute URLs", () => {
    const { rerender } = render(
      <ProductCard
        product={{
          id: "merch-key",
          type: "merchandise",
          name: "Kaos Akademi",
          price: 75000,
          image_url: "avatars/store/tee.png",
        }}
      />,
    );

    let img = screen.getByAltText("Kaos Akademi") as HTMLImageElement;
    expect(img.src).toContain("http://localhost:8080/api/v1/files/avatars/store/tee.png");
    expect(img.className).toContain("object-contain");

    rerender(
      <ProductCard
        product={{
          id: "merch-legacy",
          type: "merchandise",
          name: "Tote Akademi",
          price: 50000,
          image_url: "https://cdn.example.com/tote.png",
        }}
      />,
    );

    img = screen.getByAltText("Tote Akademi") as HTMLImageElement;
    expect(img.src).toContain("https://cdn.example.com/tote.png");
  });

  it("falls back to a gradient placeholder that fills the same 3:4 box when there is no image", () => {
    render(
      <ProductCard product={{ id: "medal-fallback", type: "medal", name: "Medali", price: 10000 }} />,
    );

    expect(screen.queryByAltText("Medali")).toBeNull();
    const box = screen.getByTestId("product-cover");
    expect(box.style.background).toContain("linear-gradient");
    expect(box.className).toContain("aspect-[3/4]");
  });

  it("shows only the type badge, name and price — no description", () => {
    render(
      <ProductCard
        product={{
          id: "book-1",
          type: "book",
          name: "Kumpulan Soal KoSSMI Fisika",
          description: "Deskripsi panjang yang tidak boleh muncul di kartu.",
          price: 20000,
        }}
      />,
    );

    expect(screen.getByText("Buku")).toBeTruthy();
    expect(screen.getByText("Kumpulan Soal KoSSMI Fisika")).toBeTruthy();
    expect(screen.getByText("Rp20.000")).toBeTruthy();
    expect(screen.queryByText(/Deskripsi panjang/)).toBeNull();
  });
});
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
cd web && npx vitest run components/catalog/ProductCard.test.tsx
```

Expected: FAIL — `getByAltText` finds nothing, because the cover is still a background-image div.

- [ ] **Step 3: Rewrite the component**

Replace the body of `ProductCard.tsx` from the `export function ProductCard` declaration to the end of the file. Keep `TYPE_META` and `COVER_GRADIENT` exactly as they are.

```tsx
export function ProductCard({ product, className }: ProductCardProps) {
  const meta = TYPE_META[product.type];
  const { Icon } = meta;
  const cover = fileUrl(product.image_url);

  return (
    <Link
      href={`/catalog/${product.id}`}
      className={cn(
        "group flex flex-col overflow-hidden rounded-lg border border-line bg-surface shadow-[var(--sh-sm)] transition-all hover:-translate-y-0.5 hover:shadow-[var(--sh-md)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring",
        className,
      )}
    >
      <div
        data-testid="product-cover"
        className="relative flex aspect-[3/4] items-center justify-center bg-paper"
        style={cover ? undefined : { background: COVER_GRADIENT[product.type] }}
      >
        {cover ? (
          <img
            src={cover}
            alt={product.name}
            loading="lazy"
            className="size-full object-contain p-2"
          />
        ) : (
          <Icon className="size-10 text-white/90 drop-shadow-sm" strokeWidth={1.5} />
        )}
        <div className="absolute left-2 top-2">
          <Badge variant="outline" className={cn("border-transparent", meta.bg, meta.tone)}>
            {meta.label}
          </Badge>
        </div>
      </div>
      <div className="flex flex-1 flex-col gap-1 p-3">
        <div className="line-clamp-2 text-sm font-semibold leading-snug text-ink-900">
          {product.name}
        </div>
        <div className="mt-auto pt-2 font-serif text-base font-bold text-success">
          {formatRupiah(product.price)}
        </div>
      </div>
    </Link>
  );
}
```

- [ ] **Step 4: Run the test and watch it pass**

```bash
cd web && npx vitest run components/catalog/ProductCard.test.tsx
```

Expected: PASS, 3 tests.

- [ ] **Step 5: Commit**

```bash
git add web/components/catalog/ProductCard.tsx web/components/catalog/ProductCard.test.tsx
git commit -m "feat: contain-fit 3:4 product cover, drop description from card"
```

---

### Task 2: CategoryRail component

**Files:**
- Create: `web/components/catalog/CategoryRail.tsx`
- Test: `web/components/catalog/CategoryRail.test.tsx`
- Modify: `web/lib/i18n.ts`

**Interfaces:**
- Produces:
  ```ts
  export type CatalogCategory = "all" | "book" | "course" | "exam" | "merchandise" | "medal";
  export const CATALOG_CATEGORIES: { value: CatalogCategory; labelKey: string }[];
  export function CategoryRail(props: {
    value: CatalogCategory;
    onChange: (v: CatalogCategory) => void;
  }): JSX.Element;
  ```
  Task 3 imports all three.

- [ ] **Step 1: Add the i18n keys**

In `web/lib/i18n.ts`, in the `id` block immediately after the line `catalog_tab_competition: "Kompetisi",`:

```ts
    catalog_category_heading: "Kategori",
    catalog_tab_merchandise: "Merchandise",
    catalog_tab_medal: "Medali",
```

In the `en` block, immediately after that block's `catalog_tab_competition:` line:

```ts
    catalog_category_heading: "Category",
    catalog_tab_merchandise: "Merchandise",
    catalog_tab_medal: "Medals",
```

- [ ] **Step 2: Write the failing test**

Create `web/components/catalog/CategoryRail.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CATALOG_CATEGORIES, CategoryRail } from "./CategoryRail";

describe("CategoryRail", () => {
  it("lists all six categories, medal and merchandise included", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(CATALOG_CATEGORIES).toHaveLength(6);
    expect(screen.getByRole("button", { name: "Merchandise" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Medali" })).toBeTruthy();
  });

  it("marks only the selected category as current", () => {
    render(<CategoryRail value="book" onChange={() => {}} />);
    const selected = screen.getByRole("button", { name: "Buku" });
    expect(selected.getAttribute("aria-current")).toBe("true");
    expect(screen.getByRole("button", { name: "Kursus" }).getAttribute("aria-current")).toBeNull();
  });

  it("reports the clicked category", () => {
    const onChange = vi.fn();
    render(<CategoryRail value="all" onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Medali" }));
    expect(onChange).toHaveBeenCalledWith("medal");
  });

  it("stays reachable while the grid scrolls", () => {
    render(<CategoryRail value="all" onChange={() => {}} />);
    expect(screen.getByTestId("category-rail").className).toContain("md:sticky");
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/catalog/CategoryRail.test.tsx
```

Expected: FAIL — `Failed to resolve import "./CategoryRail"`.

- [ ] **Step 4: Write the component**

Create `web/components/catalog/CategoryRail.tsx`. The rail is deliberately borderless and card-less: a bordered panel here would read as a second navigation column next to the 252px `AppShell` rail.

```tsx
"use client";

import { useTranslation } from "@/lib/i18n";
import { cn } from "@/lib/utils";

export type CatalogCategory = "all" | "book" | "course" | "exam" | "merchandise" | "medal";

export const CATALOG_CATEGORIES: { value: CatalogCategory; labelKey: string }[] = [
  { value: "all", labelKey: "catalog_tab_all" },
  { value: "book", labelKey: "catalog_tab_book" },
  { value: "course", labelKey: "catalog_tab_course" },
  { value: "exam", labelKey: "catalog_tab_competition" },
  { value: "merchandise", labelKey: "catalog_tab_merchandise" },
  { value: "medal", labelKey: "catalog_tab_medal" },
];

export interface CategoryRailProps {
  value: CatalogCategory;
  onChange: (value: CatalogCategory) => void;
}

export function CategoryRail({ value, onChange }: CategoryRailProps) {
  const { t } = useTranslation();

  return (
    <nav
      data-testid="category-rail"
      aria-label={t("catalog_category_heading" as any)}
      className="md:sticky md:top-6 md:h-fit md:w-[200px] md:shrink-0 md:self-start"
    >
      <h2 className="mb-3 hidden text-xs font-semibold uppercase tracking-wide text-ink-500 md:block">
        {t("catalog_category_heading" as any)}
      </h2>
      <ul className="-mx-1 flex gap-2 overflow-x-auto px-1 pb-2 md:mx-0 md:flex-col md:gap-0.5 md:overflow-visible md:px-0 md:pb-0">
        {CATALOG_CATEGORIES.map((c) => {
          const active = c.value === value;
          return (
            <li key={c.value} className="shrink-0 md:shrink">
              <button
                type="button"
                onClick={() => onChange(c.value)}
                aria-current={active ? "true" : undefined}
                className={cn(
                  "whitespace-nowrap rounded-full border px-3.5 py-1.5 text-sm transition-colors md:w-full md:rounded-md md:border-0 md:px-2.5 md:text-left",
                  active
                    ? "border-brand-400 bg-brand-50 font-semibold text-brand-600 md:bg-brand-50"
                    : "border-line bg-surface text-ink-600 hover:bg-paper md:bg-transparent",
                )}
              >
                {t(c.labelKey as any)}
              </button>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}
```

- [ ] **Step 5: Run it and watch it pass**

```bash
cd web && npx vitest run components/catalog/CategoryRail.test.tsx
```

Expected: PASS, 4 tests.

- [ ] **Step 6: Commit**

```bash
git add web/components/catalog/CategoryRail.tsx web/components/catalog/CategoryRail.test.tsx web/lib/i18n.ts
git commit -m "feat: sticky catalog category rail with merchandise and medal"
```

---

### Task 3: Wire the rail into the catalog page and widen the grid

**Files:**
- Modify: `web/app/(student)/catalog/page.tsx`

**Interfaces:**
- Consumes: `CategoryRail`, `CatalogCategory` from Task 2; `useProducts` from `@/lib/hooks/products`
- Produces: nothing downstream.

- [ ] **Step 1: Replace the tabs with the rail**

In `web/app/(student)/catalog/page.tsx`, delete the `TabValue` type, the `TABS` constant, and the `Tabs`/`TabsList`/`TabsTrigger` import and JSX block. Replace the imports at the top:

```tsx
import { CategoryRail, type CatalogCategory } from "@/components/catalog/CategoryRail";
```

and remove:

```tsx
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
```

- [ ] **Step 2: Update `CatalogGrid` and `CatalogSkeleton` to five columns**

Change the `tab` prop type on `CatalogGrid` from `TabValue` to `CatalogCategory`, and replace both grid class strings — the one in `CatalogGrid` and the one in `CatalogSkeleton` — with:

```tsx
"grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5"
```

In `CatalogSkeleton`, replace each skeleton item body so it matches the new card shape:

```tsx
        <div key={i} className="flex flex-col overflow-hidden rounded-lg border border-line bg-surface">
          <Skeleton className="aspect-[3/4] rounded-none" />
          <div className="flex flex-col gap-2 p-3">
            <Skeleton className="h-4 w-3/4" />
            <Skeleton className="mt-1 h-5 w-1/3" />
          </div>
        </div>
```

- [ ] **Step 3: Lay the page out as rail + grid**

Replace the `CatalogPage` component body's state declaration and returned JSX:

```tsx
export default function CatalogPage() {
  const { t } = useTranslation();
  const [category, setCategory] = useState<CatalogCategory>("all");
  const type = category === "all" ? undefined : category;
  const { data, isLoading, isError, error, refetch } = useProducts(type);

  return (
    <>
      <header className="mb-6">
        <h1 className="font-serif text-3xl font-bold text-ink-900 md:text-4xl">{t("nav_store")}</h1>
        <p className="mt-1 text-sm text-ink-500">{t("catalog_subtitle")}</p>
      </header>

      <div className="flex flex-col gap-4 md:flex-row md:gap-6">
        <CategoryRail value={category} onChange={setCategory} />

        <div className="min-w-0 flex-1">
          {isError ? (
            <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
              <p>{t("catalog_load_failed")} {(error as Error)?.message}</p>
              <button onClick={() => refetch()} className="mt-2 underline">
                {t("retry")}
              </button>
            </div>
          ) : isLoading ? (
            <CatalogSkeleton />
          ) : (
            <CatalogGrid products={data} tab={category} />
          )}
        </div>
      </div>
    </>
  );
}
```

`min-w-0` on the grid column is required — without it the grid refuses to shrink below its content width and the rail gets pushed off-screen on narrow viewports.

- [ ] **Step 4: Verify the page compiles and the suite is green**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

Expected: no type errors; all tests pass.

- [ ] **Step 5: Commit**

```bash
git add web/app/\(student\)/catalog/page.tsx
git commit -m "feat: catalog uses category rail and a five-column grid"
```

---

### Task 4: Follow the product cursor so the catalog isn't silently truncated

**Files:**
- Modify: `web/lib/hooks/products.ts`
- Test: `web/lib/hooks/products.test.tsx` (create)

**Interfaces:**
- Produces: `useProducts(type?)` — unchanged signature, still returns `Product[]`. It now aggregates every page instead of only the first 20 rows.

- [ ] **Step 1: Write the failing test**

Create `web/lib/hooks/products.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useProducts } from "./products";
import * as api from "@/lib/api";

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

afterEach(() => vi.restoreAllMocks());

describe("useProducts", () => {
  it("follows next_cursor so products past the first page are not dropped", async () => {
    const fetchSpy = vi.spyOn(api, "apiFetch").mockImplementation(async (path: string) => {
      if (path.includes("cursor=p2")) {
        return { data: [{ id: "b", type: "medal", name: "Medali", price: 1 }] };
      }
      return {
        data: [{ id: "a", type: "book", name: "Buku", price: 1 }],
        next_cursor: "p2",
      };
    });

    const { result } = renderHook(() => useProducts(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((p) => p.id)).toEqual(["a", "b"]);
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it("stops after ten pages so a bad cursor cannot loop forever", async () => {
    const fetchSpy = vi.spyOn(api, "apiFetch").mockResolvedValue({
      data: [{ id: "x", type: "book", name: "Buku", price: 1 }],
      next_cursor: "always",
    } as any);

    const { result } = renderHook(() => useProducts(), { wrapper });

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(fetchSpy).toHaveBeenCalledTimes(10);
  });
});
```

- [ ] **Step 2: Run it and watch it fail**

```bash
cd web && npx vitest run lib/hooks/products.test.tsx
```

Expected: FAIL — first test gets `["a"]`, and `apiFetch` was called once.

- [ ] **Step 3: Implement cursor following**

Replace `useProducts` in `web/lib/hooks/products.ts`:

```ts
const MAX_PRODUCT_PAGES = 10;

export function useProducts(type?: ProductType) {
  return useQuery({
    queryKey: productsKeys.list(type),
    queryFn: async () => {
      const all: Product[] = [];
      let cursor: string | undefined;

      for (let page = 0; page < MAX_PRODUCT_PAGES; page++) {
        const params = new URLSearchParams();
        if (type) params.set("type", type);
        if (cursor) params.set("cursor", cursor);
        const qs = params.toString() ? `?${params.toString()}` : "";

        const res = await apiFetch<{ data: Product[]; next_cursor?: string }>(`/products${qs}`);
        all.push(...(res.data ?? []));

        if (!res.next_cursor) break;
        cursor = res.next_cursor;
      }

      return all;
    },
  });
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
cd web && npx vitest run lib/hooks/products.test.tsx
```

Expected: PASS, 2 tests.

- [ ] **Step 5: Bind the cursor param in the handler (verified missing on this base)**

`ProductFilter` already has a `Cursor` field (`backend/internal/repository/product.go:44`) and `ListProducts` in the repo uses it, but the public `ListProducts` handler never reads the query param — so without this step the cursor loop from Step 3 always gets the same first page and the second test would loop to its 10-page cap in production. Add `Cursor` to the `filter` literal in `backend/internal/handler/product.go` `ListProducts` (the struct currently sets only `Type` and `Status`):

```go
	filter := repository.ProductFilter{
		Type:   c.QueryParam("type"),
		Status: c.QueryParam("status"),
		Cursor: c.QueryParam("cursor"),
	}
```

Confirm it took:

```bash
grep -n 'Cursor: c.QueryParam' backend/internal/handler/product.go
```

Expected: one match.

- [ ] **Step 6: Commit**

```bash
git add web/lib/hooks/products.ts web/lib/hooks/products.test.tsx backend/internal/handler/product.go
git commit -m "fix: follow product cursor so the catalog is not cut off at 20 items"
```

---

# Slice 2 — Product specifications

### Task 5: Migration, model and repository

**Files:**
- Create: `backend/db/migrations/0045_product_specs.up.sql`, `backend/db/migrations/0045_product_specs.down.sql`
- Modify: `backend/internal/model/product.go`, `backend/internal/repository/product.go`

**Interfaces:**
- Produces:
  ```go
  type ProductSpec struct {
      Key   string `json:"key"`
      Label string `json:"label"`
      Value string `json:"value"`
  }
  // model.Product gains: Specs []ProductSpec `json:"specs"`
  ```
  Tasks 6 and 7 depend on these exact names.

- [ ] **Step 1: Write the migration**

Create `backend/db/migrations/0045_product_specs.up.sql`:

```sql
-- 0045_product_specs.up.sql
-- Free-form product specification rows (publisher, cover type, material, ...)
-- stored as an ordered JSON array so display order survives round-trips. The
-- canonical field list per product type lives in the frontend, not here; the
-- backend only bounds the shape.
--
-- Numbered 0045 deliberately: base is 0036 and unmerged PR #44 holds 0037-0044,
-- so any lower number collides depending on merge order.

ALTER TABLE product ADD COLUMN specs JSONB NOT NULL DEFAULT '[]'::jsonb;
```

Create `backend/db/migrations/0045_product_specs.down.sql`:

```sql
-- 0045_product_specs.down.sql
-- Additive change; dropping loses every specification captured going forward.
ALTER TABLE product DROP COLUMN IF EXISTS specs;
```

- [ ] **Step 2: Add the model type**

In `backend/internal/model/product.go`, add above `type Product struct`:

```go
// ProductSpec is one row of the product specification table shown on the
// storefront. Label travels with the value so rendering never needs to look up
// a field catalogue, and so operator-added custom rows render like any other.
type ProductSpec struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Value string `json:"value"`
}
```

Then add this field to `Product`, immediately after `ImageURL`:

```go
	Specs          []ProductSpec `json:"specs"`
```

- [ ] **Step 3: Thread it through the repository**

In `backend/internal/repository/product.go`:

Update `scanProduct` to scan the new column. Replace the whole function:

```go
func scanProduct(row interface{ Scan(dest ...any) error }, p *model.Product) error {
	var description, imageURL *string
	var weightGrams *int
	var specs []byte
	err := row.Scan(
		&p.ID, &p.Type, &p.Name, &description, &p.Price, &p.Stock, &p.Status,
		&weightGrams, &imageURL, &specs, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if description != nil {
		p.Description = *description
	}
	if imageURL != nil {
		p.ImageURL = *imageURL
	}
	if weightGrams != nil {
		p.WeightGrams = *weightGrams
	}
	p.Specs = []model.ProductSpec{}
	if len(specs) > 0 {
		if err := json.Unmarshal(specs, &p.Specs); err != nil {
			return err
		}
	}
	return nil
}
```

Add `"encoding/json"` to the import block if it is not already there.

Add `specs` to every `SELECT` that feeds `scanProduct`. There are three column lists to update — in `ListProducts`, `GetProductByID`, and any other `SELECT ... FROM product` that calls `scanProduct`. Find them:

```bash
grep -n "weight_grams, image_url, created_at" backend/internal/repository/product.go
```

In each match, replace `weight_grams, image_url, created_at, updated_at` with `weight_grams, image_url, specs, created_at, updated_at`.

Update `CreateProduct`:

```go
func (r *Repository) CreateProduct(ctx context.Context, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO product (type, name, description, price, stock, status, weight_grams, image_url, specs)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return err
}

// marshalSpecs renders specs as a JSON array, never SQL NULL — the column is
// NOT NULL and a nil slice would marshal to "null".
func marshalSpecs(specs []model.ProductSpec) ([]byte, error) {
	if specs == nil {
		specs = []model.ProductSpec{}
	}
	return json.Marshal(specs)
}
```

Update both `UpdateProduct` and `UpdateProductTx` the same way — add `specs = $9` to the `SET` clause, shift `WHERE id` to `$10`, and pass the marshalled value:

```go
func (r *Repository) UpdateProduct(ctx context.Context, id string, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE product
		SET type = $1, name = $2, description = $3, price = $4, stock = $5, status = $6, weight_grams = $7, image_url = $8, specs = $9, updated_at = now()
		WHERE id = $10`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, id,
	)
	return err
}

func (r *Repository) UpdateProductTx(ctx context.Context, tx pgx.Tx, id string, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE product
		SET type = $1, name = $2, description = $3, price = $4, stock = $5, status = $6, weight_grams = $7, image_url = $8, specs = $9, updated_at = now()
		WHERE id = $10`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, id,
	)
	return err
}
```

- [ ] **Step 4: Verify it compiles**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go build ./... && go vet ./...
```

Expected: no output.

- [ ] **Step 5: Commit**

```bash
git add backend/db/migrations/0045_product_specs.up.sql backend/db/migrations/0045_product_specs.down.sql backend/internal/model/product.go backend/internal/repository/product.go
git commit -m "feat: add product.specs JSONB column and thread it through the repository"
```

---

### Task 6: Shape validation and handler wiring

**Files:**
- Create: `backend/internal/service/product_specs.go`, `backend/internal/service/product_specs_test.go`
- Modify: `backend/internal/handler/product.go`, `backend/internal/handler/errors.go`

**Interfaces:**
- Consumes: `model.ProductSpec` from Task 5
- Produces:
  ```go
  var ErrInvalidSpecs = errors.New("invalid product specs")
  func ValidateSpecs(specs []model.ProductSpec) error
  ```
  Task 6's handler calls `ValidateSpecs`; nothing else consumes it.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/service/product_specs_test.go`:

```go
package service

import (
	"errors"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestValidateSpecs(t *testing.T) {
	long := strings.Repeat("a", 501)

	tests := []struct {
		name    string
		specs   []model.ProductSpec
		wantErr bool
	}{
		{"nil is allowed", nil, false},
		{"empty is allowed", []model.ProductSpec{}, false},
		{
			"well-formed row",
			[]model.ProductSpec{{Key: "penerbit", Label: "Penerbit", Value: "Yayasan Abak Cendekia"}},
			false,
		},
		{"missing key", []model.ProductSpec{{Label: "Penerbit", Value: "x"}}, true},
		{"missing label", []model.ProductSpec{{Key: "penerbit", Value: "x"}}, true},
		{
			"key over 100 chars",
			[]model.ProductSpec{{Key: strings.Repeat("k", 101), Label: "L", Value: "v"}},
			true,
		},
		{
			"label over 100 chars",
			[]model.ProductSpec{{Key: "k", Label: strings.Repeat("l", 101), Value: "v"}},
			true,
		},
		{"value over 500 chars", []model.ProductSpec{{Key: "k", Label: "L", Value: long}}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpecs(tc.specs)
			if tc.wantErr && !errors.Is(err, ErrInvalidSpecs) {
				t.Fatalf("want ErrInvalidSpecs, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

func TestValidateSpecs_RejectsMoreThanThirtyRows(t *testing.T) {
	specs := make([]model.ProductSpec, 31)
	for i := range specs {
		specs[i] = model.ProductSpec{Key: "k", Label: "L", Value: "v"}
	}
	if err := ValidateSpecs(specs); !errors.Is(err, ErrInvalidSpecs) {
		t.Fatalf("want ErrInvalidSpecs for 31 rows, got %v", err)
	}

	if err := ValidateSpecs(specs[:30]); err != nil {
		t.Fatalf("30 rows should be accepted, got %v", err)
	}
}

// An empty value is legal — the frontend renders the canonical field list with
// blank rows the operator has not filled in yet, and skips them at display time.
func TestValidateSpecs_AllowsEmptyValue(t *testing.T) {
	if err := ValidateSpecs([]model.ProductSpec{{Key: "isbn", Label: "ISBN", Value: ""}}); err != nil {
		t.Fatalf("empty value should be accepted, got %v", err)
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestValidateSpecs -v
```

Expected: FAIL — `undefined: ValidateSpecs`.

- [ ] **Step 3: Write the validator**

Create `backend/internal/service/product_specs.go`:

```go
package service

import (
	"errors"
	"fmt"

	"akademi-bimbel/internal/model"
)

// ErrInvalidSpecs is returned when a product specification array is malformed.
var ErrInvalidSpecs = errors.New("invalid product specs")

const (
	maxSpecRows      = 30
	maxSpecKeyLen    = 100
	maxSpecLabelLen  = 100
	maxSpecValueLen  = 500
)

// ValidateSpecs bounds the shape of the specs array. It deliberately knows
// nothing about which fields belong to which product type — that catalogue
// lives in the frontend. These limits are what keep a free-form JSONB column
// from growing without bound.
func ValidateSpecs(specs []model.ProductSpec) error {
	if len(specs) > maxSpecRows {
		return fmt.Errorf("%w: at most %d rows allowed, got %d", ErrInvalidSpecs, maxSpecRows, len(specs))
	}
	for i, s := range specs {
		if s.Key == "" {
			return fmt.Errorf("%w: row %d has an empty key", ErrInvalidSpecs, i)
		}
		if s.Label == "" {
			return fmt.Errorf("%w: row %d has an empty label", ErrInvalidSpecs, i)
		}
		if len(s.Key) > maxSpecKeyLen {
			return fmt.Errorf("%w: row %d key exceeds %d characters", ErrInvalidSpecs, i, maxSpecKeyLen)
		}
		if len(s.Label) > maxSpecLabelLen {
			return fmt.Errorf("%w: row %d label exceeds %d characters", ErrInvalidSpecs, i, maxSpecLabelLen)
		}
		if len(s.Value) > maxSpecValueLen {
			return fmt.Errorf("%w: row %d value exceeds %d characters", ErrInvalidSpecs, i, maxSpecValueLen)
		}
	}
	return nil
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestValidateSpecs -v
```

Expected: PASS.

- [ ] **Step 5: Accept specs in the handlers**

In `backend/internal/handler/product.go`, `AdminCreateProduct`: add to the anonymous request struct, after `ImageURL`:

```go
		Specs       []model.ProductSpec `json:"specs"`
```

After the `if req.Type == "" || req.Name == ""` guard, add:

```go
	if err := service.ValidateSpecs(req.Specs); err != nil {
		return mapServiceError(c, err)
	}
```

Add `Specs: req.Specs,` to the `model.Product{...}` literal.

In `AdminUpdateProduct`: add to the request struct, after `ImageURL`:

```go
		Specs       *[]model.ProductSpec `json:"specs"`
```

A pointer so an omitted field preserves existing specs instead of clearing them. After the `c.Bind` guard, add:

```go
	if req.Specs != nil {
		if err := service.ValidateSpecs(*req.Specs); err != nil {
			return mapServiceError(c, err)
		}
	}
```

Then, wherever the handler builds the `model.Product` it passes to the service, set specs from either the request or the existing product. Locate the `p := model.Product{` literal in `AdminUpdateProduct` and add after it:

```go
	if req.Specs != nil {
		p.Specs = *req.Specs
	} else {
		existing, err := h.svc.GetProduct(c.Request().Context(), id, role)
		if err != nil {
			return mapServiceError(c, err)
		}
		p.Specs = existing.Specs
	}
```

Ensure `"akademi-bimbel/internal/model"` and `"akademi-bimbel/internal/service"` are imported.

- [ ] **Step 6: Map the error to a 400**

In `backend/internal/handler/errors.go`, inside the error-mapping switch, add a case alongside the other validation errors:

```go
	case errors.Is(err, service.ErrInvalidSpecs):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_specs", Message: err.Error()}
```

Find the surrounding switch first so the case lands in the right block:

```bash
grep -n "case errors.Is(err, service.Err" backend/internal/handler/errors.go | head -5
```

- [ ] **Step 7: Verify build and full backend suite**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go build ./... && go vet ./... && go test ./...
```

Expected: all packages pass.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/service/product_specs.go backend/internal/service/product_specs_test.go backend/internal/handler/product.go backend/internal/handler/errors.go
git commit -m "feat: validate and accept product specs on the admin product endpoints"
```

---

### Task 7: DB-backed round-trip test for specs

**Files:**
- Create: `backend/internal/handler/product_specs_handler_test.go`

**Interfaces:**
- Consumes: the testcontainers harness pattern in `backend/internal/handler/product_merch_handler_test.go`

This task exists because `shimService` reimplements product methods in `store_test.go` — a unit test there would prove nothing about the real code path. This test drives the real handler, real service, real repository, and real migrations.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/handler/product_specs_handler_test.go`. It reuses `newAdminProductDBEnv` and `mintProductToken` from `product_merch_handler_test.go` — same package, no import needed.

```go
package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestAdminProduct_SpecsRoundTrip(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a1", "super_admin")

	body := `{
		"type": "book",
		"name": "Kumpulan Soal KoSSMI Fisika",
		"price": 20000,
		"stock": 9,
		"specs": [
			{"key": "penerbit", "label": "Perusahaan Penerbit", "value": "Yayasan Abak Cendekia"},
			{"key": "jenis_cover", "label": "Jenis Cover", "value": "Hard Cover"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		t.Fatalf("create product: status %d body %s", rec.Code, rec.Body.String())
	}

	var created model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created product: %v", err)
	}
	if len(created.Specs) != 2 {
		t.Fatalf("want 2 specs persisted, got %d (%+v)", len(created.Specs), created.Specs)
	}
	if created.Specs[0].Key != "penerbit" || created.Specs[1].Label != "Jenis Cover" {
		t.Fatalf("spec order or content not preserved: %+v", created.Specs)
	}
}

func TestAdminProduct_OmittedSpecsPreserveExisting(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a2", "super_admin")

	createBody := `{
		"type": "book", "name": "Buku Spesifikasi", "price": 1000, "stock": 1,
		"specs": [{"key": "isbn", "label": "ISBN", "value": "978-1"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(createBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	var created model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// PATCH without a specs field must not wipe the stored specs.
	patch := `{"name": "Buku Spesifikasi v2", "price": 2000, "stock": 1}`
	req = httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+created.ID, strings.NewReader(patch))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch product: status %d body %s", rec.Code, rec.Body.String())
	}

	var updated model.Product
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if len(updated.Specs) != 1 || updated.Specs[0].Key != "isbn" {
		t.Fatalf("omitted specs should be preserved, got %+v", updated.Specs)
	}
}

func TestAdminProduct_RejectsMalformedSpecs(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000a3", "super_admin")

	body := `{
		"type": "book", "name": "Buku Rusak", "price": 1000, "stock": 1,
		"specs": [{"key": "", "label": "Tanpa Key", "value": "x"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for malformed specs, got %d body %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run it**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/handler/ -run TestAdminProduct_ -v
```

Expected: PASS. Requires Docker running for testcontainers. If Task 5 or 6 missed a `SELECT` column, these tests fail with a scan-count mismatch — that is the intended safety net.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/product_specs_handler_test.go
git commit -m "test: DB-backed round trip for product specs against real migrations"
```

---

### Task 8: Canonical field list and admin specs editor

**Files:**
- Create: `web/lib/product-specs.ts`, `web/components/admin/ProductSpecsEditor.tsx`, `web/components/admin/ProductSpecsEditor.test.tsx`
- Modify: `web/components/admin/ProductModal.tsx`, `web/lib/types.ts`

**Interfaces:**
- Produces:
  ```ts
  export interface ProductSpec { key: string; label: string; value: string }   // web/lib/types.ts
  export const SPEC_FIELDS: Record<ProductType, { key: string; label: string }[]>;  // web/lib/product-specs.ts
  export function specRowsForType(type: ProductType, saved: ProductSpec[]): ProductSpec[];
  export function ProductSpecsEditor(props: {
    type: ProductType;
    value: ProductSpec[];
    onChange: (specs: ProductSpec[]) => void;
  }): JSX.Element;
  ```
  Task 9 consumes `ProductSpec` from `web/lib/types.ts`.

- [ ] **Step 1: Add the shared type**

In `web/lib/types.ts`, above `export interface Product`:

```ts
export interface ProductSpec {
  key: string;
  label: string;
  value: string;
}
```

And add to the `Product` interface, after `image_url`:

```ts
  specs?: ProductSpec[];
```

Add the same field to `AdminCreateProductInput`:

```ts
  specs?: ProductSpec[];
```

- [ ] **Step 2: Write the failing test for the field catalogue and editor**

Create `web/components/admin/ProductSpecsEditor.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProductSpecsEditor } from "./ProductSpecsEditor";
import { specRowsForType, SPEC_FIELDS } from "@/lib/product-specs";

describe("specRowsForType", () => {
  it("offers the canonical book fields when nothing is saved yet", () => {
    const rows = specRowsForType("book", []);
    expect(rows.map((r) => r.key)).toEqual(SPEC_FIELDS.book.map((f) => f.key));
    expect(rows.every((r) => r.value === "")).toBe(true);
  });

  it("keeps saved values and appends custom rows after the canonical ones", () => {
    const rows = specRowsForType("book", [
      { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
      { key: "berat_buku", label: "Berat Buku", value: "300 g" },
    ]);

    const penerbit = rows.find((r) => r.key === "penerbit");
    expect(penerbit?.value).toBe("Yayasan Abak Cendekia");
    expect(rows[rows.length - 1].key).toBe("berat_buku");
  });

  it("gives exam and course no canonical fields", () => {
    expect(specRowsForType("exam", [])).toEqual([]);
    expect(specRowsForType("course", [])).toEqual([]);
  });
});

describe("ProductSpecsEditor", () => {
  it("emits every row including blanks so the parent can drop them at save time", () => {
    const onChange = vi.fn();
    render(<ProductSpecsEditor type="book" value={[]} onChange={onChange} />);

    const inputs = screen.getAllByPlaceholderText("Nilai");
    fireEvent.change(inputs[0], { target: { value: "Yayasan Abak Cendekia" } });

    expect(onChange).toHaveBeenCalled();
    const emitted = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(emitted[0]).toMatchObject({ key: "penerbit", value: "Yayasan Abak Cendekia" });
  });

  it("adds a custom row on demand", () => {
    const onChange = vi.fn();
    render(<ProductSpecsEditor type="medal" value={[]} onChange={onChange} />);

    const before = screen.getAllByPlaceholderText("Nilai").length;
    fireEvent.click(screen.getByRole("button", { name: /tambah baris/i }));
    expect(screen.getAllByPlaceholderText("Nilai").length).toBe(before + 1);
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/admin/ProductSpecsEditor.test.tsx
```

Expected: FAIL — unresolved imports.

- [ ] **Step 4: Write the field catalogue**

Create `web/lib/product-specs.ts`:

```ts
import type { ProductSpec, ProductType } from "@/lib/types";

// Canonical specification fields per product type. This catalogue lives in the
// frontend on purpose: the backend stores whatever it is given and only bounds
// the shape. Keys are canonical from day one so a future facet filter needs
// value normalisation, not a key clean-up across every product.
export const SPEC_FIELDS: Record<ProductType, { key: string; label: string }[]> = {
  book: [
    { key: "penerbit", label: "Perusahaan Penerbit" },
    { key: "tahun_terbit", label: "Tahun Terbit" },
    { key: "bahasa", label: "Bahasa" },
    { key: "jenis_cover", label: "Jenis Cover" },
    { key: "jenis_edisi", label: "Jenis Edisi" },
    { key: "jumlah_halaman", label: "Jumlah Halaman" },
    { key: "isbn", label: "ISBN" },
    { key: "impor_lokal", label: "Impor/Lokal" },
  ],
  merchandise: [
    { key: "bahan", label: "Bahan" },
    { key: "ukuran", label: "Ukuran" },
    { key: "warna", label: "Warna" },
    { key: "isi_paket", label: "Isi Paket" },
  ],
  medal: [
    { key: "bahan", label: "Bahan" },
    { key: "diameter", label: "Diameter" },
    { key: "finishing", label: "Finishing" },
    { key: "kemasan", label: "Kemasan" },
  ],
  course: [],
  exam: [],
};

// specRowsForType merges the canonical field list with whatever is already
// saved: canonical fields first in catalogue order carrying any saved value,
// then operator-added custom rows in their stored order.
export function specRowsForType(type: ProductType, saved: ProductSpec[]): ProductSpec[] {
  const canonical = SPEC_FIELDS[type] ?? [];
  const canonicalKeys = new Set(canonical.map((f) => f.key));

  const rows: ProductSpec[] = canonical.map((f) => {
    const hit = saved.find((s) => s.key === f.key);
    return { key: f.key, label: f.label, value: hit?.value ?? "" };
  });

  for (const s of saved) {
    if (!canonicalKeys.has(s.key)) rows.push({ ...s });
  }

  return rows;
}
```

- [ ] **Step 5: Write the editor**

Create `web/components/admin/ProductSpecsEditor.tsx`:

```tsx
"use client";

import { useEffect, useState } from "react";
import { Plus, Trash2 } from "lucide-react";
import type { ProductSpec, ProductType } from "@/lib/types";
import { specRowsForType } from "@/lib/product-specs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export interface ProductSpecsEditorProps {
  type: ProductType;
  value: ProductSpec[];
  onChange: (specs: ProductSpec[]) => void;
}

export function ProductSpecsEditor({ type, value, onChange }: ProductSpecsEditorProps) {
  const [rows, setRows] = useState<ProductSpec[]>(() => specRowsForType(type, value));

  useEffect(() => {
    setRows(specRowsForType(type, value));
    // Re-seed when the product type changes so the canonical field list follows it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [type]);

  const emit = (next: ProductSpec[]) => {
    setRows(next);
    onChange(next);
  };

  const setAt = (i: number, patch: Partial<ProductSpec>) =>
    emit(rows.map((r, idx) => (idx === i ? { ...r, ...patch } : r)));

  return (
    <div className="flex flex-col gap-2">
      <Label>Spesifikasi Produk</Label>
      {rows.map((row, i) => (
        <div key={`${row.key}-${i}`} className="flex items-center gap-2">
          <Input
            aria-label={`Label baris ${i + 1}`}
            placeholder="Label"
            value={row.label}
            onChange={(e) => setAt(i, { label: e.target.value })}
            className="w-2/5"
          />
          <Input
            aria-label={`Nilai baris ${i + 1}`}
            placeholder="Nilai"
            value={row.value}
            onChange={(e) => setAt(i, { value: e.target.value })}
            className="flex-1"
          />
          <button
            type="button"
            aria-label={`Hapus baris ${i + 1}`}
            onClick={() => emit(rows.filter((_, idx) => idx !== i))}
            className="text-ink-400 hover:text-danger"
          >
            <Trash2 className="size-4" />
          </button>
        </div>
      ))}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="self-start"
        onClick={() =>
          emit([...rows, { key: `custom_${rows.length + 1}`, label: "", value: "" }])
        }
      >
        <Plus className="mr-1 size-4" />
        Tambah baris
      </Button>
    </div>
  );
}
```

- [ ] **Step 6: Run the tests and watch them pass**

```bash
cd web && npx vitest run components/admin/ProductSpecsEditor.test.tsx
```

Expected: PASS, 5 tests.

- [ ] **Step 7: Mount the editor in `ProductModal`**

In `web/components/admin/ProductModal.tsx`:

Add the import:

```tsx
import { ProductSpecsEditor } from "@/components/admin/ProductSpecsEditor";
import type { ProductSpec } from "@/lib/types";
```

Add state next to the other field state (near `const [stock, setStock] = useState("")`):

```tsx
  const [specs, setSpecs] = useState<ProductSpec[]>([]);
```

In the effect that hydrates from an existing product (where `setStock(...)` and `setImageUrl(...)` are called), add:

```tsx
        setSpecs(product.specs ?? []);
```

Render the editor immediately before the description field's `<Label htmlFor="product-description">` block:

```tsx
            <ProductSpecsEditor type={type as ProductType} value={specs} onChange={setSpecs} />
```

In both payload builders (create and update — the two places that spread `...(showStock && imageUrl !== "" ...)`), add outside the `showStock` guard:

```tsx
        specs: specs.filter((s) => s.label.trim() !== "" && s.value.trim() !== ""),
```

Rows with a blank label or value are dropped at save time, so an untouched canonical field never persists as noise.

- [ ] **Step 8: Verify types and the full frontend suite**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

Expected: no type errors; all tests pass.

- [ ] **Step 9: Commit**

```bash
git add web/lib/product-specs.ts web/lib/types.ts web/components/admin/ProductSpecsEditor.tsx web/components/admin/ProductSpecsEditor.test.tsx web/components/admin/ProductModal.tsx
git commit -m "feat: admin product specs editor driven by a canonical field list"
```

---

### Task 9: Specification table on the product detail page

**Files:**
- Create: `web/components/catalog/ProductSpecTable.tsx`, `web/components/catalog/ProductSpecTable.test.tsx`
- Modify: `web/app/(student)/catalog/[id]/page.tsx`, `web/lib/i18n.ts`

**Interfaces:**
- Consumes: `ProductSpec` from `web/lib/types.ts` (Task 8)
- Produces: `ProductSpecTable({ specs }: { specs?: ProductSpec[] })` — renders nothing when there is no displayable row.

- [ ] **Step 1: Add the i18n key**

In `web/lib/i18n.ts`, `id` block, after `product_no_description:`:

```ts
    product_specs_heading: "Spesifikasi Produk",
```

`en` block, same position:

```ts
    product_specs_heading: "Product Specifications",
```

- [ ] **Step 2: Write the failing test**

Create `web/components/catalog/ProductSpecTable.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProductSpecTable } from "./ProductSpecTable";

describe("ProductSpecTable", () => {
  it("renders label and value pairs", () => {
    render(
      <ProductSpecTable
        specs={[
          { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
          { key: "jenis_cover", label: "Jenis Cover", value: "Hard Cover" },
        ]}
      />,
    );

    expect(screen.getByText("Perusahaan Penerbit")).toBeTruthy();
    expect(screen.getByText("Yayasan Abak Cendekia")).toBeTruthy();
    expect(screen.getByText("Hard Cover")).toBeTruthy();
  });

  it("skips rows with a blank value", () => {
    render(
      <ProductSpecTable
        specs={[
          { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
          { key: "isbn", label: "ISBN", value: "" },
        ]}
      />,
    );

    expect(screen.queryByText("ISBN")).toBeNull();
  });

  it("renders nothing at all when there is no displayable row", () => {
    const { container } = render(
      <ProductSpecTable specs={[{ key: "isbn", label: "ISBN", value: "" }]} />,
    );
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing when specs are absent", () => {
    const { container } = render(<ProductSpecTable />);
    expect(container.firstChild).toBeNull();
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/catalog/ProductSpecTable.test.tsx
```

Expected: FAIL — unresolved import.

- [ ] **Step 4: Write the component**

Create `web/components/catalog/ProductSpecTable.tsx`:

```tsx
"use client";

import type { ProductSpec } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";

export interface ProductSpecTableProps {
  specs?: ProductSpec[];
}

export function ProductSpecTable({ specs }: ProductSpecTableProps) {
  const { t } = useTranslation();
  const rows = (specs ?? []).filter((s) => s.value.trim() !== "");
  if (rows.length === 0) return null;

  return (
    <section className="rounded-lg border border-line bg-surface p-5">
      <h2 className="mb-4 font-serif text-lg font-semibold text-ink-900">
        {t("product_specs_heading" as any)}
      </h2>
      <dl className="flex flex-col gap-3 text-sm">
        {rows.map((s, i) => (
          <div key={`${s.key}-${i}`} className="grid grid-cols-[minmax(0,180px)_1fr] gap-4">
            <dt className="text-ink-500">{s.label}</dt>
            <dd className="text-ink-900">{s.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
```

- [ ] **Step 5: Run it and watch it pass**

```bash
cd web && npx vitest run components/catalog/ProductSpecTable.test.tsx
```

Expected: PASS, 4 tests.

- [ ] **Step 6: Mount it on the detail page**

In `web/app/(student)/catalog/[id]/page.tsx`, add the import:

```tsx
import { ProductSpecTable } from "@/components/catalog/ProductSpecTable";
```

Insert it immediately after the closing `</div>` of the block that renders the description and stock line, still inside the left-hand `<div className="flex flex-col gap-6">`:

```tsx
          <ProductSpecTable specs={product.specs} />
```

- [ ] **Step 7: Verify and commit**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

```bash
git add web/components/catalog/ProductSpecTable.tsx web/components/catalog/ProductSpecTable.test.tsx web/app/\(student\)/catalog/\[id\]/page.tsx web/lib/i18n.ts
git commit -m "feat: show product specifications on the detail page"
```

---

# Slice 3 — Digital quantity guard

### Task 10: Backend quantity rule

**Files:**
- Create: `backend/internal/service/item_qty.go`, `backend/internal/service/item_qty_test.go`
- Modify: `backend/internal/service/store.go`, `backend/internal/handler/errors.go`

**Interfaces:**
- Produces:
  ```go
  var ErrDigitalQtyLimit = errors.New("digital products are limited to one per order")
  var ErrInvalidQty = errors.New("invalid quantity")
  func ValidateItemQty(productType string, qty int) error
  ```
  Nothing downstream consumes these beyond `store.go` and `errors.go`. There is no
  generic `ErrValidation` sentinel in this codebase (verified) — `ValidateItemQty`
  defines its own `ErrInvalidQty` for the qty-below-one case, matching the existing
  `ErrInvalid*` naming pattern.

**Why this is a pure function:** `Service.AddItem` depends on `*repository.Repository` (a concrete type needing a pgx pool), so it cannot be unit-tested directly, and the existing `shimOrderService.AddItem` in `store_test.go:836` is a reimplementation that would happily pass while the real code stayed broken. The rule therefore lives in a pure function that both the real service and the shim call.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/service/item_qty_test.go`:

```go
package service

import (
	"errors"
	"testing"
)

func TestValidateItemQty(t *testing.T) {
	tests := []struct {
		name        string
		productType string
		qty         int
		wantErr     error
	}{
		{"exam qty 1 is fine", "exam", 1, nil},
		{"course qty 1 is fine", "course", 1, nil},
		{"exam qty 2 is rejected", "exam", 2, ErrDigitalQtyLimit},
		{"course qty 5 is rejected", "course", 5, ErrDigitalQtyLimit},
		{"book qty 3 is fine", "book", 3, nil},
		{"merchandise qty 10 is fine", "merchandise", 10, nil},
		{"medal qty 4 is fine", "medal", 4, nil},
		{"zero qty is rejected for any type", "book", 0, ErrInvalidQty},
		{"negative qty is rejected for any type", "exam", -1, ErrInvalidQty},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateItemQty(tc.productType, tc.qty)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want %v, got %v", tc.wantErr, err)
			}
		})
	}
}
```

- [ ] **Step 2: Run the test and watch it fail**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestValidateItemQty -v
```

Expected: FAIL — `undefined: ValidateItemQty`.

- [ ] **Step 3: Write the rule**

Create `backend/internal/service/item_qty.go`:

```go
package service

import (
	"errors"
	"fmt"
)

// ErrDigitalQtyLimit is returned when a caller tries to put more than one unit
// of a digital product in an order.
var ErrDigitalQtyLimit = errors.New("digital products are limited to one per order")

// ErrInvalidQty is returned for a non-positive quantity. There is no generic
// validation sentinel in this package, so the rule owns its own.
var ErrInvalidQty = errors.New("invalid quantity")

// ValidateItemQty enforces the per-line quantity rule.
//
// Digital products are capped at one because fulfilment ignores qty entirely:
// the outbox worker creates a single exam registration or course enrolment per
// order item regardless of quantity, so qty 3 charged the buyer three times and
// delivered one. Capping here is the fix; the worker is deliberately untouched.
//
// admin_school multi-seat purchases do not pass through this path — they go via
// CreateBulkExamOrder, which fans out through order_participants.
func ValidateItemQty(productType string, qty int) error {
	if qty < 1 {
		return fmt.Errorf("%w: qty must be at least 1", ErrInvalidQty)
	}
	if !isPhysicalType(productType) && qty > 1 {
		return fmt.Errorf("%w: %s accepts qty 1, got %d", ErrDigitalQtyLimit, productType, qty)
	}
	return nil
}
```

- [ ] **Step 4: Run it and watch it pass**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run TestValidateItemQty -v
```

Expected: PASS, 9 subtests.

- [ ] **Step 5: Call it from the real service**

In `backend/internal/service/store.go`, inside `AddItem`, immediately after the out-of-stock guard:

```go
	if isPhysicalType(product.Type) && product.Stock == 0 {
		return ErrOutOfStock
	}
	if err := ValidateItemQty(product.Type, qty); err != nil {
		return err
	}
```

In `UpdateItemQty`, replace the opening guard:

```go
func (s *Service) UpdateItemQty(ctx context.Context, studentID, orderID, itemID string, qty int) error {
	if qty < 1 {
		return errors.New("qty must be at least 1")
	}
```

with a lookup of the item's product type followed by the shared rule. Insert after the order is loaded and ownership verified (mirror the existing `RemoveItem` structure, which already iterates `order.Items` to find the matching item):

```go
	for _, item := range order.Items {
		if item.ID == iID {
			if err := ValidateItemQty(item.ProductType, qty); err != nil {
				return err
			}
			break
		}
	}
```

Read the current body first so the insertion lands after `iID` is parsed and the order is fetched:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && sed -n '/func (s \*Service) UpdateItemQty/,/^}/p' internal/service/store.go
```

- [ ] **Step 6: Keep the shim honest**

In `backend/internal/service/store_test.go`, `shimOrderService.AddItem` (around line 836) and `shimOrderService.UpdateItemQty` (around line 912) must call the same rule, so the shim cannot drift from real behaviour. Add to each, at the point where the product type is known:

```go
	if err := ValidateItemQty(product.Type, qty); err != nil {
		return err
	}
```

Inspect both first — the variable holding the product may be named differently:

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && sed -n '830,930p' internal/service/store_test.go
```

- [ ] **Step 7: Map the error to a 400**

In `backend/internal/handler/errors.go`, alongside the case added in Task 6:

```go
	case errors.Is(err, service.ErrDigitalQtyLimit):
		status, apiErr = http.StatusBadRequest, APIError{Code: "invalid_qty", Message: err.Error()}
```

- [ ] **Step 8: Run the full backend suite**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go build ./... && go vet ./... && go test ./...
```

Expected: all packages pass. A narrow `-run` filter is not sufficient here — changing `UpdateItemQty`'s signature area has knock-on effects in cart tests.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/service/item_qty.go backend/internal/service/item_qty_test.go backend/internal/service/store.go backend/internal/service/store_test.go backend/internal/handler/errors.go
git commit -m "fix: cap digital products at qty 1 to stop silent overcharging

The outbox worker fulfils exam and course items ignoring qty, creating one
registration no matter what, while the buyer was charged per unit."
```

---

### Task 11: Frontend quantity lock

**Files:**
- Modify: `web/app/(student)/catalog/[id]/page.tsx`, `web/components/cart/CartLineItem.tsx`, `web/lib/i18n.ts`
- Test: `web/components/cart/CartLineItem.test.tsx` (create)

**Interfaces:**
- Consumes: `Product.type` from `@/lib/types`

- [ ] **Step 1: Add the i18n key**

`id` block, after `product_qty_label:`:

```ts
    product_digital_single_qty: "Produk digital dibeli 1× per akun.",
```

`en` block, same position:

```ts
    product_digital_single_qty: "Digital products are limited to one per account.",
```

- [ ] **Step 2: Write the failing test**

Create `web/components/cart/CartLineItem.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CartLineItem } from "./CartLineItem";

const base = {
  id: "item-1",
  product_id: "p1",
  name: "Item",
  unit_price: 10000,
  qty: 1,
  jumlah: 10000,
};

describe("CartLineItem", () => {
  it("hides the quantity stepper for digital products", () => {
    render(
      <CartLineItem
        item={{ ...base, product_type: "exam" } as any}
        onRemove={() => {}}
        onQtyChange={() => {}}
      />,
    );
    expect(screen.queryByLabelText("Tambah jumlah")).toBeNull();
    expect(screen.getByText("Produk digital dibeli 1× per akun.")).toBeTruthy();
  });

  it("keeps the stepper for physical products", () => {
    render(
      <CartLineItem
        item={{ ...base, product_type: "book" } as any}
        onRemove={() => {}}
        onQtyChange={() => {}}
      />,
    );
    expect(screen.getByLabelText("Tambah jumlah")).toBeTruthy();
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/cart/CartLineItem.test.tsx
```

Expected: FAIL — the stepper renders for both, and the message is absent.

- [ ] **Step 4: Gate the stepper in `CartLineItem`**

In `web/components/cart/CartLineItem.tsx`, add near the top of the component body:

```tsx
  const isDigital = item.product_type === "exam" || item.product_type === "course";
```

Add `aria-label` attributes to the two stepper buttons so they are addressable — `aria-label="Kurangi jumlah"` on the minus button and `aria-label="Tambah jumlah"` on the plus button. Then wrap the whole stepper block (the `<div>` containing the minus button, the qty span, and the plus button) so it only renders for physical items, with the message in its place otherwise:

```tsx
          {isDigital ? (
            <span className="text-xs text-ink-500">{t("product_digital_single_qty" as any)}</span>
          ) : (
            <div className="flex items-center gap-2">
              {/* existing minus button, qty span, plus button unchanged */}
            </div>
          )}
```

Import `useTranslation` from `@/lib/i18n` if the component does not already use it.

- [ ] **Step 5: Gate the stepper on the product detail page**

In `web/app/(student)/catalog/[id]/page.tsx`, add after `const cover = fileUrl(product.image_url);`:

```tsx
  const isDigital = product.type === "exam" || product.type === "course";
```

Change the stepper's render condition from `{!alreadyInCart && (` to `{!alreadyInCart && !isDigital && (`, and add immediately after that block:

```tsx
            {isDigital && (
              <p className="mb-3 text-xs text-ink-500">{t("product_digital_single_qty" as any)}</p>
            )}
```

Because `qty` state stays at its initial `1` for digital products, `handleAdd` already sends the correct value — no change needed there.

- [ ] **Step 6: Run the tests and watch them pass**

```bash
cd web && npx vitest run components/cart/CartLineItem.test.tsx && npx tsc --noEmit
```

Expected: PASS, 2 tests; no type errors.

- [ ] **Step 7: Commit**

```bash
git add web/components/cart/CartLineItem.tsx web/components/cart/CartLineItem.test.tsx web/app/\(student\)/catalog/\[id\]/page.tsx web/lib/i18n.ts
git commit -m "feat: lock digital products to a single unit in the cart and on the detail page"
```

---

# Slice 4 — Shipping block on the order page

### Task 12: `ShippingInfo` component and order detail wiring

**Files:**
- Create: `web/components/orders/ShippingInfo.tsx`, `web/components/orders/ShippingInfo.test.tsx`
- Modify: `web/app/(student)/orders/[id]/page.tsx`, `web/lib/i18n.ts`

**Interfaces:**
- Consumes: `Order` from `@/lib/types` — fields `selected_courier`, `selected_service`, `shipping_address`, `shipping_cost`, `tracking_number`, `items[].product_type`
- Produces: `ShippingInfo({ order }: { order: Order })` — renders `null` for orders with no physical item.

- [ ] **Step 1: Add the i18n keys**

`id` block, after `order_shipping:`:

```ts
    order_shipping_heading: "Pengiriman",
    order_shipping_address: "Alamat tujuan",
    order_shipping_courier: "Kurir",
    order_shipping_estimate_note: "Estimasi — bukan tarif kurir",
```

`en` block, same position:

```ts
    order_shipping_heading: "Shipping",
    order_shipping_address: "Delivery address",
    order_shipping_courier: "Courier",
    order_shipping_estimate_note: "Estimate — not a carrier quote",
```

- [ ] **Step 2: Fix the `shipping_address` type — it exists but is wrong**

`Order.shipping_address` is currently typed `string` in `web/lib/types.ts` (verified), but the backend stores it as JSONB and serialises it as a JSON object, so at runtime it is an object and the string type is a latent bug. The `ShippingInfo` component below reads it as an object. Change the field:

```ts
  shipping_address?: Record<string, string> | null;
```

```bash
grep -n "shipping_address" web/lib/types.ts
```

Expected: the single `Order` field, now typed as the record. Then run `npx tsc --noEmit` after the component is written (Step 8) to confirm no other reader assumed a string — if one did, that reader was already relying on the wrong type and should be reconciled, not reverted.

- [ ] **Step 3: Write the failing test**

Create `web/components/orders/ShippingInfo.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ShippingInfo } from "./ShippingInfo";

const physicalOrder = {
  id: "o1",
  status: "paid",
  shipping_cost: 15000,
  selected_courier: "JNE",
  selected_service: "REG",
  tracking_number: "JP1234567",
  shipping_address: {
    penerima: "Saifullah Panca",
    telepon: "08123456789",
    alamat: "Jl. Merdeka No. 1",
    kode_pos: "40123",
  },
  items: [{ id: "i1", product_id: "p1", product_type: "book", name: "Buku", unit_price: 1, qty: 1 }],
} as any;

describe("ShippingInfo", () => {
  it("shows courier, service, address and tracking for a physical order", () => {
    render(<ShippingInfo order={physicalOrder} />);
    expect(screen.getByText("JNE — REG")).toBeTruthy();
    expect(screen.getByText(/Jl. Merdeka No. 1/)).toBeTruthy();
    expect(screen.getByText("JP1234567")).toBeTruthy();
  });

  it("renders nothing for a digital-only order", () => {
    const digital = {
      ...physicalOrder,
      items: [{ id: "i1", product_id: "p1", product_type: "exam", name: "Ujian", unit_price: 1, qty: 1 }],
    };
    const { container } = render(<ShippingInfo order={digital} />);
    expect(container.firstChild).toBeNull();
  });

  it("flags a flat-rate estimate as not being a carrier quote", () => {
    const flat = { ...physicalOrder, selected_courier: "Flat", selected_service: "Standard" };
    render(<ShippingInfo order={flat} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });
});
```

- [ ] **Step 4: Run it and watch it fail**

```bash
cd web && npx vitest run components/orders/ShippingInfo.test.tsx
```

Expected: FAIL — unresolved import.

- [ ] **Step 5: Write the component**

Create `web/components/orders/ShippingInfo.tsx`:

```tsx
"use client";

import type { Order } from "@/lib/types";
import { useTranslation } from "@/lib/i18n";
import { formatRupiah } from "@/lib/format";

const PHYSICAL_TYPES = new Set(["book", "merchandise", "medal"]);

export interface ShippingInfoProps {
  order: Order;
}

export function ShippingInfo({ order }: ShippingInfoProps) {
  const { t } = useTranslation();

  const hasPhysical = (order.items ?? []).some((i) => PHYSICAL_TYPES.has(i.product_type));
  if (!hasPhysical) return null;

  const addr = order.shipping_address ?? {};
  const addressLine = [addr.penerima, addr.telepon, addr.alamat, addr.kode_pos]
    .filter(Boolean)
    .join(" · ");

  const courier = [order.selected_courier, order.selected_service].filter(Boolean).join(" — ");
  const isEstimate = order.selected_courier === "Flat";

  return (
    <section className="rounded-lg border border-line bg-surface p-5">
      <h2 className="mb-4 font-serif text-lg font-semibold text-ink-900">
        {t("order_shipping_heading" as any)}
      </h2>
      <dl className="flex flex-col gap-2 text-sm">
        {addressLine && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_shipping_address" as any)}</dt>
            <dd className="text-right text-ink-900">{addressLine}</dd>
          </div>
        )}
        {courier && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_shipping_courier" as any)}</dt>
            <dd className="text-right text-ink-900">
              {courier}
              {isEstimate && (
                <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
                  {t("order_shipping_estimate_note" as any)}
                </span>
              )}
            </dd>
          </div>
        )}
        <div className="flex items-start justify-between gap-3">
          <dt className="text-ink-500">{t("order_shipping")}</dt>
          <dd className="text-right text-ink-900">{formatRupiah(order.shipping_cost ?? 0)}</dd>
        </div>
        {order.tracking_number && (
          <div className="flex items-start justify-between gap-3">
            <dt className="text-ink-500">{t("order_tracking")}</dt>
            <dd className="text-right text-ink-900">{order.tracking_number}</dd>
          </div>
        )}
      </dl>
    </section>
  );
}
```

- [ ] **Step 6: Run it and watch it pass**

```bash
cd web && npx vitest run components/orders/ShippingInfo.test.tsx
```

Expected: PASS, 3 tests.

- [ ] **Step 7: Mount it and move tracking out of `PaymentInfo`**

In `web/app/(student)/orders/[id]/page.tsx`:

Add the import:

```tsx
import { ShippingInfo } from "@/components/orders/ShippingInfo";
```

In `PaymentInfo`, delete these three lines so tracking is not shown twice:

```tsx
  if (order.tracking_number) {
    rows.push({ labelKey: "order_tracking", value: order.tracking_number });
  }
```

Render the block in the page body, immediately after the summary `<dl>` closes and before the next section:

```tsx
            <ShippingInfo order={order} />
```

- [ ] **Step 8: Verify and commit**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

```bash
git add web/components/orders/ShippingInfo.tsx web/components/orders/ShippingInfo.test.tsx web/app/\(student\)/orders/\[id\]/page.tsx web/lib/i18n.ts
git commit -m "feat: show courier, address and tracking on the order detail page"
```

---

# Slice 5 — Checkout address

### Task 13: Address summary with an explicit edit toggle

**Files:**
- Create: `web/components/cart/ShippingAddressSummary.tsx`, `web/components/cart/ShippingAddressSummary.test.tsx`
- Modify: `web/components/cart/ShippingAddressForm.tsx`, `web/app/(student)/cart/page.tsx`, `web/lib/i18n.ts`

**Interfaces:**
- Produces:
  ```ts
  export interface SavedAddress {
    penerima: string; telepon: string; alamat: string;
    provinsi_id: string; kota_id: string; kecamatan_id: string; kode_pos: string;
  }
  export function ShippingAddressSummary(props: {
    address: SavedAddress;
    onEdit: () => void;
  }): JSX.Element;
  ```

No backend work: `PatchCart` already binds `shipping_address` as raw JSON (`backend/internal/handler/order.go:141`) and `orders.shipping_address` is already JSONB, so a richer payload needs no schema or handler change.

- [ ] **Step 1: Add the i18n keys**

`id` block, near the other cart keys:

```ts
    cart_address_heading: "Alamat Pengiriman",
    cart_address_change: "Ubah",
    cart_address_recipient: "Nama penerima",
    cart_address_phone: "Nomor telepon",
    cart_address_street: "Alamat lengkap",
    cart_address_incomplete_profile: "Lengkapi alamat pengiriman untuk melanjutkan.",
```

`en` block:

```ts
    cart_address_heading: "Shipping Address",
    cart_address_change: "Change",
    cart_address_recipient: "Recipient name",
    cart_address_phone: "Phone number",
    cart_address_street: "Full address",
    cart_address_incomplete_profile: "Complete your shipping address to continue.",
```

- [ ] **Step 2: Write the failing test**

Create `web/components/cart/ShippingAddressSummary.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ShippingAddressSummary } from "./ShippingAddressSummary";

const address = {
  penerima: "Saifullah Panca",
  telepon: "08123456789",
  alamat: "Jl. Merdeka No. 1",
  provinsi_id: "32",
  kota_id: "3273",
  kecamatan_id: "327301",
  kode_pos: "40123",
};

describe("ShippingAddressSummary", () => {
  it("shows the saved recipient, phone and street instead of a form", () => {
    render(<ShippingAddressSummary address={address} onEdit={() => {}} />);
    expect(screen.getByText("Saifullah Panca")).toBeTruthy();
    expect(screen.getByText(/08123456789/)).toBeTruthy();
    expect(screen.getByText(/Jl. Merdeka No. 1/)).toBeTruthy();
    expect(screen.queryByLabelText("Provinsi")).toBeNull();
  });

  it("asks the parent to open the form when Ubah is pressed", () => {
    const onEdit = vi.fn();
    render(<ShippingAddressSummary address={address} onEdit={onEdit} />);
    fireEvent.click(screen.getByRole("button", { name: "Ubah" }));
    expect(onEdit).toHaveBeenCalledTimes(1);
  });

  it("prompts to complete the address when a required part is missing", () => {
    render(
      <ShippingAddressSummary address={{ ...address, alamat: "" }} onEdit={() => {}} />,
    );
    expect(screen.getByText("Lengkapi alamat pengiriman untuk melanjutkan.")).toBeTruthy();
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/cart/ShippingAddressSummary.test.tsx
```

Expected: FAIL — unresolved import.

- [ ] **Step 4: Write the summary component**

Create `web/components/cart/ShippingAddressSummary.tsx`:

```tsx
"use client";

import { useTranslation } from "@/lib/i18n";
import { Button } from "@/components/ui/button";

export interface SavedAddress {
  penerima: string;
  telepon: string;
  alamat: string;
  provinsi_id: string;
  kota_id: string;
  kecamatan_id: string;
  kode_pos: string;
}

export interface ShippingAddressSummaryProps {
  address: SavedAddress;
  onEdit: () => void;
}

export function isAddressComplete(a: SavedAddress): boolean {
  return Boolean(
    a.penerima && a.telepon && a.alamat && a.provinsi_id && a.kota_id && a.kecamatan_id && a.kode_pos,
  );
}

export function ShippingAddressSummary({ address, onEdit }: ShippingAddressSummaryProps) {
  const { t } = useTranslation();
  const complete = isAddressComplete(address);

  return (
    <div className="rounded-lg border border-line bg-surface p-5">
      <div className="mb-3 flex items-center justify-between">
        <h2 className="font-serif text-base font-semibold text-ink-900">
          {t("cart_address_heading" as any)}
        </h2>
        <Button type="button" variant="ghost" size="sm" onClick={onEdit}>
          {t("cart_address_change" as any)}
        </Button>
      </div>

      {complete ? (
        <div className="flex flex-col gap-0.5 text-sm">
          <span className="font-medium text-ink-900">{address.penerima}</span>
          <span className="text-ink-600">{address.telepon}</span>
          <span className="text-ink-600">
            {address.alamat} · {address.kode_pos}
          </span>
        </div>
      ) : (
        <p className="text-sm text-ink-500">{t("cart_address_incomplete_profile" as any)}</p>
      )}
    </div>
  );
}
```

- [ ] **Step 5: Run it and watch it pass**

```bash
cd web && npx vitest run components/cart/ShippingAddressSummary.test.tsx
```

Expected: PASS, 3 tests.

- [ ] **Step 6: Add the three missing fields to the form**

In `web/components/cart/ShippingAddressForm.tsx`, add recipient, phone and street inputs above the province select. Extend `ShippingAddressFormState` with `penerima`, `telepon` and `alamat`, seed them in the profile effect alongside the existing `setProvinsiId(profile.provinsi_id ?? "")` block:

```tsx
      setPenerima(profile.name ?? "");
      setTelepon(profile.phone ?? "");
      setAlamat(profile.alamat_domisili ?? "");
```

and render them:

```tsx
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="penerima" className="text-xs font-semibold text-ink-600">
          {t("cart_address_recipient" as any)}
        </Label>
        <input id="penerima" className={FIELD_CLASS} value={penerima}
          onChange={(e) => setPenerima(e.target.value)} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="telepon" className="text-xs font-semibold text-ink-600">
          {t("cart_address_phone" as any)}
        </Label>
        <input id="telepon" className={FIELD_CLASS} value={telepon}
          onChange={(e) => setTelepon(e.target.value)} />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="alamat" className="text-xs font-semibold text-ink-600">
          {t("cart_address_street" as any)}
        </Label>
        <input id="alamat" className={FIELD_CLASS} value={alamat}
          onChange={(e) => setAlamat(e.target.value)} />
      </div>
```

`FIELD_CLASS` is the existing shared class constant at the top of the file (verified — it is named `FIELD_CLASS`, not `inputClass`). Include the three new values in whatever object `onAddressChange` emits, and add them to the "Cek Ongkir" button's `disabled` expression so an incomplete address cannot request a quote.

- [ ] **Step 7: Switch the cart page to summary-first**

In `web/app/(student)/cart/page.tsx`, add state:

```tsx
  const [editingAddress, setEditingAddress] = useState(false);
```

Replace the `{hasPhysical && (<ShippingAddressForm ... />)}` block with:

```tsx
            {hasPhysical &&
              (editingAddress || !isAddressComplete(shippingAddress as any) ? (
                <ShippingAddressForm
                  profile={profile}
                  onAddressChange={handleAddressChange}
                  onCheckShipping={() => {
                    setEditingAddress(false);
                    handleCheckShipping();
                  }}
                  isCheckingShipping={shippingRates.isPending}
                />
              ) : (
                <ShippingAddressSummary
                  address={shippingAddress as any}
                  onEdit={() => setEditingAddress(true)}
                />
              ))}
```

Import both `ShippingAddressSummary` and `isAddressComplete` from `@/components/cart/ShippingAddressSummary`.

- [ ] **Step 8: Verify and commit**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

```bash
git add web/components/cart/ShippingAddressSummary.tsx web/components/cart/ShippingAddressSummary.test.tsx web/components/cart/ShippingAddressForm.tsx web/app/\(student\)/cart/page.tsx web/lib/i18n.ts
git commit -m "feat: show saved shipping address as a summary with an explicit edit toggle"
```

---

# Slice 6 — Stop fabricated shipping rates

### Task 14: Pure rate resolver, honest Noop client

**Files:**
- Create: `backend/internal/service/shipping_rates.go`, `backend/internal/service/shipping_rates_test.go`
- Modify: `backend/internal/service/ports_logistics.go`, `backend/internal/service/store.go`, `backend/internal/service/store_test.go`

**Interfaces:**
- Produces:
  ```go
  var ErrShippingUnavailable = errors.New("shipping is unavailable")
  // CourierRate gains: IsEstimate bool `json:"is_estimate"`
  func resolveShippingRates(rates []CourierRate, clientErr error, flatRate int64) ([]CourierRate, error)
  ```
  Task 15 consumes `is_estimate` over the wire.

**Why a pure resolver:** `Service.GetShippingRates` needs `s.storeRepo` for system config, so it cannot be unit-tested without a database — and `shimService.GetShippingRates` (`store_test.go:361`) is a one-line reimplementation that skips the fallback entirely. Putting the branching in a pure function makes the actual decision testable and leaves the method a thin wrapper.

- [ ] **Step 1: Write the failing test**

Create `backend/internal/service/shipping_rates_test.go`:

```go
package service

import (
	"context"
	"errors"
	"testing"
)

func TestResolveShippingRates(t *testing.T) {
	real := []CourierRate{{Courier: "JNE", Service: "REG", Price: 18000}}

	t.Run("real quotes pass through untouched and are not flagged", func(t *testing.T) {
		got, err := resolveShippingRates(real, nil, 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if len(got) != 1 || got[0].Courier != "JNE" {
			t.Fatalf("want the carrier quote, got %+v", got)
		}
		if got[0].IsEstimate {
			t.Error("a real carrier quote must not be flagged as an estimate")
		}
	})

	t.Run("client failure falls back to the configured flat rate, flagged", func(t *testing.T) {
		got, err := resolveShippingRates(nil, errors.New("upstream down"), 12000)
		if err != nil {
			t.Fatalf("want nil error, got %v", err)
		}
		if len(got) != 1 || got[0].Price != 12000 {
			t.Fatalf("want the flat rate, got %+v", got)
		}
		if !got[0].IsEstimate {
			t.Error("the flat-rate fallback must be flagged as an estimate")
		}
	})

	t.Run("empty quote list falls back too", func(t *testing.T) {
		got, err := resolveShippingRates([]CourierRate{}, nil, 12000)
		if err != nil || len(got) != 1 || !got[0].IsEstimate {
			t.Fatalf("want a flagged flat rate, got %+v err=%v", got, err)
		}
	})

	t.Run("no quotes and no flat rate is an explicit failure, never a made-up number", func(t *testing.T) {
		got, err := resolveShippingRates(nil, errors.New("upstream down"), 0)
		if !errors.Is(err, ErrShippingUnavailable) {
			t.Fatalf("want ErrShippingUnavailable, got %v", err)
		}
		if got != nil {
			t.Fatalf("want no rates, got %+v", got)
		}
	})
}

// The Noop client stands in for "no Biteship key configured". It used to return
// hardcoded JNE and TIKI quotes with a nil error, which meant GetShippingRates
// returned early and billed the buyer an invented amount.
func TestNoopLogisticsClient_ReturnsNoRates(t *testing.T) {
	rates, err := (&NoopLogisticsClient{}).GetRates(context.Background(), ShippingQuoteRequest{})
	if len(rates) != 0 {
		t.Fatalf("the noop client must not invent carrier quotes, got %+v", rates)
	}
	if err == nil {
		t.Fatal("the noop client must report that shipping is unavailable")
	}
}
```

- [ ] **Step 2: Run it and watch it fail**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go test ./internal/service/ -run "TestResolveShippingRates|TestNoopLogisticsClient" -v
```

Expected: FAIL — `undefined: resolveShippingRates`, and the Noop test fails because it still returns two rates.

- [ ] **Step 3: Add `IsEstimate` and make the Noop client honest**

In `backend/internal/service/ports_logistics.go`, add the field to `CourierRate`:

```go
type CourierRate struct {
	Courier       string `json:"courier"`
	Service       string `json:"service"`
	EstimatedDays int    `json:"estimated_days"`
	Price         int64  `json:"price"`
	// IsEstimate marks a rate that did not come from a carrier — currently the
	// configured flat-rate fallback. The storefront must label these so a buyer
	// is never shown an invented figure that looks like a real quote.
	IsEstimate bool `json:"is_estimate"`
}
```

Replace `NoopLogisticsClient.GetRates` entirely:

```go
func (n *NoopLogisticsClient) GetRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	// Returning fabricated JNE/TIKI quotes here meant that with no Biteship key
	// configured the storefront billed buyers a made-up shipping cost through
	// Midtrans, indistinguishable from a real quote. Failing is the honest
	// answer; the flat-rate fallback in resolveShippingRates handles the rest.
	return nil, ErrShippingUnavailable
}
```

- [ ] **Step 4: Write the resolver**

Create `backend/internal/service/shipping_rates.go`:

```go
package service

import "errors"

// ErrShippingUnavailable means no carrier quote could be obtained and no
// flat-rate fallback is configured.
var ErrShippingUnavailable = errors.New("shipping is unavailable")

// resolveShippingRates decides what the storefront may show for a shipping
// quote. Carrier quotes win. Otherwise the configured flat rate stands in,
// explicitly flagged as an estimate. If neither exists the caller gets an
// error — never an invented figure.
func resolveShippingRates(rates []CourierRate, clientErr error, flatRate int64) ([]CourierRate, error) {
	if clientErr == nil && len(rates) > 0 {
		return rates, nil
	}
	if flatRate > 0 {
		return []CourierRate{{
			Courier:    "Flat",
			Service:    "Standard",
			Price:      flatRate,
			IsEstimate: true,
		}}, nil
	}
	return nil, ErrShippingUnavailable
}
```

- [ ] **Step 5: Make `GetShippingRates` a thin wrapper**

Replace `Service.GetShippingRates` in `backend/internal/service/store.go`:

```go
func (s *Service) GetShippingRates(ctx context.Context, req ShippingQuoteRequest) ([]CourierRate, error) {
	rates, clientErr := s.logisticsClient().GetRates(ctx, req)

	var flatRate int64
	if cfg, cfgErr := s.GetSystemConfig(ctx); cfgErr == nil {
		if raw := cfg["shipping_fallback_flat_rate"]; raw != "" {
			if _, scanErr := fmt.Sscanf(raw, "%d", &flatRate); scanErr != nil {
				flatRate = 0
			}
		}
	}

	return resolveShippingRates(rates, clientErr, flatRate)
}
```

- [ ] **Step 6: Fix the pre-existing shim test that relied on fabricated rates**

`TestGetShippingRates` in `store_test.go:526` asserts `len(rates) != 0` against `shimService`, which returns the Noop client's output directly. With the Noop client now failing, that assertion is wrong. Replace the test:

```go
// The shim delegates straight to the injected client, so this only asserts the
// wiring. The real fallback decision is covered by TestResolveShippingRates.
func TestGetShippingRates(t *testing.T) {
	ctx := context.Background()
	svc := newShim(newFakeStoreRepo())
	_, err := svc.GetShippingRates(ctx, ShippingQuoteRequest{DestinationPostalCode: "12345", WeightGrams: 500})
	if !errors.Is(err, ErrShippingUnavailable) {
		t.Fatalf("with no carrier configured the shim should surface ErrShippingUnavailable, got %v", err)
	}
}
```

- [ ] **Step 7: Map the error to a clear HTTP status**

In `backend/internal/handler/errors.go`:

```go
	case errors.Is(err, service.ErrShippingUnavailable):
		status, apiErr = http.StatusServiceUnavailable, APIError{Code: "shipping_unavailable", Message: "shipping is not available right now"}
```

- [ ] **Step 8: Run the full backend suite**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go build ./... && go vet ./... && go test ./...
```

Expected: all packages pass. Other tests may construct services with `&service.NoopLogisticsClient{}` and assume a shipping quote succeeds — for example the DB-backed handler harnesses. Any such failure is a genuine finding: fix it by configuring `shipping_fallback_flat_rate` in that test's setup, never by restoring the fake rates.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/service/shipping_rates.go backend/internal/service/shipping_rates_test.go backend/internal/service/ports_logistics.go backend/internal/service/store.go backend/internal/service/store_test.go backend/internal/handler/errors.go
git commit -m "fix: stop billing buyers fabricated shipping rates

Without a Biteship key the noop client returned hardcoded JNE and TIKI
quotes with a nil error, so the flat-rate fallback never ran and invented
figures reached the order total and Midtrans. Fallbacks are now flagged as
estimates and the absence of any rate is an explicit failure."
```

---

### Task 15: Surface the estimate flag and block checkout when no rate exists

**Files:**
- Modify: `web/components/cart/CourierRateList.tsx`, `web/app/(student)/cart/page.tsx`, `web/lib/types.ts`, `web/lib/i18n.ts`
- Test: `web/components/cart/CourierRateList.test.tsx` (create)

**Interfaces:**
- Consumes: `is_estimate` on the courier rate payload from Task 14

- [ ] **Step 1: Add the i18n keys and the type field**

`id` block:

```ts
    cart_rate_estimate_badge: "Estimasi — bukan tarif kurir",
    cart_shipping_unavailable: "Pengiriman belum tersedia, hubungi admin.",
```

`en` block:

```ts
    cart_rate_estimate_badge: "Estimate — not a carrier quote",
    cart_shipping_unavailable: "Shipping is not available yet, please contact an admin.",
```

In `web/lib/types.ts`, add to the `CourierRate` interface (verified — it has `courier`, `service`, `estimated_days`, `price`):

```ts
  is_estimate?: boolean;
```

```bash
grep -n "interface CourierRate" web/lib/types.ts
```

- [ ] **Step 2: Write the failing test**

Create `web/components/cart/CourierRateList.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { CourierRateList } from "./CourierRateList";

describe("CourierRateList", () => {
  it("labels an estimate so it cannot be mistaken for a carrier quote", () => {
    render(
      <CourierRateList
        rates={[{ courier: "Flat", service: "Standard", price: 12000, is_estimate: true } as any]}
        selectedKey={null}
        onSelect={() => {}}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not label a real carrier quote", () => {
    render(
      <CourierRateList
        rates={[{ courier: "JNE", service: "REG", price: 18000 } as any]}
        selectedKey={null}
        onSelect={() => {}}
        isLoading={false}
        isError={false}
      />,
    );
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });
});
```

- [ ] **Step 3: Run it and watch it fail**

```bash
cd web && npx vitest run components/cart/CourierRateList.test.tsx
```

Expected: FAIL — the badge is never rendered.

- [ ] **Step 4: Render the badge**

In `web/components/cart/CourierRateList.tsx`, inside the element that renders each rate's courier and service labels, add:

```tsx
              {rate.is_estimate && (
                <span className="ml-2 rounded bg-warn-bg px-1.5 py-0.5 text-xs text-warn">
                  {t("cart_rate_estimate_badge" as any)}
                </span>
              )}
```

Import `useTranslation` from `@/lib/i18n` if the component does not already use it.

- [ ] **Step 5: Block checkout when no rate is obtainable**

In `web/app/(student)/cart/page.tsx`, add below the existing rate-list block:

```tsx
            {hasPhysical && shippingRates.isError && (
              <div className="rounded-lg border border-danger/30 bg-danger-bg px-5 py-4 text-sm text-danger">
                {t("cart_shipping_unavailable" as any)}
              </div>
            )}
```

Add to the checkout button's `disabled` expression:

```tsx
              || (hasPhysical && shippingRates.isError)
```

Digital-only carts are unaffected: `hasPhysical` is false, so neither the message nor the block applies.

- [ ] **Step 6: Verify and commit**

```bash
cd web && npx tsc --noEmit && npx vitest run
```

```bash
git add web/components/cart/CourierRateList.tsx web/components/cart/CourierRateList.test.tsx web/app/\(student\)/cart/page.tsx web/lib/types.ts web/lib/i18n.ts
git commit -m "feat: label estimated shipping rates and block checkout when none exist"
```

---

## Final gate

- [ ] **Run everything, both stacks**

```bash
export GOROOT=/opt/homebrew/Cellar/go/1.26.3/libexec && cd backend && go build ./... && go vet ./... && go test ./...
```

```bash
cd web && npx tsc --noEmit && npx vitest run && npm run build
```

Both must be green. A narrow `-run` filter passing is not sufficient evidence — in this repo a full `go test ./...` has twice caught regressions a filtered run missed.

- [ ] **Verify the migration applies and rolls back cleanly**

```bash
make migrate-up && make migrate-down && make migrate-up
```

- [ ] **Confirm no fabricated rates remain**

```bash
grep -rn "TIKI\|15000" backend/internal/service/ports_logistics.go
```

Expected: no matches.
