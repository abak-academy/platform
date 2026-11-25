import { describe, it, expect } from "vitest";
import { cn } from "./utils";

describe("cn", () => {
  it("lets a caller's font size replace a type-* style", () => {
    expect(cn("type-label px-4", "text-xs")).toBe("px-4 text-xs");
  });

  it("lets a later type-* style replace an earlier one", () => {
    expect(cn("type-h2", "type-h3")).toBe("type-h3");
  });

  it("keeps a text colour next to a type-* style", () => {
    expect(cn("type-body-sm", "text-ink-600")).toBe("type-body-sm text-ink-600");
  });
});
