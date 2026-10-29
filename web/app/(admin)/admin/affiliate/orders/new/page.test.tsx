import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import AffiliateNewOrderPage from "./page";
import type { AffiliatorOrderPreview } from "@/lib/types";

let readOnly = false;
vi.mock("@/lib/hooks/use-capability", () => ({ useHasCapability: () => true }));
vi.mock("@/components/admin/AffiliatorNotice", () => ({ AffiliatorNotice: () => null, useAffiliatorReadOnly: () => readOnly }));
vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));
vi.mock("@/lib/hooks/use-debounced-value", () => ({ useDebouncedValue: (v: string) => v }));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock("@/components/cart/SnapCheckout", () => ({
  SnapCheckout: ({ orderId, basePath }: { orderId: string; basePath: string }) => <div>pay {orderId} via {basePath}</div>,
}));

const studentCalls: Record<string, unknown>[] = [];
vi.mock("@/lib/hooks/admin-students", () => ({
  useAdminStudents: (opts: Record<string, unknown>) => {
    studentCalls.push(opts);
    return { isLoading: false, data: { data: [{ id: "st1", name: "Budi", school_name: "SMAN 1" }, { id: "st2", name: "Siti", school_name: "SMPN 2" }, { id: "st4", name: "Tono", school_name: null }] } };
  },
}));

const previewMutate = vi.fn();
const createMutate = vi.fn();
let liveOrders: { id: string; status: string }[] = [];
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorOrders: () => ({ data: { data: liveOrders } }),
  useAffiliatorMe: () => ({ data: { schools: [{ id: "s1", name: "SMAN 1" }, { id: "s2", name: "SMPN 2" }] } }),
  useAffiliatorProducts: () => ({ data: [{ id: "p1", name: "Try Out UTBK", type: "exam", price: 50000 }] }),
  usePreviewAffiliatorOrder: () => ({ mutateAsync: previewMutate, isPending: false }),
  useCreateAffiliatorOrder: () => ({ mutateAsync: createMutate, isPending: false }),
}));

const preview: AffiliatorOrderPreview = {
  net_new_count: 2, excluded: [{ student_id: "st3", name: "Dewi", reason: "already_owned" }],
  students: [
    { student_id: "st1", name: "Budi", school_id: "s1", school_name: "SMAN 1" },
    { student_id: "st2", name: "Siti", school_id: "s2", school_name: "SMPN 2" },
  ],
  unit_price: 50000, subtotal: 100000, discount: 10000, total: 90000, promo_code: "AFF10",
};

function fill() {
  fireEvent.change(screen.getByRole("combobox", { name: "Produk" }), { target: { value: "p1" } });
  fireEvent.click(screen.getByLabelText(/^Budi/));
  fireEvent.click(screen.getByLabelText(/^Siti/));
}

describe("AffiliateNewOrderPage", () => {
  beforeEach(() => {
    readOnly = false;
    studentCalls.length = 0;
    previewMutate.mockReset().mockResolvedValue(preview);
    createMutate.mockReset().mockResolvedValue({ id: "order-1", total: 90000 });
    liveOrders = [];
  });

  it("cannot preview until a product and at least one student are chosen", () => {
    render(<AffiliateNewOrderPage />);
    const button = screen.getByRole("button", { name: "Pratinjau" });
    expect(button).toBeDisabled();

    fill();
    expect(button).toBeEnabled();
  });

  it("lists students of every school without choosing a school first, and narrows by school on request", () => {
    render(<AffiliateNewOrderPage />);
    expect(studentCalls.at(-1)).toMatchObject({ status: "active" });
    expect(studentCalls.at(-1)?.schoolId).toBeFalsy();
    expect(screen.getByTestId("aff-student-list")).toHaveTextContent("Tanpa sekolah");

    fireEvent.change(screen.getByRole("combobox", { name: "Sekolah" }), { target: { value: "s1" } });
    expect(studentCalls.at(-1)).toMatchObject({ schoolId: "s1" });
  });

  it("keeps a student selected across searches and shows them with their school", () => {
    render(<AffiliateNewOrderPage />);
    fireEvent.click(screen.getByLabelText(/^Budi/));
    fireEvent.change(screen.getByLabelText("Cari nama siswa"), { target: { value: "sit" } });

    expect(studentCalls.at(-1)).toMatchObject({ q: "sit" });
    expect(screen.getByRole("group", { name: "Siswa dipilih" })).toHaveTextContent("Budi · SMAN 1");
  });

  it("previews the chosen students and promo, listing who is skipped", async () => {
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.change(screen.getByLabelText("Kode promo"), { target: { value: " AFF10 " } });
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));

    expect(await screen.findByTestId("aff-preview")).toHaveTextContent("Rp90.000");
    expect(previewMutate).toHaveBeenCalledWith({ product_id: "p1", student_ids: ["st1", "st2"], promo_code: "AFF10" });
    expect(screen.getByTestId("aff-preview-students")).toHaveTextContent("Siti · SMPN 2");
    expect(screen.getByText("Dewi")).toBeInTheDocument();
    expect(screen.getByText("-Rp10.000")).toBeInTheDocument();
  });

  it("shows the server's reason when the promo or school is refused", async () => {
    previewMutate.mockRejectedValue(new Error("order quantity below promo minimum"));
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("order quantity below promo minimum");
    expect(screen.queryByTestId("aff-preview")).not.toBeInTheDocument();
  });

  it("drops the preview when an input changes, so a stale total cannot be ordered", async () => {
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");

    fireEvent.click(screen.getByLabelText(/^Siti/));

    expect(screen.queryByTestId("aff-preview")).not.toBeInTheDocument();
  });

  it("creates the order and hands it to payment on the affiliator route", async () => {
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");
    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    expect(await screen.findByText("pay order-1 via /affiliates/me/orders")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Lihat pembelian" })).toHaveAttribute("href", "/admin/affiliate/orders");
  });

  it("follows the created order's status so a free or finished payment is visible", async () => {
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");
    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    expect(await screen.findByTestId("aff-order-status")).toHaveTextContent("Belum dibayar");
  });

  it("shows the settled status once the order appears in the list", async () => {
    liveOrders = [{ id: "order-1", status: "completed" }, { id: "other", status: "paid" }];
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");
    fireEvent.click(screen.getByRole("button", { name: "Buat pesanan" }));

    expect(await screen.findByTestId("aff-order-status")).toHaveTextContent("Selesai");
  });

  it("cannot create an order when nobody is left to buy for", async () => {
    previewMutate.mockResolvedValue({ ...preview, net_new_count: 0 });
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });

  it("disables ordering for a deactivated affiliator", async () => {
    readOnly = true;
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
    await screen.findByTestId("aff-preview");
    expect(screen.getByRole("button", { name: "Buat pesanan" })).toBeDisabled();
  });
});
