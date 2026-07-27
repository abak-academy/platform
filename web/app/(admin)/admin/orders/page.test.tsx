import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import OrdersPage from "./page";
import type { Order } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();

let ordersState = {
  data: null as Order[] | null,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  refetch: vi.fn(),
};

let confirmState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let shipState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let completeState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let refundState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let reconcileState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-orders", () => ({
  useAdminOrders: () => ordersState,
  useAdminOrder: () => ({}),
  useConfirmOrder: () => confirmState,
  useShipOrder: () => shipState,
  useCompleteOrder: () => completeState,
  useRefundOrder: () => refundState,
  useReconcileOrder: () => reconcileState,
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

const sampleOrders: Order[] = [
  {
    id: "o1",
    student_id: "s1",
    status: "payment_pending",
    subtotal: 100000,
    discount: 0,
    shipping_cost: 15000,
    total: 115000,
    items: [{ id: "i1", order_id: "o1", product_id: "p1", product_type: "book", name: "Buku A", unit_price: 100000, qty: 1, jumlah: 100000 }],
  },
  {
    id: "o2",
    student_id: "s2",
    status: "paid",
    subtotal: 200000,
    discount: 0,
    shipping_cost: 0,
    total: 200000,
    tracking_number: "JNE-999",
    selected_courier: "JNE",
    selected_service: "Reguler",
    shipping_address: {
      penerima: "Sabian Isaac",
      telepon: "082113092527",
      alamat: "Jl. Melati 9",
      kecamatan: "Bantargebang",
      kota: "Kota Bekasi",
      provinsi: "Jawa Barat",
      kode_pos: "17151",
      catatan: "Titip di pos satpam",
    },
    items: [{ id: "i2", order_id: "o2", product_id: "p2", product_type: "book", name: "Buku Shipped", unit_price: 200000, qty: 1, jumlah: 200000 }],
  },
  {
    id: "o3",
    student_id: "s3",
    status: "completed",
    subtotal: 50000,
    discount: 0,
    shipping_cost: 0,
    total: 50000,
    items: [{ id: "i3", order_id: "o3", product_id: "p3", product_type: "course", name: "Kursus B", unit_price: 50000, qty: 1, jumlah: 50000 }],
  },
];

