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

  it("scoped student template stays byte-for-byte legacy and has no password column", () => {
    expect(buildStudentTemplateCSV(false)).toBe(
      "name,school,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos\n" +
        'Budi Santoso,SMAN 1 Jakarta,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132\n' +
        "Siti Aminah,SMAN 1 Jakarta,SMA,,,,,,,,,,\n",
    );
    expect(buildStudentTemplateCSV(false)).not.toContain("password");
  });

  it("super-admin student template adds a blank optional password column", () => {
    const csv = buildStudentTemplateCSV(true);
    const rows = csv.trimEnd().split("\n");
    expect(rows[0]).toMatch(/,password$/);
    expect(rows[1]).toMatch(/,$/);
    expect(rows[2]).toMatch(/,$/);
    expect(csv).not.toContain("password123");
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

  it("student guide includes password only for super admin", () => {
    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t, false)).not.toContain("bulk_format_student_password");
    expect(buildStudentGuideText(t, true)).toContain("bulk_format_student_password");
  });
});
