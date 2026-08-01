import type { CertificateLayout } from "@/lib/types";

// INTERNAL_API_BASE_URL is a server-only env var, set in each compose
// manifest's `environment:` block — never NEXT_PUBLIC_API_BASE_URL, which is
// inlined into the browser bundle at build time and points at a host
// reachable from a user's browser, not from the Gotenberg container that
// fetches this print route server-side.
const INTERNAL_API_BASE_URL =
  process.env.INTERNAL_API_BASE_URL ?? "http://localhost:8080/api/v1";

export interface CertificatePrintData {
  layout: CertificateLayout;
  values: Record<string, string>;
  certificate_number: string;
  background_url: string;
  image_urls: Record<string, string>;
}

// getCertificatePrintData redeems a single-use print token against the
// print-data endpoint (backend/internal/handler/print.go). Every failure mode
// — missing id/token, invalid/expired/already-redeemed token, gate denial, a
// non-200 response, or a network error — collapses to null so the caller
// renders the same empty document for all of them (FR-22, FR-23, NFR-S1).
export async function getCertificatePrintData(
  id: string,
  token: string
): Promise<CertificatePrintData | null> {
  if (!id || !token) return null;
  try {
    const res = await fetch(
      `${INTERNAL_API_BASE_URL}/print/certificates/${encodeURIComponent(id)}?token=${encodeURIComponent(token)}`,
      { cache: "no-store" }
    );
    if (!res.ok) return null;
    return (await res.json()) as CertificatePrintData;
  } catch {
    return null;
  }
}

// CardPrintData mirrors CardPrintData (backend/internal/service/exam.go) —
// the exam card print-data endpoint's server-authored response (FR-20).
// grade, dob, subject, duration, mode and platform are still not part of it;
// tenant_name/tenant_logo_url/photo_url/footer_note restore the parity
// buildCardHTML (card_html.go) carried before Task 12 switched GetExamCard
// to this print route (Task 25).
export interface CardPrintData {
  participant_no: string;
  student_name: string;
  school: string;
  exam_title: string;
  exam_schedule: string;
  check_in_code: string;
  tenant_name?: string;
  tenant_logo_url?: string;
  photo_url?: string;
  footer_note?: string;
}

// getCardPrintData redeems a single-use print token against the print-data
// endpoint (backend/internal/handler/print.go, GET /print/cards/:id). Every
// failure mode collapses to null so the caller renders the same empty
// document for all of them (FR-22, FR-23, NFR-S1).
export async function getCardPrintData(
  id: string,
  token: string
): Promise<CardPrintData | null> {
  if (!id || !token) return null;
  try {
    const res = await fetch(
      `${INTERNAL_API_BASE_URL}/print/cards/${encodeURIComponent(id)}?token=${encodeURIComponent(token)}`,
      { cache: "no-store" }
    );
    if (!res.ok) return null;
    return (await res.json()) as CardPrintData;
  } catch {
    return null;
  }
}
