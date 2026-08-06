import type { UserRole } from "@/lib/nav-config";
import type { ProductType } from "@/lib/types";

const ALL_TYPES: ProductType[] = ["book", "course", "exam", "merchandise", "medal"];
const DIGITAL_TYPES: ProductType[] = ["course", "exam"];

// Mirrors checkTypeRBAC in backend/internal/service/store.go. The server is the
// real boundary — this only decides what to render, so drift here is a cosmetic
// bug, not a hole.
export function writableProductTypes(role: UserRole | undefined): ProductType[] {
  switch (role) {
    case "super_admin":
    case "admin_store":
      return ALL_TYPES;
    case "admin_exam":
      return DIGITAL_TYPES;
    default:
      return [];
  }
}
