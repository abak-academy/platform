import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import NewPartnerOrderPage from "./page";
import { ApiError } from "@/lib/api";

const hook = vi.hoisted(() => ({
  previewMutateAsync: vi.fn(),
  createMutate: vi.fn(),
  usePartnerStudents: vi.fn(),
}));

vi.mock("@/components/cart/SnapCheckout", () => ({
  SnapCheckout: ({ orderId, basePath }: { orderId: string; basePath: string }) => <div data-testid="snap" data-order={orderId} data-base={basePath} />,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/hooks/partner", () => ({
  partnerKeys: { orders: () => ["partner", "orders"] },
  usePartnerProducts: () => ({
    isLoading: false,
    data: {
      data: [
        { id: "p1", type: "course", name: "Kelas Matematika", price: 100000, grouping_id: "g1" },
        { id: "p2", type: "exam", name: "Paket Ujian UTBK", price: 50000, grouping_id: "g1" },
        { id: "p3", type: "course", name: "Kelas Fisika", price: 80000, grouping_id: null },
      ],
    },
  }),
  usePartnerVouchers: () => ({ data: { data: [{ id: "v1", code: "MITRA10" }] } }),
  usePartnerStudents: hook.usePartnerStudents,
  usePreviewPartnerOrder: () => ({ mutateAsync: hook.previewMutateAsync }),
  useCreatePartnerOrder: () => ({ mutateAsync: hook.createMutate }),
}));
vi.mock("@/lib/hooks/product-groupings", () => ({
  useProductGroupings: () => ({ data: [{ id: "g1", name: "Paket UTBK", status: "published", display_order: 0 }] }),
}));

interface PreviewFixture {
  name: string;
  unit_price: number;
  net_new_count: number;
  excluded?: { student_id: string; name: string; reason: string }[];
}

const itemFixtures: Record<string, PreviewFixture> = {
  p1: { name: "Kelas Matematika", unit_price: 100000, net_new_count: 2 },
  p2: { name: "Paket Ujian UTBK", unit_price: 50000, net_new_count: 2 },
};

function mockPreviewSuccess() {
  hook.previewMutateAsync.mockImplementation(async (input: { product_ids: string[]; student_ids: string[]; promo_code?: string }) => {
    const items = input.product_ids.map((id) => {
      const fixture = itemFixtures[id];
      if (!fixture) throw new ApiError("no_preview_fixture", "x", 400);
      return { product_id: id, name: fixture.name, unit_price: fixture.unit_price, net_new_count: fixture.net_new_count, excluded: fixture.excluded };
    });
    const subtotal = items.reduce((sum, item) => sum + item.unit_price * item.net_new_count, 0);
    const discount = input.promo_code ? Math.round(subtotal * 0.1) : 0;
    return { items, students: [], subtotal, discount, total: subtotal - discount, promo_code: input.promo_code };
  });
}

function mockStudents(ownedByP1: string[] = []) {
  hook.usePartnerStudents.mockImplementation((params: { productIds?: string[] }) => ({
    isLoading: false,
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
    data: {
      pages: [
        {
          data: [
            { id: "s1", name: "Ani Siswa", school_name: "SMA Contoh" },
            {
              id: "s2",
              name: "Budi Siswa",
              school_name: null,
              owned_product_ids: params.productIds?.includes("p1") && ownedByP1.includes("s2") ? ["p1"] : undefined,
            },
          ],
        },
      ],
    },
  }));
}

async function chooseProduct(name: RegExp) {
  fireEvent.click(screen.getByRole("checkbox", { name }));
}

async function chooseStudent(name: RegExp) {
  fireEvent.click(screen.getByRole("checkbox", { name }));
}

function sectionFor(headingText: string) {
  return within(screen.getByRole("heading", { name: headingText }).closest("section")!);
}

