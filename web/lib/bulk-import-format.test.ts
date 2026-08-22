import { describe, it, expect } from "vitest";
import type { I18nKey } from "./i18n";
import {
  buildSchoolTemplateCSV,
  buildStudentTemplateCSV,
  buildStudentGuideText,
  buildSchoolGuideText,
} from "./bulk-import-format";

describe("bulk-import-format templates", () => {
  it("student template has Kemendagri region names and uppercase jenjang", () => {
    const csv = buildStudentTemplateCSV();
    expect(csv).toContain("JAWA BARAT,KOTA BANDUNG,COBLONG");
    expect(csv).toContain(",SMA,");
    expect(csv.split("\n").filter(Boolean)).toHaveLength(3);
  });

  it("school template uses pipe-separated uppercase school_types", () => {
    const csv = buildSchoolTemplateCSV();
    expect(csv).toBe(
      "name,code,npsn,school_types,alamat\n" +
        'SMAN 1 Jakarta,SMAN1JKT,20100001,SMA|SMK,"Jl. Sudirman No. 1"\n' +
        "SMPN 5 Bandung,SMPN5BDG,,SMP,\n",
    );
  });

  it("guides include field rules from the translator", () => {
    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t)).toContain("bulk_format_student_school");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_code");
  });
});
