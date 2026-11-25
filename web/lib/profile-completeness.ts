import type { User } from "@/lib/types";

// Mirrors the backend's biodataMissingFields (backend/internal/service/store.go)
// so the cart can decide whether to show the complete-profile gate before
// checkout ever rejects it with biodata_incomplete.
export type MissingProfileField = "name" | "email" | "phone" | "school" | "grade" | "dob";

export function missingProfileFields(profile: User | undefined | null): MissingProfileField[] {
  if (!profile) return ["name", "email", "phone", "school", "grade", "dob"];
  const missing: MissingProfileField[] = [];
  if (!profile.name?.trim()) missing.push("name");
  if (!profile.email?.trim()) missing.push("email");
  if (!profile.phone?.trim()) missing.push("phone");
  const hasSchool = Boolean(profile.school_id) || Boolean(profile.unlisted_school_name?.trim());
  if (!hasSchool) missing.push("school");
  if (profile.grade == null) missing.push("grade");
  if (!profile.dob) missing.push("dob");
  return missing;
}

export function isProfileComplete(profile: User | undefined | null): boolean {
  return missingProfileFields(profile).length === 0;
}
