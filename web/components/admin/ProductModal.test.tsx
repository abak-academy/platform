import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ProductModal } from "./ProductModal";
import type { Product } from "@/lib/types";

const mockOnSubmit = vi.fn();
const mockOnOpenChange = vi.fn();

const sampleCourses = [
  { id: "c1", title: "Fisika Dasar" },
  { id: "c2", title: "Matematika Lanjut" },
];

const sampleExams = [
  { id: "e1", title: "UTBK 2026" },
  { id: "e2", title: "Tryout SNBT" },
];

let coursesState: { data: typeof sampleCourses | undefined };
let examsState: { data: { data: typeof sampleExams } | undefined };

const mockPresign = vi.fn();

vi.mock("@/lib/hooks/admin-courses", () => ({
  useAdminCourses: () => coursesState,
}));

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: () => examsState,
}));

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({ mutateAsync: mockPresign }),
}));

describe("ProductModal", () => {
  beforeEach(() => {
    mockOnSubmit.mockReset();
    mockOnOpenChange.mockReset();
    coursesState = { data: sampleCourses };
    examsState = { data: { data: sampleExams } };
    mockPresign.mockReset();
    mockPresign.mockResolvedValue({ url: "https://upload.example", method: "PUT", key: "avatars/u/img.png" });
    global.fetch = vi.fn().mockResolvedValue({ ok: true, status: 200 }) as unknown as typeof fetch;
  });

  // --- create mode ---

  it("shows course checkboxes when type is course in create mode", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByLabelText(/jenis/i)).toHaveValue("");

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "course" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Course" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "100000" } });

    expect(screen.getByText(/kursus terkait/i)).toBeInTheDocument();
    expect(screen.getByText("Fisika Dasar")).toBeInTheDocument();
    expect(screen.getByText("Matematika Lanjut")).toBeInTheDocument();
  });

  it("hides course checkboxes when type is book in create mode", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "book" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Buku A" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "50000" } });

    expect(screen.queryByText(/kursus terkait/i)).not.toBeInTheDocument();
  });

  it("includes course_ids in create payload for course type", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Video Course" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "200000" } });
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "course" } });

    const fisikaCheckbox = screen.getByText("Fisika Dasar").closest("label")!.querySelector("input[type=checkbox]")!;
    fireEvent.click(fisikaCheckbox);

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Video Course",
          price: 200000,
          type: "course",
          course_ids: ["c1"],
        })
      );
    });
  });

  it("excludes course_ids from create payload for book type", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Buku A" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "50000" } });
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "book" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Buku A", price: 50000, type: "book" })
      );
      const payload = mockOnSubmit.mock.calls[0][0];
      expect(payload).not.toHaveProperty("course_ids");
    });
  });

  // --- edit mode ---

  it("populates course_ids from product in edit mode", () => {
    const product: Product = {
      id: "p1",
      type: "course",
      name: "Kursus IPA",
      price: 150000,
      status: "published",
      course_ids: ["c1"],
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByDisplayValue("Kursus IPA")).toBeInTheDocument();

    const fisikaCheckbox = screen.getByText("Fisika Dasar").closest("label")!.querySelector("input[type=checkbox]")!;
    expect(fisikaCheckbox).toBeChecked();

    const matematikaCheckbox = screen.getByText("Matematika Lanjut").closest("label")!.querySelector("input[type=checkbox]")!;
    expect(matematikaCheckbox).not.toBeChecked();
  });

  it("includes course_ids in update payload for course type", async () => {
    const product: Product = {
      id: "p1",
      type: "course",
      name: "Kursus IPA",
      price: 150000,
      status: "published",
      course_ids: ["c1", "c2"],
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Kursus IPA Updated" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Kursus IPA Updated",
          course_ids: ["c1", "c2"],
        })
      );
    });
  });

  it("excludes course_ids from update payload for book type", async () => {
    const product: Product = {
      id: "p1",
      type: "book",
      name: "Buku IPA",
      price: 75000,
      stock: 10,
      status: "published",
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Buku IPA Rev 2" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Buku IPA Rev 2" })
      );
      const payload = mockOnSubmit.mock.calls[0][0];
      expect(payload).not.toHaveProperty("course_ids");
    });
  });

  it("shows empty state when no courses exist", () => {
    coursesState = { data: [] };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "course" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Empty Course" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "99999" } });

    expect(screen.getByText("Belum ada kursus.")).toBeInTheDocument();
  });

  // --- exam attach (mirrors course attach) ---

  it("shows exam checkboxes when type is exam in create mode", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Ujian" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "100000" } });

    expect(screen.getByText(/ujian terkait/i)).toBeInTheDocument();
    expect(screen.getByText("UTBK 2026")).toBeInTheDocument();
    expect(screen.getByText("Tryout SNBT")).toBeInTheDocument();
    expect(screen.queryByText(/kursus terkait/i)).not.toBeInTheDocument();
  });

  it("requires at least one exam checked before submit is enabled for exam type", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Ujian" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "100000" } });

    expect(screen.getByRole("button", { name: /^simpan$/i })).toBeDisabled();

    const utbkCheckbox = screen.getByText("UTBK 2026").closest("label")!.querySelector("input[type=checkbox]")!;
    fireEvent.click(utbkCheckbox);

    expect(screen.getByRole("button", { name: /^simpan$/i })).toBeEnabled();
  });

  it("includes exam_ids in create payload for exam type", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Ujian" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "150000" } });
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });

    const utbkCheckbox = screen.getByText("UTBK 2026").closest("label")!.querySelector("input[type=checkbox]")!;
    fireEvent.click(utbkCheckbox);

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Paket Ujian",
          price: 150000,
          type: "exam",
          exam_ids: ["e1"],
        })
      );
    });
  });

  it("populates exam_ids from product in edit mode", () => {
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
      status: "published",
      exam_ids: ["e1"],
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const utbkCheckbox = screen.getByText("UTBK 2026").closest("label")!.querySelector("input[type=checkbox]")!;
    expect(utbkCheckbox).toBeChecked();

    const tryoutCheckbox = screen.getByText("Tryout SNBT").closest("label")!.querySelector("input[type=checkbox]")!;
    expect(tryoutCheckbox).not.toBeChecked();
  });

  it("includes exam_ids in update payload for exam type", async () => {
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
      status: "published",
      exam_ids: ["e1", "e2"],
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket UTBK Updated" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Paket UTBK Updated",
          exam_ids: ["e1", "e2"],
        })
      );
    });
  });

  it("shows empty state when no exams exist", () => {
    examsState = { data: { data: [] } };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Empty Exam" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "99999" } });

    expect(screen.getByText("Belum ada ujian.")).toBeInTheDocument();
  });

  // --- merchandise (physical) ---

  it("offers merchandise and shows stock, weight, and image fields when selected", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("option", { name: "Merchandise" })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "merchandise" } });

    expect(screen.getByLabelText(/stok/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/berat/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/gambar/i)).toBeInTheDocument();
  });

  it("offers medal and shows stock, weight, and image fields when selected", () => {
    render(<ProductModal open={true} onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false} />);

    expect(screen.getByRole("option", { name: "Medali" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "medal" } });
    expect(screen.getByLabelText(/stok/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/berat/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/gambar/i)).toBeInTheDocument();
  });

  it("uploads image and includes merchandise fields in create payload", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "merchandise" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Kaos Logo" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "75000" } });
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "20" } });
    fireEvent.input(screen.getByLabelText(/berat/i), { target: { value: "250" } });

    const file = new File(["x"], "img.png", { type: "image/png" });
    fireEvent.change(screen.getByLabelText(/gambar/i), { target: { files: [file] } });

    await waitFor(() => expect(mockPresign).toHaveBeenCalled());
    expect(mockPresign).toHaveBeenCalledWith(
      expect.objectContaining({ kind: "product" })
    );
    const preview = (await screen.findByAltText(/pratinjau/i)) as HTMLImageElement;
    expect(preview.src).toContain("avatars/u/img.png");

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Kaos Logo",
          price: 75000,
          type: "merchandise",
          stock: 20,
          weight_grams: 250,
          image_url: "avatars/u/img.png",
        })
      );
    });
  });

  it("renders existing image preview and preserves untouched image_url on edit", async () => {
    const product: Product = {
      id: "p1",
      type: "book",
      name: "Buku IPA",
      price: 75000,
      stock: 10,
      status: "published",
      image_url: "avatars/u/old.png",
    };

    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        product={product}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const preview = screen.getByAltText(/pratinjau/i) as HTMLImageElement;
    expect(preview.src).toContain("avatars/u/old.png");

    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Buku IPA v2" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Buku IPA v2", image_url: "avatars/u/old.png" })
      );
    });
  });

  it("renders at the wider dialog width", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("dialog").className).toContain("max-w-2xl");
  });
});
