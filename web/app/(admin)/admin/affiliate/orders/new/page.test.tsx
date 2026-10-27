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
    return { data: { data: opts.enabled ? [{ id: "st1", name: "Budi" }, { id: "st2", name: "Siti" }] : [] } };
  },
}));

const previewMutate = vi.fn();
const createMutate = vi.fn();
vi.mock("@/lib/hooks/affiliates", () => ({
  useAffiliatorMe: () => ({ data: { schools: [{ id: "s1", name: "SMAN 1" }] } }),
  useAffiliatorProducts: () => ({ data: [{ id: "p1", name: "Try Out UTBK", type: "exam", price: 50000 }] }),
  usePreviewAffiliatorOrder: () => ({ mutateAsync: previewMutate, isPending: false }),
  useCreateAffiliatorOrder: () => ({ mutateAsync: createMutate, isPending: false }),
}));

const preview: AffiliatorOrderPreview = {
  net_new_count: 2, excluded: [{ student_id: "st3", name: "Dewi", reason: "already_owned" }],
  unit_price: 50000, subtotal: 100000, discount: 10000, total: 90000, promo_code: "AFF10",
};

function fill() {
  fireEvent.change(screen.getByRole("combobox", { name: "Sekolah" }), { target: { value: "s1" } });
  fireEvent.change(screen.getByRole("combobox", { name: "Produk" }), { target: { value: "p1" } });
  fireEvent.click(screen.getByLabelText("Budi"));
  fireEvent.click(screen.getByLabelText("Siti"));
}

describe("AffiliateNewOrderPage", () => {
  beforeEach(() => {
    readOnly = false;
    studentCalls.length = 0;
    previewMutate.mockReset().mockResolvedValue(preview);
    createMutate.mockReset().mockResolvedValue({ id: "order-1", total: 90000 });
  });

  it("cannot preview until a school, product and at least one student are chosen", () => {
    render(<AffiliateNewOrderPage />);
    const button = screen.getByRole("button", { name: "Pratinjau" });
    expect(button).toBeDisabled();

    fill();
    expect(button).toBeEnabled();
  });

  it("only loads students once a school is chosen", () => {
    render(<AffiliateNewOrderPage />);
    expect(studentCalls.at(-1)).toMatchObject({ enabled: false });

    fireEvent.change(screen.getByRole("combobox", { name: "Sekolah" }), { target: { value: "s1" } });
    expect(studentCalls.at(-1)).toMatchObject({ schoolId: "s1", enabled: true });
  });

  it("previews the chosen students and promo, listing who is skipped", async () => {
    render(<AffiliateNewOrderPage />);
    fill();
    fireEvent.change(screen.getByLabelText("Kode promo"), { target: { value: " AFF10 " } });
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));

    expect(await screen.findByTestId("aff-preview")).toHaveTextContent("Rp90.000");
    expect(previewMutate).toHaveBeenCalledWith({ school_id: "s1", product_id: "p1", student_ids: ["st1", "st2"], promo_code: "AFF10" });
    expect(screen.getByText("Dewi")).toBeInTheDocument();
    expect(screen.getByText("-Rp10.000")).toBeInTheDocument();
  });

  it("sends all=true instead of student ids when the whole school is chosen", async () => {
    render(<AffiliateNewOrderPage />);
    fireEvent.change(screen.getByRole("combobox", { name: "Sekolah" }), { target: { value: "s1" } });
    fireEvent.change(screen.getByRole("combobox", { name: "Produk" }), { target: { value: "p1" } });
    fireEvent.click(screen.getByLabelText("Semua siswa sekolah ini"));
    fireEvent.click(screen.getByRole("button", { name: "Pratinjau" }));

    await waitFor(() => expect(previewMutate).toHaveBeenCalledWith({ school_id: "s1", product_id: "p1", all: true }));
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

    fireEvent.click(screen.getByLabelText("Siti"));

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
