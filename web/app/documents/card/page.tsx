import { ExamCardPrintable } from "@/components/exam/ExamCardPrintable";
import { getCardPrintData } from "@/lib/server/print-api";

interface CardPrintPageProps {
  searchParams: Promise<{ token?: string; id?: string }>;
}

const DASH = "—";

// This route is fetched by Gotenberg's headless Chromium (FR-28) and, until
// generation runs, is reachable from the public internet with no credential
// beyond the token itself (NFR-S1) — a missing, invalid, expired or
// already-redeemed token must render nothing rather than a partial page or a
// framework error (FR-22, FR-23). It intentionally sits outside
// web/app/(print)/, whose layout is a client component that gates on the
// auth store and would redirect an unauthenticated Gotenberg fetch to
// /login instead of rendering the card. ExamCardPrintable is the single
// markup source shared with the on-screen card at
// web/app/(print)/exam/[id]/card/page.tsx (FR-28).
export default async function CardPrintPage({ searchParams }: CardPrintPageProps) {
  const { token, id } = await searchParams;
  const data = token && id ? await getCardPrintData(id, token) : null;

  if (!data) {
    return null;
  }

  return (
    <ExamCardPrintable
      fullName={data.student_name || DASH}
      participantNumber={data.participant_no || DASH}
      school={data.school || DASH}
      grade={DASH}
      dob={DASH}
      examName={data.exam_title || DASH}
      subject={DASH}
      date={data.exam_schedule || DASH}
      timeRange={DASH}
      duration={DASH}
      mode={DASH}
      platform={DASH}
      checkInCode={data.check_in_code || DASH}
    />
  );
}
