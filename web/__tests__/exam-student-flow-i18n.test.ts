import { describe, it, expect } from "vitest";
import { DICT } from "@/lib/i18n";

type Dict = Record<string, string>;
const idDict = DICT.id as Dict;
const enDict = DICT.en as Dict;

const prefixes = ["examcard_", "examdetail_", "admin_exam_preview_"];
const keysOf = (dict: Dict) => Object.keys(dict).filter((k) => prefixes.some((p) => k.startsWith(p))).sort();
const placeholders = (text: string) => (text.match(/\{[a-zA-Z]+\}/g) ?? []).sort();

describe("exam student flow copy", () => {
  it("has the same keys in id and en", () => {
    expect(keysOf(enDict)).toEqual(keysOf(idDict));
    expect(keysOf(idDict).length).toBeGreaterThan(0);
  });

  it("uses the same placeholders in id and en", () => {
    for (const key of keysOf(idDict)) {
      expect(placeholders(enDict[key]), key).toEqual(placeholders(idDict[key]));
    }
  });

  it("has no empty strings", () => {
    for (const key of keysOf(idDict)) {
      expect(idDict[key].trim(), `id.${key}`).not.toBe("");
      expect(enDict[key].trim(), `en.${key}`).not.toBe("");
    }
  });
});
