"use client";

import { useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import {
  CertificateDocumentCore,
  type CertificateDocumentCoreProps,
  type CertificateFieldInteraction,
  type RenderTextValueArgs,
} from "./CertificateDocumentCore";
import type { CertificateLayoutField } from "@/lib/types";

export type { CertificateFieldInteraction };
export type CertificateDocumentProps = Omit<CertificateDocumentCoreProps, "renderTextValue">;

function CertificateTextValue({ field, content, pxPerMm, style }: {
  field: CertificateLayoutField;
  content: string;
  pxPerMm: number;
  style: CSSProperties;
}) {
  const ref = useRef<HTMLSpanElement>(null);
  const baseSize = (field.size_pt || 12) * 0.3528 * pxPerMm;
  const [fontSize, setFontSize] = useState(baseSize);

  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return;
    element.style.fontSize = `${baseSize}px`;
    const available = element.clientWidth;
    const needed = element.scrollWidth;
    const minimum = 6 * 0.3528 * pxPerMm;
    setFontSize(available > 0 && needed > available ? Math.max(minimum, baseSize * available / needed * 0.97) : baseSize);
  }, [baseSize, content, field.font, field.italic, field.weight, pxPerMm]);

  return <span ref={ref} data-testid={`certificate-field-value-${field.id}`} className="block size-full overflow-hidden" style={{ ...style, fontSize: `${fontSize}px` }}>{content}</span>;
}

function renderTextValue(args: RenderTextValueArgs) {
  return <CertificateTextValue {...args} />;
}

// The live, interactive renderer: browser-only (shrink-to-fit needs a real
// DOM to measure), used by the admin editor and any on-screen preview.
// CertificateDocumentCore.tsx carries the shared layout/positioning logic;
// CertificateDocumentStatic.tsx is this component's server-safe sibling, used
// by the certificate-template route handler to serialize the self-contained
// HTML the async redesign stores (see that file's comment for why "use
// client" here can't simply be reused from a Route Handler).
export function CertificateDocument(props: CertificateDocumentProps) {
  return <CertificateDocumentCore {...props} renderTextValue={renderTextValue} />;
}
