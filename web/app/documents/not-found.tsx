// Scoped to app/documents/ (not a root not-found.tsx): the certificate/card
// print routes call notFound() on any failure (FR-22, FR-23) and this route
// is what Gotenberg's headless Chromium rasterizes into a PDF — Next's
// default 404 boundary renders visible boilerplate text, which would end up
// as ink on the "blank" PDF. Rendering nothing here keeps the body empty
// while still returning Next's 404 status code (NFR-R1, NFR-S1).
export default function DocumentsNotFound() {
  return null;
}
