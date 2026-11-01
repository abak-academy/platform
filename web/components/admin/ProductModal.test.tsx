import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { ProductModal } from "./ProductModal";
import { ApiError } from "@/lib/api";
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
const mockUseExams = vi.fn();

vi.mock("@/lib/hooks/admin-courses", () => ({
  useAdminCourses: () => coursesState,
}));

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: (filter?: { q?: string }) => mockUseExams(filter),
}));

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({ mutateAsync: mockPresign }),
}));

let detailState: { data: Product | undefined; isPending: boolean; isError: boolean };
const mockDetailRefetch = vi.fn();
const mockChangeRelations = vi.fn();
const mockUseAdminProduct = vi.fn();
vi.mock("@/lib/hooks/admin-products", () => ({
  useAdminProduct: (id?: string, enabled?: boolean) => {
    mockUseAdminProduct(id, enabled);
    return { ...detailState, refetch: mockDetailRefetch };
  },
  useUpdateProductRelations: () => ({ mutateAsync: mockChangeRelations, isPending: false }),
}));

let mockRole: string | undefined = "admin_store";

vi.mock("@/lib/hooks/use-capability", async (importActual) => ({
  ...(await importActual<typeof import("@/lib/hooks/use-capability")>()),
  useResolvedRole: () => ({ role: mockRole, hydrated: true, meIsError: false }),
}));

