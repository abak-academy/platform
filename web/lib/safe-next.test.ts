import { describe, expect, it } from "vitest";
import { safeNext } from "./safe-next";

describe("safeNext", () => {
  it.each(["/cart", "/cart?x=1", "/catalog/abc#frag"])("accepts %s for a student", (n) => {
    expect(safeNext(n, "student")).toBe(n);
  });

  it.each([
    "https://evil.example",
    "//evil.example",
    "/\\evil.example",
    "javascript:alert(1)",
    "cart",
    "/\tevil",
    "/ca\nrt",
    "",
    null,
    undefined,
  ])("rejects %j", (n) => {
    expect(safeNext(n, "student")).toBeNull();
  });

  it.each(["super_admin", "admin_store", "affiliator", undefined])("returns null for role %s", (role) => {
    expect(safeNext("/cart", role)).toBeNull();
  });
});
