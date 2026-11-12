import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import NewPartnerOrderPage from "./page";
import { ApiError } from "@/lib/api";

const hook = vi.hoisted(() => ({
  previewMutate: vi.fn(),
  createMutate: vi.fn(),
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
  usePartnerStudents: () => ({
    isLoading: false,
    hasNextPage: false,
    data: { pages: [{ data: [{ id: "s1", name: "Ani Siswa", school_name: "SMA Contoh" }, { id: "s2", name: "Budi Siswa", school_name: null }] }] },
  }),
  usePreviewPartnerOrder: () => ({ mutate: hook.previewMutate }),
  useCreatePartnerOrder: () => ({ mutateAsync: hook.createMutate }),
}));
vi.mock("@/lib/hooks/product-groupings", () => ({
  useProductGroupings: () => ({ data: [{ id: "g1", name: "Paket UTBK", status: "published", display_order: 0 }] }),
}));

const previewFor: Record<string, { net_new_count: number; students: unknown[]; unit_price: number; subtotal: number; discount: number; total: number; promo_code?: string }> = {
  p1: { net_new_count: 2, students: [], unit_price: 100000, subtotal: 200000, discount: 20000, total: 180000 },
  p2: { net_new_count: 2, students: [], unit_price: 50000, subtotal: 100000, discount: 0, total: 100000 },
};

function mockPreviewSuccess() {
  hook.previewMutate.mockImplementation((input: { product_id: string; promo_code?: string }, opts: { onSuccess: (d: unknown) => void }) => {
    const data = previewFor[input.product_id];
    if (data) opts.onSuccess({ ...data, promo_code: input.promo_code });
  });
}

async function chooseProduct(name: RegExp) {
  fireEvent.click(screen.getByRole("checkbox", { name }));
}

async function chooseStudent(name: RegExp) {
  fireEvent.click(screen.getByRole("checkbox", { name }));
}

describe("NewPartnerOrderPage", () => {
  beforeEach(() => {
    hook.previewMutate.mockReset();
    hook.createMutate.mockReset();
    mockPreviewSuccess();
  });

  it("asks the server for a preview once a product and students are chosen, and shows what it answers", async () => {
    render(<NewPartnerOrderPage />);
    expect(hook.previewMutate).not.toHaveBeenCalled();

    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);
    await chooseStudent(/Budi Siswa/);

    await waitFor(() =>
      expect(hook.previewMutate).toHaveBeenLastCalledWith({ product_id: "p1", student_ids: ["s1", "s2"], promo_code: undefined }, expect.anything()),
    );
    expect(screen.getByText("2 siswa dipilih")).toBeInTheDocument();
    expect(screen.getByText("2 siswa akan ditagihkan.")).toBeInTheDocument();
    expect(screen.getByText(/180\.000/)).toBeInTheDocument();
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

    await waitFor(() => expect(hook.previewMutate).toHaveBeenLastCalledWith(expect.objectContaining({ promo_code: "MITRA10" }), expect.anything()));
  });

  it("shows why the server refused a product's preview, and does not let the order be created", async () => {
    hook.previewMutate.mockImplementation((input: { product_id: string }, opts: { onError: (e: unknown) => void }) =>
      opts.onError(new ApiError("participant_already_owns", "x", 409, { details: { students: [{ name: "Ani Siswa" }] } })),
    );
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);

    expect(await screen.findByRole("alert")).toHaveTextContent("Siswa berikut sudah memiliki produk ini: Ani Siswa.");
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });

  it("creates one order per selected product and shows a payment link for each", async () => {
    hook.createMutate.mockImplementation(async (input: { product_id: string }) => ({ id: `order-${input.product_id}` }));
    render(<NewPartnerOrderPage />);
    await chooseProduct(/Kelas Matematika/);
    await chooseProduct(/Paket Ujian UTBK/);
    await chooseStudent(/Ani Siswa/);

    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).not.toBeDisabled());
    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    await waitFor(() => expect(hook.createMutate).toHaveBeenCalledTimes(2));
    expect(hook.createMutate).toHaveBeenCalledWith({ product_id: "p1", student_ids: ["s1"], promo_code: undefined });
    expect(hook.createMutate).toHaveBeenCalledWith({ product_id: "p2", student_ids: ["s1"], promo_code: undefined });
    const snaps = await screen.findAllByTestId("snap");
    expect(snaps.map((s) => s.getAttribute("data-order")).sort()).toEqual(["order-p1", "order-p2"]);
  });

  it("keeps the create button off until every selected product has someone to charge", async () => {
    render(<NewPartnerOrderPage />);
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();

    await chooseProduct(/Kelas Matematika/);
    await chooseStudent(/Ani Siswa/);
    await waitFor(() => expect(screen.getByRole("button", { name: "Buat pesanan" })).not.toBeDisabled());

    // p3 has no preview fixture, so its own preview call never succeeds.
    await chooseProduct(/Kelas Fisika/);
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });
});