describe("NewPartnerOrderPage", () => {
  beforeEach(() => {
    hook.previewMutateAsync.mockReset();
    hook.createMutate.mockReset();
    hook.usePartnerStudents.mockReset();
    mockPreviewSuccess();
    mockStudents();
  });

  it("asks the server for one preview covering every selected product, and shows what it answers", async () => {
    render(<NewPartnerOrderPage />);
    expect(hook.previewMutateAsync).not.toHaveBeenCalled();

    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);
    await chooseStudent(/Budi Siswa/);

    await waitFor(() =>
      expect(hook.previewMutateAsync).toHaveBeenLastCalledWith({ product_ids: ["p1"], student_ids: ["s1", "s2"], promo_code: undefined }),
    );
    expect(screen.getByText("2 siswa dipilih")).toBeInTheDocument();
    expect(await screen.findByText("2 siswa akan ditagihkan.")).toBeInTheDocument();
    expect(screen.getAllByText(/200\.000/).length).toBeGreaterThan(0);
  });

  it("filters the product list by the chosen grouping", async () => {
    render(<NewPartnerOrderPage />);
    expect(screen.getByText(/Kelas Fisika/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("combobox", { name: "Grouping" }));
    fireEvent.click(await screen.findByRole("option", { name: "Paket UTBK" }));

    expect(screen.getByText(/Kelas Matematika/)).toBeInTheDocument();
    expect(screen.getByText(/Paket Ujian UTBK/)).toBeInTheDocument();
    expect(screen.queryByText(/Kelas Fisika/)).toBeNull();
  });

  it("filters the product list by name search", () => {
    render(<NewPartnerOrderPage />);
    fireEvent.change(screen.getByPlaceholderText("Cari produk..."), { target: { value: "fisika" } });

    expect(screen.getByText(/Kelas Fisika/)).toBeInTheDocument();
    expect(screen.queryByText(/Kelas Matematika/)).toBeNull();
  });

  it("sends the voucher it was given", async () => {
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);
    fireEvent.click(screen.getByRole("combobox", { name: "Voucher" }));
    fireEvent.click(await screen.findByRole("option", { name: "MITRA10" }));

    await waitFor(() => expect(hook.previewMutateAsync).toHaveBeenLastCalledWith(expect.objectContaining({ promo_code: "MITRA10" })));
  });

  it("shows why the server refused the preview, and does not let the order be created", async () => {
    hook.previewMutateAsync.mockRejectedValue(
      new ApiError("participant_already_owns", "x", 409, { details: { students: [{ name: "Ani Siswa" }] } }),
    );
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);

    expect(await screen.findByRole("alert")).toHaveTextContent("Siswa berikut sudah memiliki produk ini: Ani Siswa.");
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });

  it("previews every selected product in one combined request and renders each item's own line", async () => {
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseProduct(/Paket Ujian UTBK/);
    await chooseStudent(/Ani Siswa/);

    await waitFor(() =>
      expect(hook.previewMutateAsync).toHaveBeenLastCalledWith({ product_ids: ["p1", "p2"], student_ids: ["s1"], promo_code: undefined }),
    );
    expect(hook.previewMutateAsync).toHaveBeenCalledTimes(1);
    expect(await screen.findAllByText("2 siswa akan ditagihkan.")).toHaveLength(2);
    const previewSection = sectionFor("Rincian pembelian");
    expect(previewSection.getByText("Kelas Matematika")).toBeInTheDocument();
    expect(previewSection.getByText("Paket Ujian UTBK")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).not.toBeDisabled());
  });

  it("creates a single order for every selected product and shows one payment link", async () => {
    hook.createMutate.mockResolvedValue({ id: "order-1" });
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseProduct(/Paket Ujian UTBK/);
    await chooseStudent(/Ani Siswa/);

    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).not.toBeDisabled());
    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    await waitFor(() => expect(hook.createMutate).toHaveBeenCalledTimes(1));
    expect(hook.createMutate).toHaveBeenCalledWith({ product_ids: ["p1", "p2"], student_ids: ["s1"], promo_code: undefined });
    const snaps = await screen.findAllByTestId("snap");
    expect(snaps).toHaveLength(1);
    expect(snaps[0]).toHaveAttribute("data-order", "order-1");
  });

  it("selects every visible product with one click, scoped to the current grouping filter", async () => {
    render(<NewPartnerOrderPage />);

    fireEvent.click(screen.getByRole("combobox", { name: "Grouping" }));
    fireEvent.click(await screen.findByRole("option", { name: "Paket UTBK" }));

    fireEvent.click(sectionFor("Produk").getByRole("checkbox", { name: "Pilih semua" }));

    expect(sectionFor("Produk").getByRole("checkbox", { name: /Kelas Matematika/ })).toBeChecked();
    expect(sectionFor("Produk").getByRole("checkbox", { name: /Paket Ujian UTBK/ })).toBeChecked();
    expect(screen.getByText("2 produk dipilih")).toBeInTheDocument();

    fireEvent.click(sectionFor("Produk").getByRole("checkbox", { name: "Pilih semua" }));
    expect(screen.getByText("0 produk dipilih")).toBeInTheDocument();
  });

  it("selects every loaded student with one click", async () => {
    render(<NewPartnerOrderPage />);

    fireEvent.click(sectionFor("Siswa").getByRole("checkbox", { name: "Pilih semua" }));

    expect(screen.getByText("2 siswa dipilih")).toBeInTheDocument();

    fireEvent.click(sectionFor("Siswa").getByRole("checkbox", { name: "Pilih semua" }));
    expect(screen.getByText("0 siswa dipilih")).toBeInTheDocument();
  });

  it("keeps the create button off until the order as a whole has someone to charge", async () => {
    render(<NewPartnerOrderPage />);
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();

    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);
    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).not.toBeDisabled());

    // p3 has no preview fixture, so the combined preview request fails outright.
    await chooseProduct(/Kelas Fisika/);
    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled());
  });

  it("disables a student already owning one of the selected products, and will not let it be checked", async () => {
    mockStudents(["s2"]);
    render(<NewPartnerOrderPage />);

    await chooseProduct(/Kelas Matematika/);

    const row = sectionFor("Siswa").getByRole("checkbox", { name: /Budi Siswa/ });
    expect(row).toBeDisabled();
    expect(screen.getByText("Sudah memiliki salah satu produk yang dipilih")).toBeInTheDocument();
  });

  it("drops a checked student from the selection once a newly added product makes them already-own it", async () => {
    mockStudents(["s2"]);
    render(<NewPartnerOrderPage />);

    await chooseProduct(/Paket Ujian UTBK/);
    await chooseStudent(/Budi Siswa/);
    expect(screen.getByText("1 siswa dipilih")).toBeInTheDocument();

    await chooseProduct(/Kelas Matematika/);

    await waitFor(() => expect(screen.getByText("0 siswa dipilih")).toBeInTheDocument());
    expect(sectionFor("Siswa").getByRole("checkbox", { name: /Budi Siswa/ })).toBeDisabled();
  });

  it("select-all only picks students not already blocked by an owned product", async () => {
    mockStudents(["s2"]);
    render(<NewPartnerOrderPage />);

    await chooseProduct(/Kelas Matematika/);
    fireEvent.click(sectionFor("Siswa").getByRole("checkbox", { name: "Pilih semua" }));

    expect(screen.getByText("1 siswa dipilih")).toBeInTheDocument();
    expect(sectionFor("Siswa").getByRole("checkbox", { name: /Ani Siswa/ })).toBeChecked();
  });
});
