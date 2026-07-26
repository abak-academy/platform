import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { CertificateFieldEditor } from "./CertificateFieldEditor";
import type { CertificateLayout } from "@/lib/types";

// Container is mocked to 1188x840 px so the FR-1 uniform scale
// (mm = px * (page_width_mm / preview_width_px)) works out to an exact
// 0.25mm/px in both axes (1188/297 === 840/210 === 4), keeping the expected
// mm values in each test hand-checkable.
const CONTAINER_WIDTH_PX = 1188;
const CONTAINER_HEIGHT_PX = 840;

function mockContainerRect() {
  vi.spyOn(HTMLDivElement.prototype, "getBoundingClientRect").mockReturnValue({
    width: CONTAINER_WIDTH_PX,
    height: CONTAINER_HEIGHT_PX,
    left: 0,
    top: 0,
    right: CONTAINER_WIDTH_PX,
    bottom: CONTAINER_HEIGHT_PX,
    x: 0,
    y: 0,
    toJSON: () => {},
  });
}

const baseLayout: CertificateLayout = {
  page: { width_mm: 297, height_mm: 210 },
  background: { kind: "builtin", ref: "classic" },
  fields: [
    {
      id: "student_name",
      x_mm: 48.5,
      y_mm: 100,
      w_mm: 200,
      align: "center",
      font: "source_serif_4",
      weight: "bold",
      size_pt: 26,
      color: "#1F2A44",
      visible: true,
    },
    {
      id: "title",
      x_mm: 48.5,
      y_mm: 42,
      w_mm: 200,
      align: "center",
      visible: false,
    },
  ],
};

