// Canonical grade (kelas) values.
//
// users.grade is an INT column, but the product spans TK through perguruan
// tinggi, so a few levels get fixed numeric codes: 0 = TK and 13–15 = S1–S3.
// Grades 1–12 keep their ordinary school meaning (SD 1–6, SMP 7–9,
// SMA/SMK/MA 10–12). The backend stores and filters the raw integer; this
// module is the single source of truth for what the UI offers and displays.
export const GRADE_TK = 0;
export const GRADE_S1 = 13;
export const GRADE_S2 = 14;
export const GRADE_S3 = 15;

const TK_GRADES = [GRADE_TK];
const SD_GRADES = [1, 2, 3, 4, 5, 6];
const SMP_GRADES = [7, 8, 9];
const SMA_GRADES = [10, 11, 12];
const PT_GRADES = [GRADE_S1, GRADE_S2, GRADE_S3];

const GRADE_LABELS: Record<number, string> = {
  [GRADE_TK]: "TK",
  [GRADE_S1]: "S1",
  [GRADE_S2]: "S2",
  [GRADE_S3]: "S3",
};

// Jenjang (or school type) to the grade range its students sit in.
const JENJANG_GRADES: Record<string, number[]> = {
  TK: TK_GRADES,
  RA: TK_GRADES,
  KB: TK_GRADES,
  TPA: TK_GRADES,
  PAUDQ: TK_GRADES,
  SD: SD_GRADES,
  SDLB: SD_GRADES,
  SLB: SD_GRADES,
  MI: SD_GRADES,
  SMP: SMP_GRADES,
  MTS: SMP_GRADES,
  SMA: SMA_GRADES,
  MA: SMA_GRADES,
  SMK: SMA_GRADES,
  D1: PT_GRADES,
  D2: PT_GRADES,
  D3: PT_GRADES,
  S1: PT_GRADES,
  S2: PT_GRADES,
  S3: PT_GRADES,
};

// Fallback for jenjang outside the map (PKBM, LKP, Kursus, unknown): the full
// TK–S3 span, grouped so a flat list stays readable.
export interface GradeGroup {
  label: string;
  values: number[];
}

export const GRADE_GROUPS: GradeGroup[] = [
  { label: "TK / PAUD", values: TK_GRADES },
  { label: "SD", values: SD_GRADES },
  { label: "SMP", values: SMP_GRADES },
  { label: "SMA / SMK", values: SMA_GRADES },
  { label: "Perguruan Tinggi", values: PT_GRADES },
];

export function formatGrade(grade: number | null | undefined): string {
  if (grade == null) return "";
  return GRADE_LABELS[grade] ?? String(grade);
}

// Returns [] when the jenjang has no fixed range — callers fall back to
// GRADE_GROUPS.
export function gradeOptionsForJenjang(jenjang: string | null | undefined): number[] {
  const key = (jenjang ?? "").trim().toUpperCase();
  return JENJANG_GRADES[key] ?? [];
}
