import { describe, expect, it } from "vitest";
import { DICT } from "./i18n";

describe("event copy", () => {
  it("labels the Toko filter Event in both languages and keeps the affiliate filter as Grouping", () => {
    expect(DICT.id.catalog_grouping_label).toBe("Event");
    expect(DICT.en.catalog_grouping_label).toBe("Event");
    expect(DICT.id.catalog_grouping_all).toBe("Semua event");
    expect(DICT.en.catalog_grouping_all).toBe("All events");
    expect(DICT.id.partner_new_grouping_label).toBe("Grouping");
    expect(DICT.en.partner_new_grouping_label).toBe("Grouping");
  });
});