describe("CertificateFieldEditor", () => {
  beforeEach(() => {
    mockContainerRect();
  });

  it("renders a box only for each visible field", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} />);

    expect(screen.getByTestId("certificate-field-box-student_name")).toBeInTheDocument();
    expect(screen.queryByTestId("certificate-field-box-title")).not.toBeInTheDocument();
  });

  it("dragging a field to the lower-left updates x_mm,y_mm via the FR-1 uniform scale with no Y flip", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} />);

    const box = screen.getByTestId("certificate-field-box-student_name");

    // Grab exactly at the box's current top-left (48.5mm, 100mm) -> (194px, 400px)
    // at 0.25mm/px, so the drag offset within the box is zero.
    fireEvent.pointerDown(box, { pointerId: 1, clientX: 194, clientY: 400 });
    // Move to (80px, 600px) -> (20mm, 150mm): small x (left half), large y
    // (bottom third) - the lower-left quadrant, well clear of the page center.
    // These are the exact x_mm/y_mm reused by the backend raster test
    // (certificate_dnd_editor_test.go) to prove the same drop renders in the
    // lower-left of the PDF, not mirrored to the top (R1).
    fireEvent.pointerMove(box, { pointerId: 1, clientX: 80, clientY: 600 });
    fireEvent.pointerUp(box, { pointerId: 1 });

    expect(onChange).toHaveBeenCalledTimes(1);
    const fields = onChange.mock.calls[0][0];
    const dragged = fields.find((f: { id: string }) => f.id === "student_name");
    expect(dragged.x_mm).toBeCloseTo(20, 5);
    expect(dragged.y_mm).toBeCloseTo(150, 5);
  });

  it("clamps a drop that would place the box outside the page instead of persisting off-page values", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} />);

    const box = screen.getByTestId("certificate-field-box-student_name");

    fireEvent.pointerDown(box, { pointerId: 1, clientX: 194, clientY: 400 });
    // Move way past the bottom-right corner: (1000px, 1000px) -> (250mm, 250mm).
    fireEvent.pointerMove(box, { pointerId: 1, clientX: 1000, clientY: 1000 });
    fireEvent.pointerUp(box, { pointerId: 1 });

    expect(onChange).toHaveBeenCalledTimes(1);
    const fields = onChange.mock.calls[0][0];
    const dragged = fields.find((f: { id: string }) => f.id === "student_name");
    // w_mm=200 on a 297mm-wide page -> max x_mm is 97.
    expect(dragged.x_mm).toBeCloseTo(97, 5);
    // size_pt=26 -> nominal line height 26*0.3528*1.15≈10.5488mm -> max y_mm
    // is 210-10.5488≈199.4513: the box's bottom edge, not its top-left
    // corner, must stay on the page (FR-28).
    expect(dragged.y_mm).toBeCloseTo(199.45128, 3);
  });

  it("derives a text field's clamp from its font size so the whole line stays on the page, not just its top-left corner", () => {
    const onChange = vi.fn();
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [
        {
          id: "certificate_number",
          x_mm: 48.5,
          y_mm: 100,
          w_mm: 200,
          align: "center",
          size_pt: 9,
          visible: true,
        },
      ],
    };
    render(<CertificateFieldEditor layout={layout} onChange={onChange} />);

    const box = screen.getByTestId("certificate-field-box-certificate_number");
    // Grab at (48.5mm,100mm) -> (194px,400px).
    fireEvent.pointerDown(box, { pointerId: 1, clientX: 194, clientY: 400 });
    // Drop right at the bottom edge.
    fireEvent.pointerMove(box, { pointerId: 1, clientX: 194, clientY: 840 });
    fireEvent.pointerUp(box, { pointerId: 1 });

    const fields = onChange.mock.calls[0][0];
    const dragged = fields.find((f: { id: string }) => f.id === "certificate_number");
    // size_pt=9 -> nominal line height 9*0.3528*1.15≈3.6515mm -> max y_mm
    // ≈206.3485, well short of the page's 210mm bottom edge.
    expect(dragged.y_mm).toBeLessThan(210);
    expect(dragged.y_mm).toBeCloseTo(206.34852, 3);
  });

  it("clamps a logo drop against the box's height, not just its y origin", () => {
    const onChange = vi.fn();
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [
        { id: "logo", x_mm: 138.5, y_mm: 15, w_mm: 20, h_mm: 20, align: "center", visible: true },
      ],
    };
    render(<CertificateFieldEditor layout={layout} onChange={onChange} />);

    const box = screen.getByTestId("certificate-field-box-logo");
    // (138.5mm,15mm) -> (554px,60px)
    fireEvent.pointerDown(box, { pointerId: 1, clientX: 554, clientY: 60 });
    // Drop far past the bottom edge.
    fireEvent.pointerMove(box, { pointerId: 1, clientX: 554, clientY: 1000 });
    fireEvent.pointerUp(box, { pointerId: 1 });

    const fields = onChange.mock.calls[0][0];
    const dragged = fields.find((f: { id: string }) => f.id === "logo");
    // h_mm=20 on a 210mm-tall page -> max y_mm is 190.
    expect(dragged.y_mm).toBeCloseTo(190, 5);
  });

  it("renders the certificate artwork as the drag surface instead of an empty rectangle", () => {
    const onChange = vi.fn();
    render(
      <CertificateFieldEditor
        layout={baseLayout}
        onChange={onChange}
        backgroundUrl="https://cdn.example.com/cert-bg.png"
      />,
    );

    const bg = screen.getByTestId("certificate-field-editor-background") as HTMLImageElement;
    expect(bg.src).toBe("https://cdn.example.com/cert-bg.png");
  });

  it("falls back to a plain surface when no background is available yet", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} />);

    expect(screen.queryByTestId("certificate-field-editor-background")).not.toBeInTheDocument();
  });

  it("renders the field's actual value styled per its font/size/color/align instead of the dashed label (FR-16 WYSIWYG)", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} examTitle="Ujian Matematika" />);

    const box = screen.getByTestId("certificate-field-box-student_name");
    const value = screen.getByTestId("certificate-field-value-student_name");
    expect(value.textContent).toBe("Nama Peserta Contoh");
    expect(box.textContent).not.toContain("Nama Siswa");
    expect(value.style.fontFamily).toBe("source_serif_4");
    expect(value.style.fontWeight).toBe("700");
    expect(value.style.fontSize).toBe("26pt");
    expect(value.style.color).toBe("rgb(31, 42, 68)");
    expect(value.style.textAlign).toBe("center");
  });

  it("falls back to the default brand font when a field's font is unknown or unset (FR-9)", () => {
    const onChange = vi.fn();
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [
        { id: "exam_title", x_mm: 48.5, y_mm: 139, w_mm: 200, align: "center", font: "papyrus", size_pt: 15, visible: true },
      ],
    };
    render(<CertificateFieldEditor layout={layout} onChange={onChange} />);

    const value = screen.getByTestId("certificate-field-value-exam_title");
    expect(value.style.fontFamily).toBe("source_serif_4");
  });

  it("falls back to black when a field's color is malformed (FR-9)", () => {
    const onChange = vi.fn();
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [
        { id: "date", x_mm: 48.5, y_mm: 162, w_mm: 200, align: "center", color: "not-a-color", visible: true },
      ],
    };
    render(<CertificateFieldEditor layout={layout} onChange={onChange} />);

    const value = screen.getByTestId("certificate-field-value-date");
    expect(value.style.color).toBe("rgb(0, 0, 0)");
  });

  it("keeps image fields (logo/signature) showing their label, not styled text, since they have no text value", () => {
    const onChange = vi.fn();
    const layout: CertificateLayout = {
      page: { width_mm: 297, height_mm: 210 },
      background: { kind: "builtin", ref: "classic" },
      fields: [
        { id: "logo", x_mm: 138.5, y_mm: 15, w_mm: 20, h_mm: 20, align: "center", visible: true },
      ],
    };
    render(<CertificateFieldEditor layout={layout} onChange={onChange} />);

    expect(screen.queryByTestId("certificate-field-value-logo")).not.toBeInTheDocument();
    expect(screen.getByTestId("certificate-field-box-logo").textContent).toBe("Logo");
  });

  it("lets the position be set via the numeric mm inputs as a non-drag alternative", () => {
    const onChange = vi.fn();
    render(<CertificateFieldEditor layout={baseLayout} onChange={onChange} />);

    const xInput = screen.getByLabelText(/x.*student_name|student.*x/i, { exact: false }) as HTMLInputElement | null;
    // Fall back to a broader query if the exact accessible name doesn't match;
    // the important behavioral contract is that *some* non-drag input exists
    // and commits a clamped value.
    const input =
      xInput ?? (screen.getAllByRole("spinbutton")[0] as HTMLInputElement);

    fireEvent.change(input, { target: { value: "500" } });

    expect(onChange).toHaveBeenCalled();
    const fields = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    const dragged = fields.find((f: { id: string }) => f.id === "student_name");
    expect(dragged.x_mm).toBeLessThanOrEqual(97);
  });
});
