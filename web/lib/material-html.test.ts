import { describe, it, expect } from "vitest";
import { materialHTMLOrNull } from "./rich-text";

describe("material HTML clearing", () => {
  it.each([
    [null, null],
    ["   ", null],
    ["<p></p>", null],
    ["<p><br></p>", null],
    ["<p>&nbsp;</p>", null],
    ["<p>Passage</p>", "<p>Passage</p>"],
    ['<p><img src="/context.png"></p>', '<p><img src="/context.png"></p>'],
    ["<table><tbody><tr><td></td></tr></tbody></table>", "<table><tbody><tr><td></td></tr></tbody></table>"],
  ])("normalizes %s to %s", (html, want) => {
    expect(materialHTMLOrNull(html)).toBe(want);
  });
});
