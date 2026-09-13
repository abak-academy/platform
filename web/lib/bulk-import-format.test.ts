import { describe, it, expect } from "vitest";
import { DICT, type I18nKey } from "./i18n";
import {
  buildSchoolTemplateCSV,
  buildStudentTemplateCSV,
  buildStudentGuideText,
  buildSchoolGuideText,
} from "./bulk-import-format";

describe("bulk-import-format templates", () => {
  it("uses required school_npsn identity instead of school name", () => {
    const csv = buildStudentTemplateCSV();
    expect(csv.split("\n")[0]).toContain("name,school_npsn,jenjang");
    expect(csv.split("\n")[0]).not.toContain("name,school,jenjang");

    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t)).toContain("bulk_format_student_school_npsn");
    expect(buildStudentGuideText(t)).not.toContain("bulk_format_student_school\n");
  });

  it("student template has Kemendagri region names and uppercase jenjang", () => {
    const csv = buildStudentTemplateCSV();
    expect(csv).toContain("JAWA BARAT,KOTA BANDUNG,COBLONG");
    expect(csv).toContain(",SMA,");
    expect(csv.split("\n").filter(Boolean)).toHaveLength(3);
  });

  it("scoped student template uses school_npsn and has no password column", () => {
    expect(buildStudentTemplateCSV(false)).toBe(
      "name,school_npsn,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos\n" +
        'Budi Santoso,20100001,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132\n' +
        "Siti Aminah,P1234567,SMA,,,,,,,,,,\n",
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
    expect(buildStudentGuideText(t)).toContain("bulk_format_student_school_npsn");
    expect(buildSchoolGuideText(t)).toContain("bulk_format_school_code");
  });

  it("school NPSN guide describes the enforced format and uniqueness", () => {
    expect(DICT.id.bulk_format_school_npsn).toContain("8");
    expect(DICT.id.bulk_format_school_npsn.toLowerCase()).toContain("unik");
    expect(DICT.en.bulk_format_school_npsn).toContain("8");
    expect(DICT.en.bulk_format_school_npsn.toLowerCase()).toContain("unique");
  });

  it("student NPSN guide explains the transitional legacy school header", () => {
    expect(DICT.id.bulk_format_student_school_npsn).toContain(
      "Selama rollout NPSN masih berlangsung, sekolah yang belum memiliki NPSN boleh mengganti header `school_npsn` dengan `school` dan mengisi nama sekolah yang terdaftar. Ini satu-satunya pengecualian untuk aturan jangan mengubah nama header di bawah.",
    );
    expect(DICT.id.bulk_format_student_school_npsn).toContain(
      "Jangan sertakan kedua header tersebut sekaligus; jika keduanya ada, nilai `school_npsn` yang akan digunakan.",
    );
    expect(DICT.en.bulk_format_student_school_npsn).toContain(
      "While the NPSN rollout is pending, a school without an NPSN may replace the `school_npsn` header with `school` and provide the registered school name. This is the sole exception to the do-not-rename-headers rule below.",
    );
    expect(DICT.en.bulk_format_student_school_npsn).toContain(
      "Do not include both headers; when both are present, the `school_npsn` value will be used.",
    );
  });

  it("student guide includes password only for super admin", () => {
    const t = (key: I18nKey) => key;
    expect(buildStudentGuideText(t, false)).not.toContain("bulk_format_student_password");
    expect(buildStudentGuideText(t, true)).toContain("bulk_format_student_password");
  });
});
