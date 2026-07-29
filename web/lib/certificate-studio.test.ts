import { describe, expect, it } from "vitest";
import { createImageLayer, createTextLayer, moveLayer, normalizeCertificateLayout, previewContent } from "./certificate-studio";
import type { CertificateLayout } from "./types";

describe("certificate studio model", () => {
  it("normalizes legacy fields into editable token content", () => {
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [{ id: "student_name", x_mm: 40, y_mm: 80, w_mm: 200, align: "center", visible: true }],
    };
    expect(normalizeCertificateLayout(layout).fields[0]).toMatchObject({
      kind: "text",
      content: "{{student_name}}",
    });
  });

  it("creates and reorders independent image layers", () => {
    const a = createImageLayer("a.png", "Logo");
    const b = createImageLayer("b.png", "Icon", [a]);
    expect(a.id).toMatch(/^image_/);
    expect(a.id).not.toBe(b.id);
    expect([b.x_mm, b.y_mm]).not.toEqual([a.x_mm, a.y_mm]);
    expect(moveLayer([a, b], a.id, "forward")).toEqual([b, a]);
  });

  it("places sequential text layers in open slots", () => {
    const a = createTextLayer("First", "First");
    const b = createTextLayer("Second", "Second", undefined, [a]);
    expect([b.x_mm, b.y_mm]).not.toEqual([a.x_mm, a.y_mm]);
  });

  it("renders multiple preview tokens", () => {
    expect(previewContent("{{student_name}} · {{score_percent}}", {
      student_name: "Budi",
      score_percent: "86%",
    })).toBe("Budi · 86%");
  });
});
