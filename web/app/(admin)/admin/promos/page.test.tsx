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

    expect(screen.getByText("DISKON10").closest("article")).toHaveTextContent("10% diskon");
    expect(screen.getByText("Rp20.000")).toBeInTheDocument();
    expect(screen.getByText((_, el) => el?.textContent === "12 / 100" && el.tagName === "SPAN")).toBeInTheDocument();
    expect(screen.getByText((_, el) => el?.textContent === "5 / 50" && el.tagName === "SPAN")).toBeInTheDocument();
    expect(screen.getAllByRole("article")).toHaveLength(2);
    expect(screen.getByText("DISKON10").closest("article")).toHaveTextContent("Aktif");
    expect(screen.getAllByRole("progressbar")).toHaveLength(2);
  });

  it("filters the cards by code in the toolbar", async () => {
    render(<PromosPage />);
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "cashback" } });
    expect(screen.getAllByRole("article")).toHaveLength(1);
    expect(screen.getByText("CASHBACK20K")).toBeInTheDocument();
    expect(screen.queryByText("DISKON10")).not.toBeInTheDocument();
  });

  it("opens create modal and calls create mutation on save", async () => {
    mockMutateAsync.mockResolvedValueOnce({ id: "promo-3", code: "NEWPROMO", discount_percent: 10 });

    render(<PromosPage />);

    await waitFor(() => expect(screen.getByText("DISKON10")).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /buat promo/i }));
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

    const row = screen.getByText("DISKON10").closest("article");
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

    const row = screen.getByText("CASHBACK20K").closest("article");
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
