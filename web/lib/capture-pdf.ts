const A4_WIDTH_PT = 595.28;
const A4_HEIGHT_PT = 841.89;

const bytes = (text: string) => new TextEncoder().encode(text);

interface PdfPage {
  jpeg: Uint8Array;
  imgW: number;
  imgH: number;
  pageW: number;
  pageH: number;
}

export function imagePdf(pages: PdfPage[]): Blob {
  const parts: Uint8Array[] = [];
  const offsets: number[] = [];
  let size = 0;
  const push = (chunk: Uint8Array) => {
    parts.push(chunk);
    size += chunk.length;
  };
  const object = (n: number, head: string, stream?: Uint8Array) => {
    offsets[n] = size;
    push(bytes(`${n} 0 obj\n${head}\n`));
    if (stream) {
      push(bytes("stream\n"));
      push(stream);
      push(bytes("\nendstream\n"));
    }
    push(bytes("endobj\n"));
  };
  push(bytes("%PDF-1.4\n"));
  object(1, "<< /Type /Catalog /Pages 2 0 R >>");
  object(2, `<< /Type /Pages /Kids [${pages.map((_, i) => `${3 + i * 3} 0 R`).join(" ")}] /Count ${pages.length} >>`);
  pages.forEach(({ jpeg, imgW, imgH, pageW, pageH }, i) => {
    const page = 3 + i * 3;
    const content = `q ${pageW} 0 0 ${pageH} 0 0 cm /Im0 Do Q`;
    object(page, `<< /Type /Page /Parent 2 0 R /MediaBox [0 0 ${pageW} ${pageH}] /Contents ${page + 1} 0 R /Resources << /XObject << /Im0 ${page + 2} 0 R >> >> >>`);
    object(page + 1, `<< /Length ${content.length} >>`, bytes(content));
    object(page + 2, `<< /Type /XObject /Subtype /Image /Width ${imgW} /Height ${imgH} /ColorSpace /DeviceRGB /BitsPerComponent 8 /Filter /DCTDecode /Length ${jpeg.length} >>`, jpeg);
  });
  const count = 3 + pages.length * 3;
  const xref = size;
  push(bytes(`xref\n0 ${count}\n0000000000 65535 f \n${offsets.slice(1).map((o) => `${String(o).padStart(10, "0")} 00000 n \n`).join("")}trailer\n<< /Size ${count} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`));
  return new Blob(parts as BlobPart[], { type: "application/pdf" });
}

async function captureJpeg(node: HTMLElement, background: string) {
  const { toJpeg } = await import("html-to-image");
  const dataUrl = await toJpeg(node, { quality: 0.95, pixelRatio: 2, backgroundColor: background, cacheBust: false });
  const jpeg = new Uint8Array(await (await fetch(dataUrl)).arrayBuffer());
  const bitmap = await createImageBitmap(new Blob([jpeg as BlobPart], { type: "image/jpeg" }));
  const { width, height } = bitmap;
  bitmap.close();
  return { jpeg, imgW: width, imgH: height };
}

function download(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = fileName;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** Renders the node to one A4 PDF page and saves it; `background` fills what the node leaves transparent. */
export async function savePageAsPdf(node: HTMLElement, fileName: string, background: string) {
  const shot = await captureJpeg(node, background);
  download(imagePdf([{ ...shot, pageW: A4_WIDTH_PT, pageH: A4_HEIGHT_PT }]), fileName);
}
