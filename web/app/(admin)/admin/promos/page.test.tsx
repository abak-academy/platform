import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import PromosPage from "./page";
import type { PromoCode } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let promosState = {
  data: null as PromoCode[] | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let deleteState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-promos", () => ({
  useAdminPromoCodes: () => promosState,
  useCreatePromoCode: () => createState,
  useUpdatePromoCode: () => updateState,
  useDeletePromoCode: () => deleteState,
}));

vi.mock("@/lib/hooks/admin-products", () => ({
  useAdminProducts: () => ({ data: [] }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/components/admin/PromoModal", () => ({
  PromoModal: ({ open, onSubmit, promo }: any) =>
    open ? (
      <div role="dialog" aria-label={promo ? "edit promo code" : "create promo code"}>
        <button
          data-testid="modal-save"
          onClick={() =>
            onSubmit(
              promo
                ? { max_uses: 200, expires_at: "2027-01-01T00:00:00Z" }
                : { code: "NEWPROMO", discount_percent: 10, max_uses: 50 }
            )
          }
        >
          Save
        </button>
      </div>
    ) : null,
}));

const samplePromos: PromoCode[] = [
  {
    id: "promo-1",
    code: "DISKON10",
    discount_percent: 10,
    max_uses: 100,
    used_count: 12,
    expires_at: "2026-12-31T00:00:00Z",
  },
  {
    id: "promo-2",
    code: "CASHBACK20K",
    discount_amount: 20000,
    max_uses: 50,
    used_count: 5,
  },
];

describe("PromosPage", () => {
  beforeEach(() => {
    promosState = {
      data: samplePromos,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    createState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    updateState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    deleteState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders the promos table with code, discount, usage, and expiry", async () => {
    render(<PromosPage />);

    await waitFor(() => {
      expect(screen.getByText("DISKON10")).toBeInTheDocument();
      expect(screen.getByText("CASHBACK20K")).toBeInTheDocument();
    });

    expect(screen.getByText("10%")).toBeInTheDocument();
    expect(screen.getByText("Rp20.000")).toBeInTheDocument();
    expect(screen.getByText("12 / 100")).toBeInTheDocument();
    expect(screen.getByText("5 / 50")).toBeInTheDocument();
  });

  it("marks an affiliator-only promo in the public column", async () => {
    promosState = {
      ...promosState,
      data: [{ id: "promo-9", code: "AFF10", discount_percent: 10, used_count: 0, affiliator_only: true }],
    };
    render(<PromosPage />);

    expect(await screen.findByText("AFF10")).toBeInTheDocument();
    expect(screen.getByText("Affiliator")).toBeInTheDocument();
  });

  it("opens create modal and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "promo-3", code: "NEWPROMO", discount_percent: 10 });

    render(<PromosPage />);

    await waitFor(() => expect(screen.getByText("DISKON10")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat kode promo/i }));
    fireEvent.click(screen.getByTestId("modal-save"));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ code: "NEWPROMO", discount_percent: 10, max_uses: 50 })
      );
      expect(toast.success).toHaveBeenCalledWith("Kode promo dibuat.");
    });
  });

  it("opens edit modal prefilled and calls update mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "promo code updated" });

    render(<PromosPage />);

    await waitFor(() => expect(screen.getByText("DISKON10")).toBeInTheDocument());

    const row = screen.getByText("DISKON10").closest("tr");
    expect(row).toBeTruthy();
    const editButton = within(row!).getByRole("button", { name: /edit/i });
    fireEvent.click(editButton);

    fireEvent.click(screen.getByTestId("modal-save"));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ id: "promo-1", input: expect.objectContaining({ max_uses: 200 }) })
      );
      expect(toast.success).toHaveBeenCalledWith("Perubahan disimpan.");
    });
  });

  it("deletes a promo after confirm and shows a success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce(undefined);
    vi.stubGlobal("confirm", () => true);

    render(<PromosPage />);

    await waitFor(() => expect(screen.getByText("CASHBACK20K")).toBeInTheDocument());

    const row = screen.getByText("CASHBACK20K").closest("tr");
    expect(row).toBeTruthy();
    const deleteButton = within(row!).getByRole("button", { name: /hapus/i });
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("promo-2");
      expect(toast.success).toHaveBeenCalledWith("Kode promo dihapus.");
    });

    vi.unstubAllGlobals();
  });

  it("surfaces an API error as inline error text", async () => {
    promosState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<PromosPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
