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
