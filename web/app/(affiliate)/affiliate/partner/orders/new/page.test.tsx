import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import NewPartnerOrderPage from "./page";
import { ApiError } from "@/lib/api";

const hook = vi.hoisted(() => ({
  previewMutate: vi.fn(),
  previewState: { data: undefined } as { data?: unknown },
  createMutate: vi.fn(),
}));

vi.mock("@/components/cart/SnapCheckout", () => ({
  SnapCheckout: ({ orderId, basePath }: { orderId: string; basePath: string }) => <div data-testid="snap" data-order={orderId} data-base={basePath} />,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/lib/hooks/partner", () => ({
  usePartnerProducts: () => ({ isLoading: false, data: { data: [{ id: "p1", type: "course", name: "Kelas Matematika", price: 100000 }] } }),
  usePartnerVouchers: () => ({ data: { data: [{ id: "v1", code: "MITRA10" }] } }),
  usePartnerStudents: () => ({
    isLoading: false,
    hasNextPage: false,
    data: { pages: [{ data: [{ id: "s1", name: "Ani Siswa", school_name: "SMA Contoh" }, { id: "s2", name: "Budi Siswa", school_name: null }] }] },
  }),
  usePreviewPartnerOrder: () => ({ mutate: hook.previewMutate, reset: vi.fn(), ...hook.previewState }),
  useCreatePartnerOrder: () => ({ mutate: hook.createMutate, isPending: false }),
}));

const preview = { net_new_count: 2, students: [], unit_price: 100000, subtotal: 200000, discount: 20000, total: 180000, promo_code: "MITRA10" };

async function chooseProduct() {
  fireEvent.click(screen.getByRole("combobox", { name: "Produk" }));
  fireEvent.click(await screen.findByRole("option", { name: /Kelas Matematika/ }));
}

describe("NewPartnerOrderPage", () => {
  beforeEach(() => {
    hook.previewMutate.mockReset();
    hook.createMutate.mockReset();
    hook.previewState = { data: undefined };
  });

  it("asks the server for a preview once a product and students are chosen, and shows what it answers", async () => {
    hook.previewState = { data: preview };
    render(<NewPartnerOrderPage />);
    expect(hook.previewMutate).not.toHaveBeenCalled();

    await chooseProduct();
    fireEvent.click(screen.getByRole("checkbox", { name: /Ani Siswa/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Budi Siswa/ }));

    await waitFor(() =>
      expect(hook.previewMutate).toHaveBeenLastCalledWith({ product_id: "p1", student_ids: ["s1", "s2"], promo_code: undefined }, expect.anything()),
    );
    expect(screen.getByText("2 siswa dipilih")).toBeInTheDocument();
    expect(screen.getByText("2 siswa akan ditagihkan.")).toBeInTheDocument();
    expect(screen.getByText(/180\.000/)).toBeInTheDocument();
  });

  it("sends the voucher it was given", async () => {
    render(<NewPartnerOrderPage />);
    await chooseProduct();
    fireEvent.click(screen.getByRole("checkbox", { name: /Ani Siswa/ }));
    fireEvent.click(screen.getByRole("combobox", { name: "Voucher" }));
    fireEvent.click(await screen.findByRole("option", { name: "MITRA10" }));

    await waitFor(() => expect(hook.previewMutate).toHaveBeenLastCalledWith(expect.objectContaining({ promo_code: "MITRA10" }), expect.anything()));
  });

  it("shows why the server refused, and does not let the order be created", async () => {
    hook.previewMutate.mockImplementation((_input, opts) =>
      opts.onError(new ApiError("participant_already_owns", "x", 409, { details: { students: [{ name: "Ani Siswa" }] } })),
    );
    hook.previewState = { data: preview };
    render(<NewPartnerOrderPage />);
    await chooseProduct();
    fireEvent.click(screen.getByRole("checkbox", { name: /Ani Siswa/ }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Siswa berikut sudah memiliki produk ini: Ani Siswa.");
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
    expect(screen.queryByText("2 siswa akan ditagihkan.")).toBeNull();
  });

  it("creates the order and moves on to the payment, in the Mitra's own order path", async () => {
    hook.previewState = { data: preview };
    hook.createMutate.mockImplementation((_input, opts) => opts.onSuccess({ id: "order-9" }));
    render(<NewPartnerOrderPage />);
    await chooseProduct();
    fireEvent.click(screen.getByRole("checkbox", { name: /Ani Siswa/ }));

    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    expect(hook.createMutate).toHaveBeenCalledWith({ product_id: "p1", student_ids: ["s1"], promo_code: undefined }, expect.anything());
    const snap = await screen.findByTestId("snap");
    expect(snap).toHaveAttribute("data-order", "order-9");
    expect(snap).toHaveAttribute("data-base", "/partners/me/orders");
    expect(screen.queryByRole("button", { name: "Buat pesanan" })).toBeNull();
  });

  it("keeps the create button off until there is a preview with someone to charge", async () => {
    render(<NewPartnerOrderPage />);
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
    hook.previewState = { data: { ...preview, net_new_count: 0 } };
    await chooseProduct();
    fireEvent.click(screen.getByRole("checkbox", { name: /Ani Siswa/ }));
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });
});
