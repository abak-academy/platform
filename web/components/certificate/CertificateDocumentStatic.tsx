import {
  CertificateDocumentCore,
  type CertificateDocumentCoreProps,
  type RenderTextValueArgs,
} from "./CertificateDocumentCore";

// Deliberately NOT "use client". A "use client" module's exports become
// opaque client references when imported from a server-context module graph
// (Next.js: "Attempted to call CertificateDocument() from the server but
// CertificateDocument is on the client") — verified empirically against
// CertificateDocument.tsx before this split existed. This sibling exists so
// app/api/admin/certificate-template/route.tsx can call
// react-dom/server's renderToStaticMarkup directly, once per admin design
// save, to produce the self-contained HTML stored in
// exam.certificate_template_html.
//
// The only behavioral difference from CertificateDocument.tsx is text
// sizing: the browser editor shrinks long text to fit via a DOM-measuring
// hook (useLayoutEffect), which cannot run without a browser. This renders
// every text field at its authored base size instead — a static template
// has no real student data yet (every text token is still literally
// "{{token}}"), so there is nothing meaningful to shrink-fit against here
// regardless.
export type CertificateDocumentStaticProps = Omit<CertificateDocumentCoreProps, "renderTextValue">;

function renderStaticTextValue({ field, content, style }: RenderTextValueArgs) {
  const baseSize = (field.size_pt || 12) * 0.3528 * (96 / 25.4);
  return (
    <span data-testid={`certificate-field-value-${field.id}`} className="block size-full overflow-hidden" style={{ ...style, fontSize: `${baseSize}px` }}>
      {content}
    </span>
  );
}

export function CertificateDocumentStatic(props: CertificateDocumentStaticProps) {
  return <CertificateDocumentCore {...props} renderTextValue={renderStaticTextValue} />;
}
