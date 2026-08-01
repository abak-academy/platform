import { notFound } from "next/navigation";
import { CertificateDocument } from "@/components/certificate/CertificateDocument";
import { getCertificatePrintData } from "@/lib/server/print-api";

interface CertificatePrintPageProps {
  searchParams: Promise<{ token?: string; id?: string }>;
}

// This route is fetched by Gotenberg's headless Chromium (FR-27) and, until
// generation runs, is reachable from the public internet with no credential
// beyond the token itself (NFR-S1) — a missing, invalid, expired or
// already-redeemed token must render nothing rather than a partial page or a
// framework error (FR-22, FR-23). It intentionally sits outside
// web/app/(print)/, whose layout is a client component that gates on the
// auth store and would redirect an unauthenticated Gotenberg fetch to
// /login instead of rendering the certificate.
//
// A failure calls notFound() rather than returning null: a 200 response with
// an empty body is indistinguishable from a successful render to Gotenberg,
// which happily rasterizes it into a valid blank PDF that then gets uploaded
// and cached permanently (NFR-R1). notFound()'s 404 falls inside Gotenberg's
// failOnHttpStatusCodes default ([499,599], i.e. the whole 4xx/5xx range), so
// RenderURL returns an error instead — the body stays empty (still no data
// leak, FR-22/FR-23) but the failure is no longer silent.
export default async function CertificatePrintPage({ searchParams }: CertificatePrintPageProps) {
  const { token, id } = await searchParams;
  const data = token && id ? await getCertificatePrintData(id, token) : null;

  if (!data) {
    notFound();
  }

  return (
    <CertificateDocument
      layout={data.layout}
      values={data.values}
      assetUrls={data.image_urls}
      backgroundUrl={data.background_url}
    />
  );
}