describe("OrdersPage", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    ordersState = {
      data: sampleOrders,
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };
    confirmState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    shipState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    completeState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    refundState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    reconcileState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders the orders table with order number, buyer, product, amount, payment and shipping", async () => {
    render(<OrdersPage />);

    await waitFor(() => {
      expect(screen.getByText(/Buku A/)).toBeInTheDocument();
    });

    expect(screen.getByText("Rp115.000")).toBeInTheDocument();
    expect(screen.getByText("...s1")).toBeInTheDocument();
    // Shipping column renders — "Dikirim" appears both as a filter chip and as a badge
    expect(screen.getAllByText("Dikirim").length).toBeGreaterThanOrEqual(1);
  });

  it("shows confirm and reconcile actions for pending orders", async () => {
    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr");
    expect(row).toBeTruthy();
    expect(within(row!).queryByRole("button", { name: /konfirmasi/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /rekonsiliasi/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /kirim/i })).not.toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /refund/i })).not.toBeInTheDocument();
  });

  it("confirms an order and shows success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order confirmed" });

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr");
    const confirmButton = within(row!).getByRole("button", { name: /konfirmasi/i });
    fireEvent.click(confirmButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("o1");
      expect(toast.success).toHaveBeenCalledWith("Pesanan dikonfirmasi.");
    });
  });

  // The tracking number used to be collected with window.prompt, which cannot be
  // styled, validated or cancelled cleanly, and blocks the whole tab.
  it("ships an order with the tracking number typed into the modal", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order shipped" });

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText(/no\. resi/i), {
      target: { value: "JNE-123" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /^kirim$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "o2", trackingNumber: "JNE-123" });
      expect(toast.success).toHaveBeenCalledWith("Pesanan dikirim.");
    });
  });

  it("will not submit an empty tracking number", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByRole("button", { name: /^kirim$/i })).toBeDisabled();
  });

  // The row was inert: an admin could see that an order existed but not who it
  // was going to, what was in it, or which courier had it.
  it("opens the order detail when a row is clicked", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    fireEvent.click(screen.getByText(/Buku Shipped/).closest("tr")!);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Jl. Melati 9")).toBeInTheDocument();
    expect(within(dialog).getByText("Titip di pos satpam")).toBeInTheDocument();
    expect(within(dialog).getByText("JNE-999")).toBeInTheDocument();
    expect(within(dialog).getByText(/JNE — Reguler/)).toBeInTheDocument();
  });

  // A street and a postcode alone do not say where a parcel is going.
  it("shows the district, city and province, narrowest first", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    fireEvent.click(screen.getByText(/Buku Shipped/).closest("tr")!);

    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("Bantargebang, Kota Bekasi, Jawa Barat"),
    ).toBeInTheDocument();
  });

  // Orders placed before checkout stored the names only ever held the IDs, and
  // an ID is not an address — the line is dropped rather than shown raw.
  it("omits the region line on an order that predates the snapshot", async () => {
    ordersState = {
      ...ordersState,
      data: [
        {
          ...sampleOrders[1],
          shipping_address: {
            penerima: "Sabian Isaac",
            alamat: "Jl. Melati 9",
            provinsi_id: "32",
            kota_id: "3275",
            kecamatan_id: "327501",
            kode_pos: "17151",
          },
        },
      ],
    };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    fireEvent.click(screen.getByText(/Buku Shipped/).closest("tr")!);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Jl. Melati 9")).toBeInTheDocument();
    expect(within(dialog).queryByText(/3275/)).toBeNull();
    expect(within(dialog).queryByText("327501")).toBeNull();
  });

  // Every action button sits inside the clickable row.
  it("does not open the detail when a row action is clicked", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /konfirmasi/i }));

    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("requires a merchandise order to ship before it can complete", async () => {
    ordersState = {
      data: [{
        id: "o-merch",
        student_id: "s-merch",
        status: "processing",
        subtotal: 75000,
        discount: 0,
        shipping_cost: 15000,
        total: 90000,
        items: [{ id: "i-merch", order_id: "o-merch", product_id: "p-merch", product_type: "merchandise", name: "Kaos Akademi", unit_price: 75000, qty: 1, jumlah: 75000 }],
      }],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    const { rerender } = render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText("Kaos Akademi")).toBeInTheDocument());

    let row = screen.getByText("Kaos Akademi").closest("tr");
    expect(within(row!).getByRole("button", { name: /kirim/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /selesai/i })).not.toBeInTheDocument();

    ordersState = {
      ...ordersState,
      data: [{ ...ordersState.data![0], status: "shipped", tracking_number: "JNE-MERCH-1" }],
    };
    rerender(<OrdersPage />);

    row = screen.getByText("Kaos Akademi").closest("tr");
    expect(within(row!).queryByRole("button", { name: /kirim/i })).not.toBeInTheDocument();
    expect(within(row!).getByRole("button", { name: /selesai/i })).toBeInTheDocument();
  });

  it("requires a medal order to ship before it can complete", async () => {
    ordersState = {
      data: [{ id: "o-medal", student_id: "s-medal", status: "processing", subtotal: 75000, discount: 0, shipping_cost: 15000, total: 90000,
        items: [{ id: "i-medal", order_id: "o-medal", product_id: "p-medal", product_type: "medal", name: "Medali Emas", unit_price: 75000, qty: 1, jumlah: 75000 }] }],
      isLoading: false, isError: false, error: null, refetch: vi.fn(),
    };
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText("Medali Emas")).toBeInTheDocument());
    const row = screen.getByText("Medali Emas").closest("tr");
    expect(within(row!).getByRole("button", { name: /kirim/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /selesai/i })).not.toBeInTheDocument();
  });

  it("refunds an order after confirmation", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order refunded" });
    vi.stubGlobal("confirm", () => true);

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Kursus B/)).toBeInTheDocument());

    const row = screen.getByText(/Kursus B/).closest("tr");
    const refundButton = within(row!).getByRole("button", { name: /refund/i });
    fireEvent.click(refundButton);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("o3");
      expect(toast.success).toHaveBeenCalledWith("Pesanan direfund.");
    });

  });

  it("filters rows by status chips", async () => {
    ordersState = {
      data: sampleOrders.filter((o) => o.status === "paid"),
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    };

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const paidChip = screen.getByRole("button", { name: /^dibayar$/i });
    fireEvent.click(paidChip);

    expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument();
    expect(screen.queryByText(/Buku A/)).not.toBeInTheDocument();
  });

  it("surfaces an API error as inline error text", async () => {
    ordersState = {
      data: null,
      isLoading: false,
      isError: true,
      error: new Error("gagal memuat"),
      refetch: vi.fn(),
    };

    render(<OrdersPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});
