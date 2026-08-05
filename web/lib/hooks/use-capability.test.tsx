import { describe, it, expect, vi, beforeEach } from "vitest";
import { renderHook } from "@testing-library/react";
import { useHasCapability, hasCapability } from "./use-capability";
import { NAV_CONFIG } from "@/lib/nav-config";

const mocks = vi.hoisted(() => ({
  authState: { token: "test-token", user: null as { role: string } | null },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: (selector: (s: typeof mocks.authState) => unknown) =>
    selector(mocks.authState),
}));

vi.mock("@/lib/hooks/auth", () => ({
  useMe: () => ({ data: undefined, isError: false }),
}));

function renderCapability(role: string | null, required: string): boolean {
  mocks.authState.user = role ? { role } : null;
  const { result } = renderHook(() => useHasCapability(required));
  return result.current;
}

describe("useHasCapability", () => {
  beforeEach(() => {
    mocks.authState.token = "test-token";
    mocks.authState.user = null;
  });

  it("denies revenue:read to admin_store", () => {
    expect(renderCapability("admin_store", "revenue:read")).toBe(false);
  });

  it("grants revenue:read to super_admin", () => {
    expect(renderCapability("super_admin", "revenue:read")).toBe(true);
  });

  it("still grants orders:write to admin_store", () => {
    expect(renderCapability("admin_store", "orders:write")).toBe(true);
  });

  it("denies everything to an unknown role", () => {
    expect(renderCapability("admin_marketing", "revenue:read")).toBe(false);
    expect(renderCapability("admin_marketing", "orders:write")).toBe(false);
  });

  it("denies everything when no role is resolved", () => {
    expect(renderCapability(null, "orders:write")).toBe(false);
  });
});

describe("hasCapability", () => {
  it("matches a wildcard prefix only within its own namespace", () => {
    expect(hasCapability("admin_store", "orders:read")).toBe(true);
    expect(hasCapability("admin_store", "orders-archive:read")).toBe(false);
  });

  it("denies an undefined role", () => {
    expect(hasCapability(undefined, "revenue:read")).toBe(false);
  });
});

describe("nav config revenue entry", () => {
  function hrefs(role: keyof typeof NAV_CONFIG): string[] {
    return NAV_CONFIG[role].flatMap((group) => group.items.map((item) => item.href));
  }

  it("no longer offers revenue to admin_store", () => {
    expect(hrefs("admin_store")).not.toContain("/admin/revenue");
  });

  it("still offers revenue to super_admin", () => {
    expect(hrefs("super_admin")).toContain("/admin/revenue");
  });
});
