import type { CertificateLayout, CertificateLayoutField } from "./types";

export const CERTIFICATE_TOKENS = [
  ["student_name", "Student name"],
  ["exam_title", "Exam title"],
  ["completion_date", "Completion date"],
  ["certificate_number", "Certificate number"],
  ["score", "Score"],
  ["max_score", "Maximum score"],
  ["score_percent", "Score percentage"],
  ["rank", "Rank"],
  ["percentile", "Percentile"],
  ["duration", "Duration"],
  ["total_questions", "Total questions"],
] as const;

export const PREVIEW_VALUES: Record<string, string> = {
  student_name: "Nama Peserta Contoh",
  exam_title: "Ujian Matematika",
  completion_date: "28 July 2026",
  certificate_number: "ABK/2026/0000/000000",
  score: "86",
  max_score: "100",
  score_percent: "86%",
  rank: "3",
  percentile: "Top 15%",
  duration: "90 minutes",
  total_questions: "50 questions",
};

const DEFAULT_CONTENT: Record<string, string> = {
  title: "CERTIFICATE OF COMPLETION",
  subtitle: "This certificate is proudly awarded to",
  student_name: "{{student_name}}",
  completion_text: "for successfully completing",
  exam_title: "{{exam_title}}",
  date: "{{completion_date}}",
  certificate_number: "{{certificate_number}}",
};

function defaultLayerName(id: string) {
  const name = id.replaceAll("_", " ");
  return name.charAt(0).toUpperCase() + name.slice(1);
}

export function normalizeCertificateLayout(layout: CertificateLayout): CertificateLayout {
  return {
    ...layout,
    fields: layout.fields.map((field) => {
      const kind = field.kind ?? (field.id === "logo" || field.id === "signature" ? "image" : "text");
      return {
        ...field,
        kind,
        name: field.name ?? defaultLayerName(field.id),
        content: kind === "text" ? (field.content || DEFAULT_CONTENT[field.id] || "") : undefined,
        font: kind === "text" ? (field.font || "public_sans") : field.font,
        size_pt: kind === "text" ? (field.size_pt || 12) : field.size_pt,
        color: kind === "text" ? (field.color || "#17213B") : field.color,
        align: field.align || "center",
        asset_key: field.asset_key ?? (field.id === "signature" ? layout.signature_key : null),
      };
    }),
    signature_key: undefined,
  };
}

function id(prefix: "text" | "image") {
  return `${prefix}_${crypto.randomUUID()}`;
}

function overlaps(fields: CertificateLayoutField[], x: number, y: number, width: number, height: number) {
  return fields.some((field) => {
    const fieldHeight = field.kind === "image" ? (field.h_mm || 30) : (field.size_pt || 12) * 0.3528 * 1.15;
    return x < field.x_mm + field.w_mm + 2 &&
      x + width + 2 > field.x_mm &&
      y < field.y_mm + fieldHeight + 2 &&
      y + height + 2 > field.y_mm;
  });
}

function textPosition(fields: CertificateLayoutField[]) {
  for (let y = 10; y <= 190; y += 12) {
    if (!overlaps(fields, 48.5, y, 200, 6)) return { x_mm: 48.5, y_mm: y };
  }
  return { x_mm: 10, y_mm: 10 };
}

function imagePosition(fields: CertificateLayoutField[]) {
  for (let y = 10; y <= 170; y += 40) {
    for (let x = 10; x <= 250; x += 40) {
      if (!overlaps(fields, x, y, 30, 30)) return { x_mm: x, y_mm: y };
    }
  }
  return { x_mm: 10, y_mm: 10 };
}

export function createTextLayer(content: string, name: string, stableID?: string, fields: CertificateLayoutField[] = []): CertificateLayoutField {
  const position = textPosition(fields);
  return {
    id: stableID ?? id("text"),
    kind: "text",
    name,
    content,
    ...position,
    w_mm: 200,
    align: "center",
    font: "public_sans",
    weight: "regular",
    italic: false,
    size_pt: 14,
    color: "#17213B",
    visible: true,
  };
}

export function createImageLayer(assetKey: string, name: string, fields: CertificateLayoutField[] = [], stableID?: string): CertificateLayoutField {
  const position = imagePosition(fields);
  return {
    id: stableID ?? id("image"),
    kind: "image",
    name,
    asset_key: assetKey,
    ...position,
    w_mm: 30,
    h_mm: 30,
    align: "center",
    visible: true,
  };
}

export function moveLayer(fields: CertificateLayoutField[], fieldID: string, direction: "forward" | "backward") {
  const index = fields.findIndex((field) => field.id === fieldID);
  const next = direction === "forward" ? index + 1 : index - 1;
  if (index < 0 || next < 0 || next >= fields.length) return fields;
  const copy = [...fields];
  [copy[index], copy[next]] = [copy[next], copy[index]];
  return copy;
}

export function previewContent(content: string, values = PREVIEW_VALUES) {
  return content.replace(/\{\{([a-z_]+)\}\}/g, (match, token: string) => values[token] ?? match);
}
