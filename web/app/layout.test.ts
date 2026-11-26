import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

const src = readFileSync(join(__dirname, "layout.tsx"), "utf8");

describe("root layout fonts", () => {
  it("puts the next/font variable classes on <html>, where :root tokens resolve them", () => {
    const html = src.match(/<html[\s\S]*?>/)![0];
    for (const v of ["plusJakarta", "publicSans", "sourceSerif", "splineMono", "cinzel", "inter"]) {
      expect(html).toContain(`${v}.variable`);
    }
    const body = src.match(/<body[\s\S]*?>/)![0];
    expect(body).not.toContain(".variable");
  });
});
