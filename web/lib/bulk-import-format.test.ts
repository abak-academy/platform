import { describe, it, expect } from "vitest";
import { DICT, type I18nKey } from "./i18n";
import {
  buildSchoolTemplateCSV,
  buildStudentTemplateCSV,
  buildStudentGuideText,
  buildSchoolGuideText,
} from "./bulk-import-format";

describe("bulk-import-format templates", () => {
  it("omits school identity from the admin-school template", () => {
    const csv = buildStudentTemplateCSV();
    expect(csv.split("\n")[0]).toContain("name,jenjang");
    expect(csv.split("\n")[0]).not.toContain("school_npsn");
    expect(csv.split("\n")[0]).not.toContain("school_code");

    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t)).not.toContain("bulk_format_student_school_npsn");
    expect(buildStudentGuideText(t)).not.toContain("bulk_format_student_school_code");
  });

  it("student template has Kemendagri region names and uppercase jenjang", () => {
    const csv = buildStudentTemplateCSV();
    expect(csv).toContain("JAWA BARAT,KOTA BANDUNG,COBLONG");
    expect(csv).toContain(",SMA,");
    expect(csv.split("\n").filter(Boolean)).toHaveLength(3);
  });

  it("scoped student template derives school from the authenticated admin", () => {
    expect(buildStudentTemplateCSV(false)).toBe(
      "name,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos\n" +
        'Budi Santoso,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132\n' +
        "Siti Aminah,SMA,,,,,,,,,,\n",
    );
    expect(buildStudentTemplateCSV(false)).not.toContain("password");
  });

  it("super-admin template supports NPSN or internal school code", () => {
    const csv = buildStudentTemplateCSV(true);
    const rows = csv.trimEnd().split("\n");
    expect(rows[0]).toContain("name,school_npsn,school_code,jenjang");
    expect(rows[1]).toContain("Budi Santoso,20100001,,SMA");
    expect(rows[2]).toContain("Siti Aminah,,YAYASANBIAN,SMA");
    expect(rows[2].split(",")).toHaveLength(rows[0].split(",").length);
    expect(rows[0]).toMatch(/,password$/);
    expect(rows[1]).toMatch(/,$/);
    expect(rows[2]).toMatch(/,$/);
    expect(csv).not.toContain("password123");
  });

  it("school template uses pipe-separated uppercase school_types", () => {
    const csv = buildSchoolTemplateCSV();
    expect(csv).toBe(
      "name,code,npsn,school_types,alamat,category,provinsi,kota\n" +
        'SMAN 1 Jakarta,SMAN1JKT,20100001,SMA|SMK,"Jl. Sudirman No. 1",SMA,DKI JAKARTA,KOTA JAKARTA PUSAT\n' +
        "SMPN 5 Bandung,SMPN5BDG,,SMP,,SMP,JAWA BARAT,KOTA BANDUNG\n",
    );
  });

  it("guides include field rules from the translator", () => {
    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t)).toContain("bulk_format_student_jenjang");
    expect(buildStudentGuideText(t, true)).toContain("bulk_format_student_school_npsn");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_code");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_category");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_provinsi");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_kota");
  });

  it("school NPSN guide describes the enforced format and uniqueness", () => {
    expect(DICT.id.bulk_format_school_npsn).toContain("8");
    expect(DICT.id.bulk_format_school_npsn.toLowerCase()).toContain("unik");
    expect(DICT.en.bulk_format_school_npsn).toContain("8");
    expect(DICT.en.bulk_format_school_npsn.toLowerCase()).toContain("unique");
  });

  it("super-admin guide documents the NPSN and school-code alternatives", () => {
    expect(DICT.id.bulk_format_student_school_npsn).toContain("school_code");
    expect(DICT.id.bulk_format_student_school_code).toContain("tanpa NPSN");
    expect(DICT.en.bulk_format_student_school_npsn).toContain("school_code");
    expect(DICT.en.bulk_format_student_school_code).toContain("without an NPSN");
  });

  it("student guide includes password only for super admin", () => {
    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t, false)).not.toContain("bulk_format_student_password");
    expect(buildStudentGuideText(t, true)).toContain("bulk_format_student_password");
  });
});
