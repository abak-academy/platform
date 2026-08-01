import type { User } from "./types";

/** A profile is complete ⟺ a school is known (real `school_id` or a free-text
 *  `unlisted_school_name`) AND grade is non-null/non-empty. Either school
 *  signal is sufficient -- the "unlisted" fallback exists so the gate doesn't
 *  loop forever for students whose school is not in the dropdown.
 *  Grade may arrive as a string or number; both are valid non-empty states. */
export function isProfileComplete(user: User | null | undefined): boolean {
  if (!user) return false;
  const hasSchool = !!user.school_id || !!user.unlisted_school_name;
  const hasGrade = user.grade != null && String(user.grade) !== "";
  return hasSchool && hasGrade;
}

export type ExamBiodataField = "school" | "grade" | "dob";

/** Mirrors the backend's exam-registration biodata gate (school + grade +
 *  dob) so the exam page can warn a student before checkout rejects them.
 *  Stricter than isProfileComplete, which the Google-onboarding gate doesn't
 *  require dob for. */
export function missingExamBiodataFields(
  user: User | null | undefined,
): ExamBiodataField[] {
  if (!user) return ["school", "grade", "dob"];
  const missing: ExamBiodataField[] = [];
  const hasSchool = !!user.school_id || !!user.unlisted_school_name;
  const hasGrade = user.grade != null && String(user.grade) !== "";
  if (!hasSchool) missing.push("school");
  if (!hasGrade) missing.push("grade");
  if (!user.dob) missing.push("dob");
  return missing;
}
