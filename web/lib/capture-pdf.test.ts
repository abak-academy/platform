import { describe, expect, it } from "vitest";
import { imagePdf } from "./capture-pdf";

const A4 = { pageW: 595.28, pageH: 841.89 };

async function build() {
  const jpeg = new Uint8Array([0xff, 0xd8, 0xff, 0xd9]);
  const blob = imagePdf([{ jpeg, imgW: 1240, imgH: 1754, ...A4 }]);
  return { blob, text: new TextDecoder("latin1").decode(await blob.arrayBuffer()) };
}

describe("imagePdf", () => {
  it("emits a one-page A4 PDF carrying the JPEG", async () => {
    const { blob, text } = await build();
    expect(blob.type).toBe("application/pdf");
    expect(text.startsWith("%PDF-1.4")).toBe(true);
    expect(text.trimEnd().endsWith("%%EOF")).toBe(true);
    expect(text).toContain("/Count 1");
    expect(text.match(/\/Type \/Page /g)).toHaveLength(1);
    expect(text).toContain("/MediaBox [0 0 595.28 841.89]");
    expect(text).toContain("/Filter /DCTDecode");
    expect(text).toContain("/Width 1240 /Height 1754");
  });

  it("points startxref at the xref table", async () => {
    const { text } = await build();
    const offset = Number(/startxref\n(\d+)/.exec(text)![1]);
    expect(text.slice(offset, offset + 4)).toBe("xref");
  });
});
