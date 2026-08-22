import type { I18nKey } from "./i18n";

export type BulkFieldSpec = {
  column: string;
  required: boolean;
  ruleKey:
    | "bulk_format_student_name"
    | "bulk_format_student_school"
    | "bulk_format_student_jenjang"
    | "bulk_format_student_email"
    | "bulk_format_student_dob"
    | "bulk_format_student_gender"
    | "bulk_format_student_grade"
    | "bulk_format_student_target_exam"
    | "bulk_format_student_alamat_domisili"
    | "bulk_format_student_provinsi"
    | "bulk_format_student_kota"
    | "bulk_format_student_kecamatan"
    | "bulk_format_student_kode_pos"
    | "bulk_format_school_name"
    | "bulk_format_school_code"
    | "bulk_format_school_npsn"
    | "bulk_format_school_school_types"
    | "bulk_format_school_alamat";
  example: string;
};

export const STUDENT_TEMPLATE_HEADER =
  "name,school,jenjang,email,dob,gender,grade,target_exam,alamat_domisili,provinsi,kota,kecamatan,kode_pos";

export const STUDENT_TEMPLATE_ROWS = [
  'Budi Santoso,SMAN 1 Jakarta,SMA,budi@example.com,2008-05-14,male,11,UTBK,"Jl. Melati No. 3, RT 04",JAWA BARAT,KOTA BANDUNG,COBLONG,40132',
  "Siti Aminah,SMAN 1 Jakarta,SMA,,,,,,,,,,",
];

export const SCHOOL_TEMPLATE_HEADER = "name,code,npsn,school_types,alamat";

export const SCHOOL_TEMPLATE_ROWS = [
  'SMAN 1 Jakarta,SMAN1JKT,20100001,SMA|SMK,"Jl. Sudirman No. 1"',
  "SMPN 5 Bandung,SMPN5BDG,,SMP,",
];

export const STUDENT_BULK_FIELDS: BulkFieldSpec[] = [
  { column: "name", required: true, ruleKey: "bulk_format_student_name", example: "Budi Santoso" },
  { column: "school", required: true, ruleKey: "bulk_format_student_school", example: "SMAN 1 Jakarta" },
  { column: "jenjang", required: true, ruleKey: "bulk_format_student_jenjang", example: "SMA" },
  { column: "email", required: false, ruleKey: "bulk_format_student_email", example: "budi@example.com" },
  { column: "dob", required: false, ruleKey: "bulk_format_student_dob", example: "2008-05-14" },
  { column: "gender", required: false, ruleKey: "bulk_format_student_gender", example: "male" },
  { column: "grade", required: false, ruleKey: "bulk_format_student_grade", example: "11" },
  { column: "target_exam", required: false, ruleKey: "bulk_format_student_target_exam", example: "UTBK" },
  {
    column: "alamat_domisili",
    required: false,
    ruleKey: "bulk_format_student_alamat_domisili",
    example: "Jl. Melati No. 3, RT 04",
  },
  { column: "provinsi", required: false, ruleKey: "bulk_format_student_provinsi", example: "JAWA BARAT" },
  { column: "kota", required: false, ruleKey: "bulk_format_student_kota", example: "KOTA BANDUNG" },
  { column: "kecamatan", required: false, ruleKey: "bulk_format_student_kecamatan", example: "COBLONG" },
  { column: "kode_pos", required: false, ruleKey: "bulk_format_student_kode_pos", example: "40132" },
];

export const SCHOOL_BULK_FIELDS: BulkFieldSpec[] = [
  { column: "name", required: true, ruleKey: "bulk_format_school_name", example: "SMAN 1 Jakarta" },
  { column: "code", required: true, ruleKey: "bulk_format_school_code", example: "SMAN1JKT" },
  { column: "npsn", required: false, ruleKey: "bulk_format_school_npsn", example: "20100001" },
  { column: "school_types", required: false, ruleKey: "bulk_format_school_school_types", example: "SMA|SMK" },
  { column: "alamat", required: false, ruleKey: "bulk_format_school_alamat", example: "Jl. Sudirman No. 1" },
];

export const STUDENT_GUIDE_PITFALL_KEYS = [
  "bulk_format_pitfall_csv",
  "bulk_format_pitfall_header",
  "bulk_format_pitfall_comma",
  "bulk_format_pitfall_excel_text",
  "bulk_format_student_nis",
  "bulk_format_max_rows",
] as const satisfies readonly I18nKey[];

export const SCHOOL_GUIDE_PITFALL_KEYS = [
  "bulk_format_pitfall_csv",
  "bulk_format_pitfall_header",
  "bulk_format_pitfall_comma",
  "bulk_format_school_reupload",
  "bulk_format_max_rows",
] as const satisfies readonly I18nKey[];

export function buildStudentTemplateCSV(): string {
  return `${STUDENT_TEMPLATE_HEADER}\n${STUDENT_TEMPLATE_ROWS.join("\n")}\n`;
}

export function buildSchoolTemplateCSV(): string {
  return `${SCHOOL_TEMPLATE_HEADER}\n${SCHOOL_TEMPLATE_ROWS.join("\n")}\n`;
}

type Translate = (key: I18nKey) => string;

function formatFieldGuide(fields: BulkFieldSpec[], t: Translate): string {
  return fields
    .map((f) => {
      const req = f.required ? t("bulk_format_required") : t("bulk_format_optional");
      return `${f.column} (${req})\n  ${t(f.ruleKey)}\n  ${t("bulk_format_example")}: ${f.example}`;
    })
    .join("\n\n");
}

export function buildStudentGuideText(t: Translate): string {
  return [
    t("bulk_format_student_guide_title"),
    "",
    formatFieldGuide(STUDENT_BULK_FIELDS, t),
    "",
    ...STUDENT_GUIDE_PITFALL_KEYS.map((k) => `- ${t(k)}`),
    "",
  ].join("\n");
}

export function buildSchoolGuideText(t: Translate): string {
  return [
    t("bulk_format_school_guide_title"),
    "",
    formatFieldGuide(SCHOOL_BULK_FIELDS, t),
    "",
    ...SCHOOL_GUIDE_PITFALL_KEYS.map((k) => `- ${t(k)}`),
    "",
  ].join("\n");
}

export function downloadTextFile(filename: string, content: string, mime: string): void {
  const blob = new Blob([content], { type: mime });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  URL.revokeObjectURL(url);
}
