import { describe, it, expect } from "vitest";
import { isPhysicalType, isDigitalType } from "./shipping";
import type { ProductType } from "./types";

// Go counterpart: backend/internal/service/physical_type_test.go
describe("isPhysicalType / isDigitalType", () => {
  const ALL_TYPES: ProductType[] = ["book", "course", "exam", "merchandise", "medal"];

  it.each(ALL_TYPES)("exactly one of isPhysicalType/isDigitalType is true for '%s'", (type) => {
    expect(isPhysicalType(type)).not.toBe(isDigitalType(type));
  });
});
