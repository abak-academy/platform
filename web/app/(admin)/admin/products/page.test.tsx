import { describe, it, expect, vi, beforeEach, beforeAll } from "vitest";
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
  useAdminProduct: (id?: string) => ({ data: sampleProducts.find((product) => product.id === id), isPending: false, isError: false, refetch: vi.fn() }),
  useUpdateProductRelations: () => ({ mutateAsync: vi.fn(), isPending: false }),
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

beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

function product(overrides: Partial<Product>): Product {
  return {
    id: "fixture", type: "book", name: "Fixture product", price: 10000, stock: 0,
    stock_limited: false, status: "draft", description: "", weight_grams: 0,
    image_url: "", specs: [], course_ids: [], exam_ids: [],
    available_from: null, available_until: null,
    created_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z",
    ...overrides,
  };
}

async function choose(name: string, option: string) {
  fireEvent.pointerDown(screen.getByRole("combobox", { name }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  fireEvent.click(within(listbox).getByRole("option", { name: option }));
  await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument());
}
function openFilters() {
  fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
}
function rowNames() {
  return within(screen.getByRole("table")).getAllByRole("row").slice(1).map((row) => within(row).getAllByRole("cell")[0].textContent);
}
const manyProducts = Array.from({ length: 55 }, (_, i) => product({
  id: `p${i}`, name: `Produk ${String(i + 1).padStart(2, "0")}`,
  type: i % 2 ? "course" : "book", price: (55 - i) * 1000, stock: i + 1,
  status: i % 2 ? "draft" : "published",
}));

const sampleProducts: Product[] = [
  product({ id: "p1", type: "book", name: "Buku Matematika", price: 75000, stock: 12, status: "published" }),
  product({ id: "p2", type: "course", name: "Kursus Fisika", price: 150000, status: "draft" }),
  product({ id: "p3", type: "exam", name: "Paket UTBK", price: 500000, status: "hidden" }),
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

  it("filters rows by type in the Filters panel", async () => {
    render(<ProductsPage />);
    openFilters();
    await choose("Jenis produk", "Kursus");
    expect(rowNames()).toEqual(["Kursus Fisika"]);
    await choose("Jenis produk", "Semua jenis");
    expect(rowNames()).toEqual(sampleProducts.map((p) => p.name));
  });

  it("opens the create modal and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce(product({ id: "p4", type: "book", name: "Buku Baru", price: 10000, status: "draft" }));

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Buku Matematika")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat produk/i }));

    expect(screen.getByRole("dialog", { name: /buat produk/i })).toBeInTheDocument();

    const nameInput = within(screen.getByRole("dialog")).getByLabelText(/nama/i);
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
    mockMutateAsync.mockResolvedValueOnce(product({ id: "p1", type: "book", name: "Buku Edit", price: 80000, status: "published" }));

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Buku Matematika")).toBeInTheDocument());

    const row = screen.getByText("Buku Matematika").closest("tr");
    expect(row).toBeTruthy();
    const editButton = within(row!).getByRole("button", { name: /^edit$/i });
    fireEvent.click(editButton);

    expect(screen.getByRole("dialog", { name: /edit produk/i })).toBeInTheDocument();
    expect(screen.getByDisplayValue("Buku Matematika")).toBeInTheDocument();

    const nameInput = within(screen.getByRole("dialog")).getByLabelText(/nama/i);
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

  it("offers an exam admin only course and exam types in the panel", async () => {
    mockRole = "admin_exam";
    render(<ProductsPage />);
    openFilters();
    fireEvent.pointerDown(screen.getByRole("combobox", { name: "Jenis produk" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
    const listbox = await screen.findByRole("listbox");
    expect(within(listbox).getAllByRole("option").map((o) => o.textContent)).toEqual(["Semua jenis", "Kursus", "Ujian"]);
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
      expect(screen.queryByRole("combobox", { name: "Baris per halaman" })).not.toBeInTheDocument();
    });
  });

  it("shows the quota for a limited exam and a dash for an unlimited exam", async () => {
    productsState = {
      data: [
        product({ id: "e1", type: "exam", name: "Limited Exam", price: 100000, stock: 5, stock_limited: true, status: "published" }),
        product({ id: "e2", type: "exam", name: "Unlimited Exam", price: 100000, status: "published" }),
      ],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<ProductsPage />);

    await waitFor(() => expect(screen.getByText("Limited Exam")).toBeInTheDocument());

    const limitedRow = screen.getByText("Limited Exam").closest("tr")!;
    expect(within(limitedRow).getAllByRole("cell")[3].textContent).toBe("5");

    const unlimitedRow = screen.getByText("Unlimited Exam").closest("tr")!;
    expect(within(unlimitedRow).getAllByRole("cell")[3].textContent).toBe("-");
  });

  it("searches product names and shows the filtered total", () => {
    render(<ProductsPage />);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "  FISIKA  " } });
    expect(rowNames()).toEqual(["Kursus Fisika"]);
    expect(screen.getByText("Menampilkan 1-1 dari 1")).toBeInTheDocument();
  });

  it.each([
    ["Nama Z-A", ["Alpha", "Delta", "Charlie", "Bravo"], ["Delta", "Charlie", "Bravo", "Alpha"]],
    ["Harga terendah-tertinggi", ["Bravo", "Delta", "Alpha", "Charlie"], ["Alpha", "Charlie", "Bravo", "Delta"]],
    ["Stok tertinggi-terendah", ["Delta", "Charlie", "Bravo", "Alpha"], ["Bravo", "Alpha", "Charlie", "Delta"]],
    ["Stok terendah-tertinggi", ["Delta", "Bravo", "Charlie", "Alpha"], ["Charlie", "Alpha", "Bravo", "Delta"]],
  ])("sorts rows by %s", async (label, names, expected) => {
    const prices = { Alpha: 10000, Bravo: 30000, Charlie: 20000, Delta: 40000 };
    const stocks = { Alpha: 5, Bravo: 9, Charlie: 2, Delta: 0 };
    productsState.data = (names as (keyof typeof prices)[]).map((name) => product({ id: name, name, price: prices[name], stock: stocks[name], type: name === "Delta" ? "course" : "book" }));
    render(<ProductsPage />);
    await choose("Urutkan produk", label as string);
    expect(rowNames()).toEqual(expected);
  });

  it("pages without skipping or duplicating rows", async () => {
    productsState.data = manyProducts;
    render(<ProductsPage />);
    expect(screen.getByText("Menampilkan 1-20 dari 55")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    const seen = [...rowNames()];
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(screen.getByText("Menampilkan 21-40 dari 55")).toBeInTheDocument();
    seen.push(...rowNames());
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(screen.getByText("Menampilkan 41-55 dari 55")).toBeInTheDocument();
    seen.push(...rowNames());
    expect(seen).toEqual(manyProducts.map((p) => p.name));
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Sebelumnya" }));
    expect(rowNames()).toEqual(manyProducts.slice(20, 40).map((p) => p.name));
  });

  it("changes page size without skipping or duplicating rows", async () => {
    productsState.data = manyProducts;
    render(<ProductsPage />);
    await choose("Baris per halaman", "50");
    expect(screen.getByText("Menampilkan 1-50 dari 55")).toBeInTheDocument();
    expect(rowNames()).toEqual(manyProducts.slice(0, 50).map((p) => p.name));
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(rowNames()).toEqual(manyProducts.slice(50).map((p) => p.name));
    await choose("Baris per halaman", "100");
    expect(screen.getByText("Menampilkan 1-55 dari 55")).toBeInTheDocument();
    expect(rowNames()).toEqual(manyProducts.map((p) => p.name));
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
  });

  it.each(["search", "type", "status", "sort", "limit"])("returns to page one after changing %s", async (field) => {
    productsState.data = manyProducts;
    render(<ProductsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(screen.getByText("Menampilkan 21-40 dari 55")).toBeInTheDocument();
    if (field === "search") fireEvent.change(screen.getByRole("searchbox"), { target: { value: "Produk" } });
    if (field === "type" || field === "status") {
      openFilters();
      await choose(field === "type" ? "Jenis produk" : "Status", field === "type" ? "Kursus" : "Draf");
      fireEvent.click(screen.getByRole("button", { name: "Tampilkan produk" }));
    }
    if (field === "sort") await choose("Urutkan produk", "Nama Z-A");
    if (field === "limit") await choose("Baris per halaman", "50");
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
    expect(screen.getByText(/^Menampilkan 1-/)).toBeInTheDocument();
    if (field === "type" || field === "status") expect(rowNames()).toEqual(manyProducts.filter((p) => p.type === "course").slice(0, 20).map((p) => p.name));
  });

  it("clamps to the last page when rows shrink, including an empty refetch", async () => {
    productsState.data = manyProducts;
    const { rerender } = render(<ProductsPage />);
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    productsState = { ...productsState, data: manyProducts.slice(0, 23) };
    rerender(<ProductsPage />);
    await screen.findByText("Menampilkan 21-23 dari 23");
    expect(rowNames()).toEqual(manyProducts.slice(20, 23).map((p) => p.name));
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
    productsState = { ...productsState, data: [] };
    rerender(<ProductsPage />);
    await screen.findByText("Menampilkan 0-0 dari 0");
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();
  });

  it("uses the existing empty state when filters match nothing", async () => {
    render(<ProductsPage />);
    openFilters();
    await choose("Status", "Diarsipkan");
    fireEvent.click(screen.getByRole("button", { name: "Tampilkan produk" }));
    expect(screen.getByText("Tidak ada produk.")).toBeInTheDocument();
    expect(screen.getByText("Menampilkan 0-0 dari 0")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Berikutnya" })).toBeDisabled();
  });

  it("hides the footer during loading", () => {
    productsState = { ...productsState, data: null, isLoading: true };
    render(<ProductsPage />);
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: "Baris per halaman" })).not.toBeInTheDocument();
  });
});
