import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { CertificateDocument } from "./CertificateDocument";
import type { CertificateLayout } from "@/lib/types";

const layout: CertificateLayout = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [
    {
      id: "student_name",
      x_mm: 40,
      y_mm: 60,
      w_mm: 200,
      align: "center",
      size_pt: 26,
      visible: true,
    },
  ],
};

describe("CertificateDocument", () => {
  it("renders a field at x_mm/y_mm as real mm CSS units (FR-25)", () => {
    render(<CertificateDocument layout={layout} values={{}} />);

    const box = screen.getByTestId("certificate-field-box-student_name");
    expect(box.style.left).toBe("40mm");
    expect(box.style.top).toBe("60mm");
  });

  it("sizes the page in real mm and carries an @page rule for print", () => {
    render(<CertificateDocument layout={layout} values={{}} />);

    const page = screen.getByTestId("certificate-document-page");
    expect(page.style.width).toBe("297mm");
    expect(page.style.height).toBe("210mm");
    expect(page.innerHTML).toContain("@page { size: 297mm 210mm; margin: 0; }");
  });

  it("is non-interactive by default: no pointer/keyboard handlers, not tabbable", () => {
    render(<CertificateDocument layout={layout} values={{}} />);

    const box = screen.getByTestId("certificate-field-box-student_name");
    expect(box.tabIndex).toBe(-1);
  });
});
