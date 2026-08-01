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
export default async function CertificatePrintPage({ searchParams }: CertificatePrintPageProps) {
  const { token, id } = await searchParams;
  const data = token && id ? await getCertificatePrintData(id, token) : null;

  if (!data) {
    return null;
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