describe("ProductModal", () => {
  beforeEach(() => {
    mockRole = "admin_store";
    detailState = { data: { id: "p1", type: "exam", name: "Detail", price: 150000, status: "published", exam_ids: ["e1"] }, isPending: false, isError: false };
    mockDetailRefetch.mockReset();
    mockChangeRelations.mockReset();
    mockChangeRelations.mockResolvedValue(detailState.data);
    mockUseAdminProduct.mockReset();
    mockOnSubmit.mockReset();
    mockOnOpenChange.mockReset();
    coursesState = { data: sampleCourses };
    examsState = { data: { data: sampleExams } };
    mockUseExams.mockReset();
    mockUseExams.mockImplementation(() => examsState);
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

  it("shows read-only course_ids from detail in edit mode", () => {
    detailState.data = { id: "p1", type: "course", name: "Detail", price: 150000, status: "published", course_ids: ["c1"], relations: [{ id: "c1", title: "Fisika Dasar" }] };
    const product: Product = {
      id: "p1",
      type: "course",
      name: "Kursus IPA",
      price: 150000,
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

    expect(screen.getByDisplayValue("Kursus IPA")).toBeInTheDocument();

    expect(screen.getByText("Fisika Dasar")).toBeInTheDocument();
    expect(screen.queryByText("Matematika Lanjut")).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("excludes all relation keys from update payload for course type", async () => {
    detailState.data = { id: "p1", type: "course", name: "Detail", price: 150000, status: "published", course_ids: ["c1"] };
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
        })
      );
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("course_ids");
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("exam_ids");
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

  it("shows read-only exam_ids from detail in edit mode", () => {
    detailState.data = { ...detailState.data!, relations: [{ id: "e1", title: "UTBK 2026" }] };
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
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

    expect(screen.getByText("UTBK 2026")).toBeInTheDocument();
    expect(screen.queryByText("Tryout SNBT")).not.toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("excludes all relation keys from update payload for exam type", async () => {
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
        })
      );
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("course_ids");
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("exam_ids");
    });
  });

  it("searches exams with a debounced q and keeps selections across result changes", async () => {
    mockUseExams.mockImplementation((filter?: { q?: string }) =>
      filter?.q === "snbt"
        ? { data: { data: [{ id: "e3", title: "SNBT Paket C" }] } }
        : examsState,
    );

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
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "99999" } });
    fireEvent.click(screen.getByLabelText("UTBK 2026"));
    expect(screen.getByText("1 dipilih")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Cari ujian..."), { target: { value: "snbt" } });
    expect(mockUseExams).not.toHaveBeenCalledWith({ q: "snbt" });

    expect(await screen.findByText("SNBT Paket C")).toBeInTheDocument();
    expect(mockUseExams).toHaveBeenCalledWith({ q: "snbt" });
    expect(screen.queryByText("UTBK 2026")).not.toBeInTheDocument();
    expect(screen.getByText("1 dipilih")).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("SNBT Paket C"));
    expect(screen.getByText("2 dipilih")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Cari ujian..."), { target: { value: "" } });
    await waitFor(() => {
      expect(screen.getByLabelText("UTBK 2026")).toBeChecked();
    });
    expect(screen.getByLabelText("Tryout SNBT")).not.toBeChecked();

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ exam_ids: ["e1", "e3"] })
      );
    });
  });

  it("does not show the selected count when editing a product", () => {
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
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

    expect(screen.queryByPlaceholderText("Cari ujian...")).not.toBeInTheDocument();
    expect(screen.queryByText(/dipilih/)).not.toBeInTheDocument();
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

    // Wait for the upload to be APPLIED, not merely started: presign is
    // followed by a second await (the PUT) before setImageUrl runs, so waiting
    // on "presign was called" lets Simpan fire while imageUrl is still empty —
    // onSubmit then commits once, without image_url, and never fires again.
    await waitFor(() => expect(screen.getByAltText("Pratinjau gambar")).toBeInTheDocument());
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

  // --- digital product image (exam/course) ---

  it("defaults the exam quota switch off and hides stock and weight", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });

    expect(screen.getByLabelText(/gambar/i)).toBeInTheDocument();
    expect(screen.getByRole("switch", { name: "Batasi kuota" })).toHaveAttribute("aria-checked", "false");
    expect(screen.queryByLabelText(/stok/i)).not.toBeInTheDocument();
    expect(screen.queryByLabelText(/berat/i)).not.toBeInTheDocument();
  });

  it("includes image_url and stock:null but excludes weight_grams in create payload for exam type", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Ujian Bergambar" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "150000" } });

    const utbkCheckbox = screen.getByText("UTBK 2026").closest("label")!.querySelector("input[type=checkbox]")!;
    fireEvent.click(utbkCheckbox);

    const file = new File(["x"], "img.png", { type: "image/png" });
    fireEvent.change(screen.getByLabelText(/gambar/i), { target: { files: [file] } });
    // Wait for the upload to be APPLIED, not merely started: presign is
    // followed by a second await (the PUT) before setImageUrl runs, so waiting
    // on "presign was called" lets Simpan fire while imageUrl is still empty —
    // onSubmit then commits once, without image_url, and never fires again.
    await waitFor(() => expect(screen.getByAltText("Pratinjau gambar")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Paket Ujian Bergambar",
          type: "exam",
          image_url: "avatars/u/img.png",
        })
      );
      const payload = mockOnSubmit.mock.calls[0][0];
      expect(payload).toHaveProperty("stock", null);
      expect(payload).not.toHaveProperty("weight_grams");
    });
  });

  it("sends a number for a filled exam stock in create mode", async () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Paket Ujian Kuota" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "150000" } });
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    expect(screen.getByRole("switch", { name: "Batasi kuota" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByLabelText(/stok/i)).toBeRequired();
    expect(screen.getByLabelText(/stok/i)).toHaveAttribute("min", "0");
    expect(screen.getByLabelText(/stok/i)).toHaveAttribute("step", "1");
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "20" } });

    const utbkCheckbox = screen.getByText("UTBK 2026").closest("label")!.querySelector("input[type=checkbox]")!;
    fireEvent.click(utbkCheckbox);

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ type: "exam", stock: 20 })
      );
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("weight_grams");
    });
  });

  it("sends stock:null on update for an unlimited exam with the switch off", async () => {
    detailState.data = { id: "p1", type: "exam", name: "Detail", price: 150000, status: "published", exam_ids: ["e1"] };
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
      stock: 0,
      stock_limited: false,
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

    expect(screen.getByRole("switch", { name: "Batasi kuota" })).toHaveAttribute("aria-checked", "false");
    expect(screen.queryByLabelText(/stok/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ stock: null })
      );
      expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("weight_grams");
    });
  });

  it("keeps a limited exam's switch on and submits its prefilled stock on edit", () => {
    detailState.data = { id: "p1", type: "exam", name: "Detail", price: 150000, status: "published", exam_ids: ["e1"] };
    const product: Product = {
      id: "p1",
      type: "exam",
      name: "Paket UTBK",
      price: 150000,
      stock: 0,
      stock_limited: true,
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

    expect(screen.getByRole("switch", { name: "Batasi kuota" })).toHaveAttribute("aria-checked", "true");
    expect(screen.getByLabelText(/stok/i)).toHaveValue(0);
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ stock: 0 }));
  });

  it.each(["", "0", "-1", "1.5"])("validates an enabled exam quota %j in create mode", (quota) => {
    render(<ProductModal open onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false} />);
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Exam quota" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "150000" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /UTBK 2026/i }));
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: quota } });
    const save = screen.getByRole("button", { name: /^simpan$/i });
    if (quota === "0") {
      expect(save).toBeEnabled();
      fireEvent.click(save);
      expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ stock: 0 }));
    } else {
      expect(save).toBeDisabled();
      fireEvent.submit(save.closest("form")!);
      expect(mockOnSubmit).not.toHaveBeenCalled();
    }
  });

  it("rejects empty limited stock on edit and sends null after switching off", () => {
    const product: Product = { id: "p1", type: "exam", name: "Exam quota", price: 150000, stock: 12, stock_limited: true, status: "published" };
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={product} onSubmit={mockOnSubmit} isPending={false} />);
    expect(screen.getByLabelText(/stok/i)).toHaveValue(12);
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ stock: 12 }));
    mockOnSubmit.mockClear();
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "" } });
    const save = screen.getByRole("button", { name: /^simpan$/i });
    expect(save).toBeDisabled();
    fireEvent.submit(save.closest("form")!);
    expect(mockOnSubmit).not.toHaveBeenCalled();
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "12" } });
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    expect(screen.queryByLabelText(/stok/i)).not.toBeInTheDocument();
    fireEvent.click(save);
    expect(mockOnSubmit).toHaveBeenCalledWith(expect.objectContaining({ stock: null }));
  });

  it.each(["book", "merchandise", "medal"])("keeps %s stock plain and clears quota when changing types", (type) => {
    render(<ProductModal open onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false} />);
    fireEvent.input(screen.getByLabelText(/nama/i), { target: { value: "Product" } });
    fireEvent.input(screen.getByLabelText(/harga/i), { target: { value: "150000" } });
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    fireEvent.click(screen.getByRole("checkbox", { name: /UTBK 2026/i }));
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "20" } });
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    fireEvent.click(screen.getByRole("switch", { name: "Batasi kuota" }));
    expect(screen.getByLabelText(/stok/i)).toHaveValue(null);
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "20" } });
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: type } });
    expect(screen.queryByRole("switch", { name: "Batasi kuota" })).not.toBeInTheDocument();
    expect(screen.getByLabelText(/stok/i)).toHaveValue(null);
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    expect(mockOnSubmit).toHaveBeenLastCalledWith(expect.objectContaining({ type, stock: 0 }));
    fireEvent.input(screen.getByLabelText(/stok/i), { target: { value: "7" } });
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    expect(mockOnSubmit).toHaveBeenLastCalledWith(expect.objectContaining({ type, stock: 7 }));
    fireEvent.change(screen.getByLabelText(/jenis/i), { target: { value: "exam" } });
    expect(screen.getByRole("switch", { name: "Batasi kuota" })).toHaveAttribute("aria-checked", "false");
    expect(screen.queryByLabelText(/stok/i)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));
    expect(mockOnSubmit).toHaveBeenLastCalledWith(expect.objectContaining({ type: "exam", stock: null }));
  });

  it("includes image_url in update payload for an existing course product", async () => {
    detailState.data = { id: "p1", type: "course", name: "Detail", price: 150000, status: "published", course_ids: ["c1"] };
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

    const file = new File(["x"], "cover.png", { type: "image/png" });
    fireEvent.change(screen.getByLabelText(/gambar/i), { target: { files: [file] } });
    // Wait for the upload to be APPLIED, not merely started: presign is
    // followed by a second await (the PUT) before setImageUrl runs, so waiting
    // on "presign was called" lets Simpan fire while imageUrl is still empty —
    // onSubmit then commits once, without image_url, and never fires again.
    await waitFor(() => expect(screen.getByAltText("Pratinjau gambar")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Kursus IPA",
          image_url: "avatars/u/img.png",
        })
      );
      const payload = mockOnSubmit.mock.calls[0][0];
      expect(payload).not.toHaveProperty("stock");
      expect(payload).not.toHaveProperty("weight_grams");
    });
  });

  // A book carries eight specification rows. Without a bounded, scrolling body
  // the dialog grew past the viewport and took the title and the Save button
  // off screen with it — the form could be filled in but not submitted.
  it("keeps the header and footer reachable when the form is long", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    const dialog = screen.getByRole("dialog");
    expect(dialog.className).toContain("overflow-hidden");
    expect(dialog.className).toMatch(/max-h-\[/);
    expect(dialog.querySelector(".overflow-y-auto")).not.toBeNull();
    expect(screen.getByRole("button", { name: /^simpan$/i })).toBeInTheDocument();
  });

  it("groups the fields under section headings", () => {
    render(
      <ProductModal
        open={true}
        onOpenChange={mockOnOpenChange}
        onSubmit={mockOnSubmit}
        isPending={false}
      />
    );

    expect(screen.getByRole("heading", { name: /^produk$/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /^deskripsi$/i })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /ketersediaan di marketplace/i })).toBeInTheDocument();
  });
  it("shows the relation action to admin_exam", () => {
    mockRole = "admin_exam";
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type: "exam", name: "Exam", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    expect(screen.getByRole("button", { name: "Ubah relasi" })).toBeInTheDocument();
  });

  it.each(["admin_store", "admin_school", "student", undefined])("hides relation action for %s", (role) => {
    mockRole = role;
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type: "exam", name: "Exam", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    expect(screen.queryByRole("button", { name: "Ubah relasi" })).not.toBeInTheDocument();
  });

  it.each(["exam", "course"] as const)("allows metadata edit with no %s relations selected", (type) => {
    detailState.data = { ...detailState.data!, type, exam_ids: [], course_ids: [] };
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type, name: "Digital", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    expect(screen.getByRole("button", { name: "Simpan" })).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "Simpan" }));
    expect(mockOnSubmit).toHaveBeenCalledOnce();
    expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("exam_ids");
    expect(mockOnSubmit.mock.calls[0][0]).not.toHaveProperty("course_ids");
  });

  it("changes exam relations separately as super_admin and refreshes detail", async () => {
    mockRole = "super_admin";
    mockChangeRelations.mockImplementation(async ({ input }) => {
      detailState.data = { ...detailState.data!, exam_ids: input.exam_ids, relations: sampleExams.filter((e) => input.exam_ids.includes(e.id)) };
      return detailState.data;
    });
    detailState.data = { ...detailState.data!, relations: [sampleExams[0]] };
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type: "exam", name: "Exam", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    expect(mockUseAdminProduct).toHaveBeenCalledWith("p1", true);
    fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
    expect(screen.getByLabelText("UTBK 2026")).toBeEnabled();
    fireEvent.click(screen.getByLabelText("Tryout SNBT"));
    fireEvent.click(screen.getByRole("button", { name: "Simpan relasi" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Ubah relasi" })).toBeInTheDocument());
    expect(mockChangeRelations).toHaveBeenCalledWith({ id: "p1", input: { exam_ids: ["e1", "e2"] } });
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getByText("Tryout SNBT")).toBeInTheDocument();
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it("reuses exam search when changing relations and keeps targets beyond the current results", async () => {
    mockRole = "super_admin";
    detailState.data = { ...detailState.data!, exam_ids: ["e1", "e-other-page"], relations: [{ id: "e1", title: "UTBK 2026" }, { id: "e-other-page", title: "Ujian Halaman Lain" }] };
    mockUseExams.mockImplementation((filter?: { q?: string }) => filter?.q === "snbt" ? { data: { data: [sampleExams[1]] } } : examsState);
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type: "exam", name: "Exam", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    expect(screen.getByText("Ujian Halaman Lain")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
    expect(screen.getByLabelText("Ujian Halaman Lain")).toBeChecked();
    fireEvent.change(screen.getByPlaceholderText("Cari ujian..."), { target: { value: "snbt" } });
    await waitFor(() => expect(screen.queryByLabelText("UTBK 2026")).not.toBeInTheDocument());
    fireEvent.click(screen.getByLabelText("Tryout SNBT"));
    fireEvent.click(screen.getByRole("button", { name: "Simpan relasi" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Ubah relasi" })).toBeInTheDocument());
    expect(mockChangeRelations).toHaveBeenCalledWith({ id: "p1", input: { exam_ids: ["e1", "e-other-page", "e2"] } });
  });

  it("shows Indonesian guidance when relations are locked by an order", async () => {
    mockRole = "super_admin";
    mockChangeRelations.mockRejectedValue(new ApiError("product_relations_locked", "locked", 409));
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ id: "p1", type: "exam", name: "Exam", price: 1000, status: "published" }} onSubmit={mockOnSubmit} isPending={false} />);
    fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
    fireEvent.click(screen.getByRole("button", { name: "Simpan relasi" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Produk ini sudah memiliki pesanan. Buat produk baru untuk menggunakan relasi yang berbeda.");
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it("shows detail loading and retry on an error without allowing relation changes", () => {
    mockRole = "super_admin";
    detailState = { data: undefined, isPending: true, isError: false };
    const props = { open: true, onOpenChange: mockOnOpenChange, product: { id: "p1", type: "exam" as const, name: "Exam", price: 1000, status: "published" as const }, onSubmit: mockOnSubmit, isPending: false };
    const { rerender } = render(<ProductModal {...props} />);
    expect(screen.getByText("Memuat relasi...")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Ubah relasi" })).not.toBeInTheDocument();
    detailState = { data: undefined, isPending: false, isError: true };
    rerender(<ProductModal {...props} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat relasi produk.");
    fireEvent.click(screen.getByRole("button", { name: "Coba lagi" }));
    expect(mockDetailRefetch).toHaveBeenCalledOnce();
  });

  it("changes course relations separately and can cancel an unsubmitted selection", async () => {
    mockRole = "super_admin";
    detailState.data = { id: "p1", type: "course", name: "Course", price: 1000, status: "published", course_ids: ["c1"], relations: [{ id: "c1", title: "Fisika Dasar" }] };
    render(<ProductModal open onOpenChange={mockOnOpenChange} product={detailState.data} onSubmit={mockOnSubmit} isPending={false} />);
    fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
    fireEvent.click(screen.getByLabelText("Fisika Dasar"));
    expect(screen.getByRole("button", { name: "Simpan relasi" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Batal ubah relasi" }));
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(screen.getByText("Fisika Dasar")).toBeInTheDocument();
    expect(mockChangeRelations).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
    fireEvent.click(screen.getByLabelText("Matematika Lanjut"));
    fireEvent.click(screen.getByRole("button", { name: "Simpan relasi" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Ubah relasi" })).toBeInTheDocument());
    expect(mockChangeRelations).toHaveBeenCalledWith({ id: "p1", input: { course_ids: ["c1", "c2"] } });
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  describe("linked items", () => {
    const uuid = "29ccb3e4-a46e-4f1e-9d57-1c2b3a4d5e6f";
    const examProduct = { id: "p1", type: "exam" as const, name: "Exam", price: 1000, status: "published" as const };
    const before = (a: HTMLElement, b: HTMLElement) => Boolean(a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

    it("lists linked titles, never ids, even when the item is outside the search results", () => {
      detailState.data = { ...detailState.data!, exam_ids: ["e1", uuid, "e-untitled"], relations: [{ id: "e1", title: "UTBK 2026" }, { id: uuid, title: "Ujian Halaman Lain" }, { id: "e-untitled", title: "" }] };
      render(<ProductModal open onOpenChange={mockOnOpenChange} product={examProduct} onSubmit={mockOnSubmit} isPending={false} />);
      expect(screen.getByText("UTBK 2026")).toBeInTheDocument();
      expect(screen.getByText("Ujian Halaman Lain")).toBeInTheDocument();
      expect(screen.queryByText(new RegExp(uuid.slice(0, 8)))).not.toBeInTheDocument();
      expect(screen.queryByText("e-untitled")).not.toBeInTheDocument();
      expect(screen.getAllByRole("listitem")).toHaveLength(2);
      expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    });

    it("resolves a selected linked title from the detail when changing relations", () => {
      mockRole = "super_admin";
      detailState.data = { ...detailState.data!, exam_ids: ["e1", uuid], relations: [{ id: "e1", title: "UTBK 2026" }, { id: uuid, title: "Ujian Halaman Lain" }] };
      render(<ProductModal open onOpenChange={mockOnOpenChange} product={examProduct} onSubmit={mockOnSubmit} isPending={false} />);
      fireEvent.click(screen.getByRole("button", { name: "Ubah relasi" }));
      expect(screen.getByLabelText("Ujian Halaman Lain")).toBeChecked();
      expect(screen.queryByText(new RegExp(uuid.slice(0, 8)))).not.toBeInTheDocument();
    });

    it("places the relations section right after Produk, before the image section, when creating", () => {
      render(<ProductModal open onOpenChange={mockOnOpenChange} onSubmit={mockOnSubmit} isPending={false} />);
      fireEvent.change(screen.getByLabelText("Jenis"), { target: { value: "exam" } });
      const heading = screen.getByRole("heading", { name: "Ujian terkait" });
      expect(before(screen.getByRole("heading", { name: /^produk$/i }), heading)).toBe(true);
      expect(before(heading, screen.getByRole("heading", { name: "Gambar produk" }))).toBe(true);
    });

    it.each([["exam", "Ujian terkait"], ["course", "Kursus terkait"]] as const)("places the %s relations section before the image section when editing", (type, title) => {
      detailState.data = { ...detailState.data!, type };
      render(<ProductModal open onOpenChange={mockOnOpenChange} product={{ ...examProduct, type }} onSubmit={mockOnSubmit} isPending={false} />);
      expect(before(screen.getByRole("heading", { name: title }), screen.getByRole("heading", { name: "Gambar produk" }))).toBe(true);
    });
  });

});
