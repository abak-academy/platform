import { describe, expect, it } from "vitest";
import { toCsv } from "./csv";

describe("toCsv", () => {
  it("quotes commas, quotes and newlines", () => {
    expect(toCsv(["a", "b"], [['x,"y"', "l1\nl2"]])).toBe('a,b\n"x,""y""","l1\nl2"\n');
  });

  it("defuses cells a spreadsheet would read as a formula", () => {
    expect(toCsv(["a"], [["=SUM(A1)"], ["-1"], [null]])).toBe("a\n'=SUM(A1)\n'-1\n\n");
  });
});
