import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import ProductsPage from "./page";
import type { Product } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let productsState = {
  data: null as Product[] | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let publishState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let deleteState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-products", () => ({
  useAdminProducts: () => productsState,
  useCreateProduct: () => createState,
  useUpdateProduct: () => updateState,
  usePublishProduct: () => publishState,
  useDeleteProduct: () => deleteState,
}));

vi.mock("@/lib/hooks/admin-courses", () => ({
  useAdminCourses: () => ({ data: [], isLoading: false }),
}));

vi.mock("@/lib/hooks/admin-exams", () => ({
  useExams: () => ({ data: { data: [] }, isLoading: false }),
}));

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => ({ mutateAsync: vi.fn() }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

let mockRole = "admin_store";

vi.mock("@/lib/hooks/use-capability", () => ({
  useResolvedRole: () => ({ role: mockRole, hydrated: true, meIsError: false }),
}));

const sampleProducts: Product[] = [
  { id: "p1", type: "book", name: "Buku Matematika", price: 75000, stock: 12, status: "published" },
  { id: "p2", type: "course", name: "Kursus Fisika", price: 150000, status: "draft" },
  { id: "p3", type: "exam", name: "Paket UTBK", price: 500000, status: "hidden" },
];

describe("ProductsPage", () => {
  beforeEach(() => {
    mockRole = "admin_store";
    productsState = {
      data: sampleProducts,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    publishState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    deleteState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders the products table with type, price, stock, and status", async () => {
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText("Buku Matematika")).toBeInTheDocument();
      expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
      expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
    });

    expect(screen.getByText("Rp75.000")).toBeInTheDocument();
    expect(screen.getByText("12")).toBeInTheDocument();
    expect(screen.getByText("published")).toBeInTheDocument();
  });

  it("filters rows by type chips", async () => {
    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Buku Matematika")).toBeInTheDocument());

    const courseChip = screen.getByRole("button", { name: /^kursus$/i });
    fireEvent.click(courseChip);

    expect(screen.queryByText("Buku Matematika")).not.toBeInTheDocument();
    expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    expect(screen.queryByText("Paket UTBK")).not.toBeInTheDocument();

    const allChip = screen.getByRole("button", { name: /^semua$/i });
    fireEvent.click(allChip);

    expect(screen.getByText("Buku Matematika")).toBeInTheDocument();
    expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
  });

  it("opens the create modal and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "p4", type: "book", name: "Buku Baru", price: 10000, status: "draft" });

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Buku Matematika")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat produk/i }));

    expect(screen.getByRole("dialog", { name: /buat produk/i })).toBeInTheDocument();

    const nameInput = screen.getByLabelText(/nama/i);
    fireEvent.input(nameInput, { target: { value: "Buku Baru" } });

    const priceInput = screen.getByLabelText(/harga/i);
    fireEvent.input(priceInput, { target: { value: "10000" } });

    const typeSelect = screen.getByLabelText(/jenis/i);
    fireEvent.change(typeSelect, { target: { value: "book" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ name: "Buku Baru", price: 10000, type: "book" })
      );
      expect(toast.success).toHaveBeenCalledWith("Produk dibuat.");
    });
  });

  it("opens the edit modal prefilled and calls update mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "p1", type: "book", name: "Buku Edit", price: 80000, status: "published" });

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Buku Matematika")).toBeInTheDocument());

    const row = screen.getByText("Buku Matematika").closest("tr");
    expect(row).toBeTruthy();
    const editButton = within(row!).getByRole("button", { name: /^edit$/i });
    fireEvent.click(editButton);

    expect(screen.getByRole("dialog", { name: /edit produk/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Buku Matematika")).toBeInTheDocument();

    const nameInput = screen.getByLabelText(/nama/i);
    fireEvent.input(nameInput, { target: { value: "Buku Edit" } });

    fireEvent.click(screen.getByRole("button", { name: /^simpan$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ id: "p1", input: expect.objectContaining({ name: "Buku Edit" }) })
      );
      expect(toast.success).toHaveBeenCalledWith("Perubahan disimpan.");
    });
  });

  it("publishes a product and shows a success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "product published" });

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Kursus Fisika")).toBeInTheDocument());

    const row = screen.getByText("Kursus Fisika").closest("tr");
    expect(row).toBeTruthy();
    const publishButton = within(row!).getByRole("button", { name: /publikasikan/i });
    fireEvent.click(publishButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("p2");
      expect(toast.success).toHaveBeenCalledWith("Produk dipublikasikan.");
    });
  });

  it("deletes a product after confirm and shows a success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce(undefined);
    vi.stubGlobal("confirm", () => true);

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Paket UTBK")).toBeInTheDocument());

    const row = screen.getByText("Paket UTBK").closest("tr");
    expect(row).toBeTruthy();
    const deleteButton = within(row!).getByRole("button", { name: /hapus/i });
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("p3");
      expect(toast.success).toHaveBeenCalledWith("Produk dihapus.");
    });

    vi.unstubAllGlobals();
  });

  it("shows an exam admin only the digital products", async () => {
    mockRole = "admin_exam";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    });
    expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
    expect(screen.queryByText("Buku Matematika")).not.toBeInTheDocument();
  });

  it("offers an exam admin only the digital filter tabs", async () => {
    mockRole = "admin_exam";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Kursus" })).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: "Ujian" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Buku" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Merchandise" })).not.toBeInTheDocument();
  });

  it("shows a store admin every product", async () => {
    mockRole = "admin_store";
    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText("Buku Matematika")).toBeInTheDocument();
    });
    expect(screen.getByText("Kursus Fisika")).toBeInTheDocument();
    expect(screen.getByText("Paket UTBK")).toBeInTheDocument();
  });

  it("surfaces an API error as inline error text", async () => {
    productsState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<ProductsPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
