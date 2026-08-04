import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within, fireEvent } from "@testing-library/react";
import { toast } from "sonner";
import OrdersPage from "./page";
import type { Order, OrderSummary } from "@/lib/types";

const mockMutate = vi.fn();
const mockMutateAsync = vi.fn();
const mockFetchNextPage = vi.fn();

interface OrdersResultPage {
  data: Order[];
  next_cursor: string;
}

function pages(orders: Order[]): { pages: OrdersResultPage[] } {
  return { pages: [{ data: orders, next_cursor: "" }] };
}

let ordersState = {
  data: pages([]) as { pages: OrdersResultPage[] } | undefined,
  isLoading: true,
  isError: false,
  error: null as Error | null,
  hasNextPage: false,
  isFetchingNextPage: false,
  fetchNextPage: mockFetchNextPage,
};

let summaryState = { data: undefined as OrderSummary | undefined, isLoading: false };

let confirmState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let shipState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let shipManualState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let completeState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let refundState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let reconcileState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let refreshShipmentState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
let cancelShipmentState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };

vi.mock("@/lib/hooks/admin-orders", () => ({
  useAdminOrders: () => ordersState,
  useAdminOrderSummary: () => summaryState,
  useAdminOrder: () => ({}),
  useConfirmOrder: () => confirmState,
  useShipOrder: () => shipState,
  useShipOrderManual: () => shipManualState,
  useCompleteOrder: () => completeState,
  useRefundOrder: () => refundState,
  useReconcileOrder: () => reconcileState,
  useOrderTracking: () => ({ data: null, isLoading: false, isError: false }),
  useRefreshShipment: () => refreshShipmentState,
  useCancelShipment: () => cancelShipmentState,
  useFetchPaymentProofURL: () => ({ mutate: vi.fn(), isPending: false }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// ConfirmOrderModal's own upload-then-submit behavior (proof required before
// submit, request body carries the uploaded key) is covered directly by
// ConfirmOrderModal.test.tsx; here we only need to prove the page wires the
// modal's onConfirm callback to useConfirmOrder correctly.
vi.mock("@/components/admin/ConfirmOrderModal", () => ({
  ConfirmOrderModal: ({ open, onConfirm }: { open: boolean; onConfirm: (key: string) => void }) =>
    open ? <button onClick={() => onConfirm("payment_proof/admin-1/proof.jpg")}>Confirm-Stub</button> : null,
}));

vi.mock("@/components/admin/RefundOrderModal", () => ({
  RefundOrderModal: ({ open, onRefund }: { open: boolean; onRefund: (key: string) => void }) =>
    open ? <button onClick={() => onRefund("refund_proof/admin-1/trf.jpg")}>Refund-Stub</button> : null,
}));

const sampleOrders: Order[] = [
  {
    id: "o1",
    student_id: "s1",
    student_name: "Siswa Uji A",
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
    student_name: "Siswa Uji B",
    status: "paid",
    subtotal: 200000,
    discount: 0,
    shipping_cost: 0,
    total: 200000,
    tracking_number: "JNE-999",
    selected_courier: "JNE",
    selected_service: "Reguler",
    shipping_address: {
      penerima: "Budi Test",
      telepon: "081200000000",
      alamat: "Jl. Contoh No. 1",
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
    student_name: "Siswa Uji C",
    status: "completed",
    subtotal: 50000,
    discount: 0,
    shipping_cost: 0,
    total: 50000,
    items: [{ id: "i3", order_id: "o3", product_id: "p3", product_type: "course", name: "Kursus B", unit_price: 50000, qty: 1, jumlah: 50000 }],
  },
];

/** The row's last cell — the only place a primary action button may appear. */
function actionCell(row: HTMLElement): HTMLElement {
  const cells = within(row).getAllByRole("cell");
  return cells[cells.length - 1] as HTMLElement;
}

function openRowMenu(row: HTMLElement) {
  fireEvent.pointerDown(within(row).getByTestId("row-menu-trigger"), { button: 0 });
}

describe("OrdersPage", () => {
  beforeEach(() => {
    vi.unstubAllGlobals();
    ordersState = {
      data: pages(sampleOrders),
      isLoading: false,
      isError: false,
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: mockFetchNextPage,
    };
    summaryState = { data: undefined, isLoading: false };
    confirmState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    shipState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    shipManualState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    completeState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    refundState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    reconcileState = { mutate: mockMutate, mutateAsync: mockMutateAsync, isPending: false };
    mockMutate.mockReset();
    mockMutateAsync.mockReset();
    mockFetchNextPage.mockReset();
    (toast.success as ReturnType<typeof vi.fn>).mockReset();
    (toast.error as ReturnType<typeof vi.fn>).mockReset();
  });

  it("renders the orders table with order number, buyer, product, amount, payment and shipping", async () => {
    render(<OrdersPage />);

    await waitFor(() => {
      expect(screen.getByText(/Buku A/)).toBeInTheDocument();
    });

    // The total is printed twice per row: once in the md+ column, once in the
    // stacked mobile summary.
    expect(screen.getAllByText("Rp115.000").length).toBeGreaterThanOrEqual(1);
    // Status column renders a badge per row. "Dikirim" is no longer also a
    // filter chip — the status filter is a select whose options are portalled
    // until opened — so this asserts the badge of a status the fixture has.
    expect(screen.getAllByText("Menunggu Pembayaran").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Selesai").length).toBeGreaterThanOrEqual(1);
  });

  // FR-33: buyer name is the primary label; no truncated-UUID label
  // (`...<last12chars>`) appears anywhere in the rendered output.
  it("renders the buyer's name and no truncated-UUID label appears anywhere", async () => {
    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    expect(screen.getAllByText("Siswa Uji A").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Siswa Uji B").length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText("Siswa Uji C").length).toBeGreaterThanOrEqual(1);
    expect(screen.queryByText("...s1")).toBeNull();
    expect(screen.queryByText(/^\.\.\./)).toBeNull();
    // The raw student_id is deliberately NOT in the row: it is a UUID that
    // meant nothing to anyone reading the table. It stays in the order detail.
    expect(screen.queryByText("s1")).toBeNull();
  });

  it("shows confirm as the row button and reconcile in the menu for pending orders", async () => {
    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr");
    expect(row).toBeTruthy();
    expect(within(row!).queryByRole("button", { name: /konfirmasi/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /rekonsiliasi/i })).not.toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /kirim/i })).not.toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /refund/i })).not.toBeInTheDocument();

    openRowMenu(row!);
    expect(await screen.findByRole("menuitem", { name: /rekonsiliasi/i })).toBeInTheDocument();
  });

  // Only one action is promoted to a button; anything else would put the admin
  // back in front of the button wall this row replaced.
  it("gives a payment_pending row exactly one primary button beside its menu", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr")!;
    const buttons = within(actionCell(row)).getAllByRole("button");

    expect(buttons).toHaveLength(2);
    expect(buttons[0].textContent).toMatch(/konfirmasi/i);
    expect(buttons[1]).toHaveAttribute("data-testid", "row-menu-trigger");
  });

  it("confirms an order after uploading a proof and shows success toast", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order confirmed" });

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    const row = screen.getByText(/Buku A/).closest("tr");
    const confirmButton = within(row!).getByRole("button", { name: /konfirmasi/i });
    fireEvent.click(confirmButton);

    // ConfirmOrderModal is mocked above; clicking its stub simulates the
    // admin having already uploaded a proof and pressed submit.
    fireEvent.click(screen.getByRole("button", { name: "Confirm-Stub" }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "o1", paymentProofUrl: "payment_proof/admin-1/proof.jpg" });
      expect(toast.success).toHaveBeenCalledWith("Pesanan dikonfirmasi.");
    });
  });

  it("reconciles an order from the row menu", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order reconciled" });

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    openRowMenu(screen.getByText(/Buku A/).closest("tr")!);
    fireEvent.click(await screen.findByRole("menuitem", { name: /rekonsiliasi/i }));

    await waitFor(() => expect(mockMutateAsync).toHaveBeenCalledWith("o1"));
  });

  // The tracking number used to be collected with window.prompt, which cannot be
  // styled, validated or cancelled cleanly, and blocks the whole tab.
  it("ships an order via the default Biteship auto-booking action", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order shipped" });

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /pesan kurir/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("o2");
      expect(toast.success).toHaveBeenCalledWith("Pesanan dikirim.");
    });
  });

  // The manual tracking-number field is an escape hatch, not the default path:
  // it must not be present until the admin explicitly asks for it.
  it("does not render the manual tracking-number field until manual entry is chosen", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByLabelText(/no\. resi/i)).toBeNull();
    expect(within(dialog).getByRole("button", { name: /pesan kurir/i })).toBeInTheDocument();
  });

  it("ships an order with a manually entered tracking number after choosing manual entry", async () => {
    mockMutateAsync.mockResolvedValueOnce({ message: "order shipped" });

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /masukkan nomor resi/i }));

    expect(within(dialog).getByLabelText(/no\. resi/i)).toBeInTheDocument();
    fireEvent.change(within(dialog).getByLabelText(/no\. resi/i), {
      target: { value: "JNE-123" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: /^kirim$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({ id: "o2", trackingNumber: "JNE-123" });
      expect(toast.success).toHaveBeenCalledWith("Pesanan dikirim.");
    });
  });

  it("will not submit an empty tracking number in manual mode", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /masukkan nomor resi/i }));

    expect(within(dialog).getByRole("button", { name: /^kirim$/i })).toBeDisabled();
  });

  // Track C's whole point is that a failed Biteship booking tells the admin the
  // real reason, not a generic "gagal" — otherwise the manual fallback exists
  // for nothing.
  it("renders the server's error message verbatim in the dialog when booking fails", async () => {
    mockMutateAsync.mockRejectedValueOnce(new Error("order has no persisted courier code"));

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /^kirim$/i }));

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /pesan kurir/i }));

    await waitFor(() => {
      expect(within(dialog).getByText("order has no persisted courier code")).toBeInTheDocument();
    });
    // Dialog stays open on failure — the admin can retry or fall back to manual.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  // The row was inert: an admin could see that an order existed but not who it
  // was going to, what was in it, or which courier had it.
  it("opens the order detail when a row is clicked", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    fireEvent.click(screen.getByText(/Buku Shipped/).closest("tr")!);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Jl. Contoh No. 1")).toBeInTheDocument();
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
      data: pages([
        {
          ...sampleOrders[1],
          shipping_address: {
            penerima: "Budi Test",
            alamat: "Jl. Contoh No. 1",
            provinsi_id: "32",
            kota_id: "3275",
            kecamatan_id: "327501",
            kode_pos: "17151",
          },
        },
      ]),
    };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    fireEvent.click(screen.getByText(/Buku Shipped/).closest("tr")!);

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Jl. Contoh No. 1")).toBeInTheDocument();
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
      ...ordersState,
      data: pages([{
        id: "o-merch",
        student_id: "s-merch",
        status: "processing",
        subtotal: 75000,
        discount: 0,
        shipping_cost: 15000,
        total: 90000,
        items: [{ id: "i-merch", order_id: "o-merch", product_id: "p-merch", product_type: "merchandise", name: "Kaos Akademi", unit_price: 75000, qty: 1, jumlah: 75000 }],
      }]),
    };

    const { rerender } = render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText("Kaos Akademi")).toBeInTheDocument());

    let row = screen.getByText("Kaos Akademi").closest("tr");
    expect(within(row!).getByRole("button", { name: /kirim/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /selesai/i })).not.toBeInTheDocument();

    ordersState = {
      ...ordersState,
      data: pages([{ ...ordersState.data!.pages[0].data[0], status: "shipped", tracking_number: "JNE-MERCH-1" }]),
    };
    rerender(<OrdersPage />);

    row = screen.getByText("Kaos Akademi").closest("tr");
    expect(within(row!).queryByRole("button", { name: /^kirim$/i })).not.toBeInTheDocument();
    expect(within(row!).getByRole("button", { name: /selesai/i })).toBeInTheDocument();
  });

  it("requires a medal order to ship before it can complete", async () => {
    ordersState = {
      ...ordersState,
      data: pages([{ id: "o-medal", student_id: "s-medal", status: "processing", subtotal: 75000, discount: 0, shipping_cost: 15000, total: 90000,
        items: [{ id: "i-medal", order_id: "o-medal", product_id: "p-medal", product_type: "medal", name: "Medali Emas", unit_price: 75000, qty: 1, jumlah: 75000 }] }]),
    };
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText("Medali Emas")).toBeInTheDocument());
    const row = screen.getByText("Medali Emas").closest("tr");
    expect(within(row!).getByRole("button", { name: /kirim/i })).toBeInTheDocument();
    expect(within(row!).queryByRole("button", { name: /selesai/i })).not.toBeInTheDocument();
  });

  // Completing is irreversible, so it asks first — but in a dialog, not a
  // window.confirm the page cannot style, translate or test.
  it("completes an order only after the confirm dialog is accepted", async () => {
    const confirmSpy = vi.fn(() => true);
    vi.stubGlobal("confirm", confirmSpy);
    mockMutateAsync.mockResolvedValueOnce({ message: "order completed" });

    ordersState = {
      ...ordersState,
      data: pages([{ ...sampleOrders[2], status: "processing" }]),
    };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Kursus B/)).toBeInTheDocument());

    const row = screen.getByText(/Kursus B/).closest("tr");
    fireEvent.click(within(row!).getByRole("button", { name: /selesai/i }));

    expect(confirmSpy).not.toHaveBeenCalled();
    expect(mockMutateAsync).not.toHaveBeenCalled();

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("Tandai pesanan selesai?")).toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: /^selesai$/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith("o3");
      expect(toast.success).toHaveBeenCalledWith("Pesanan selesai");
    });
  });

  it("does not complete an order when the confirm dialog is dismissed", async () => {
    ordersState = {
      ...ordersState,
      data: pages([{ ...sampleOrders[2], status: "processing" }]),
    };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Kursus B/)).toBeInTheDocument());

    fireEvent.click(
      within(screen.getByText(/Kursus B/).closest("tr")!).getByRole("button", { name: /selesai/i }),
    );

    const dialog = await screen.findByRole("dialog");
    fireEvent.click(within(dialog).getByRole("button", { name: /^batal$/i }));

    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(mockMutateAsync).not.toHaveBeenCalled();
  });

  // A refund is a manual bank transfer, so the action opens a modal demanding
  // the receipt rather than a yes/no confirm — the backend rejects the call
  // without one, and a window.confirm could never supply it.
  it("opens the refund modal instead of a confirm dialog, and warns that money is not returned automatically", async () => {
    const confirmSpy = vi.fn(() => true);
    vi.stubGlobal("confirm", confirmSpy);

    render(<OrdersPage />);

    // A paid order, not the completed one: refund is no longer offered once an
    // order is complete.
    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    const row = screen.getByText(/Buku Shipped/).closest("tr")!;
    // Refund is never promoted to a row button — it is the one action that
    // moves money, and it sits behind the menu with the destructive variant.
    expect(within(row).queryByRole("button", { name: /refund/i })).toBeNull();

    openRowMenu(row);
    const item = await screen.findByRole("menuitem", { name: /refund/i });
    expect(item).toHaveAttribute("data-variant", "destructive");
    fireEvent.click(item);

    // The old flow fired the mutation straight from a confirm(); the new one
    // cannot, because it has no receipt yet.
    expect(confirmSpy).not.toHaveBeenCalled();
    expect(mockMutateAsync).not.toHaveBeenCalled();

    // The modal opens instead; the receipt it collects is what finally sends
    // the mutation.
    const stub = await screen.findByRole("button", { name: "Refund-Stub" });
    fireEvent.click(stub);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        id: "o2",
        refundProofUrl: "refund_proof/admin-1/trf.jpg",
      });
      expect(toast.success).toHaveBeenCalledWith("Pesanan direfund.");
    });
  });

  it("filters rows by the status select", async () => {
    ordersState = {
      ...ordersState,
      data: pages(sampleOrders.filter((o) => o.status === "paid")),
    };

    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument());

    // The status filter is a select, not a row of chips: the chips wrapped to a
    // second line and never lined up with the search and date controls.
    expect(screen.getByTestId("orders-status-filter")).toBeInTheDocument();

    expect(screen.getByText(/Buku Shipped/)).toBeInTheDocument();
    expect(screen.queryByText(/Buku A/)).not.toBeInTheDocument();
  });

  // Refund on a finished order is a returns case, not a routine action.
  // Offering it on every historical row buried the states where it is the
  // right move.
  it("does not offer refund on a completed order", async () => {
    ordersState = {
      ...ordersState,
      data: pages([sampleOrders[2]]),
    };
    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Kursus B/)).toBeInTheDocument());

    const row = screen.getByText(/Kursus B/).closest("tr")!;
    expect(within(row).queryByRole("button", { name: /refund/i })).toBeNull();
    // Refund was this row's only remaining action, so the overflow menu is not
    // rendered at all — there is nothing left to put in it.
    expect(within(row).queryByTestId("row-menu-trigger")).toBeNull();
  });

  // ...unless the parcel died: money was taken for goods that will not arrive,
  // and orders.status is never walked back by the webhook.
  it("still offers refund on a completed order whose shipment failed", async () => {
    ordersState = {
      ...ordersState,
      data: pages([{ ...sampleOrders[2], shipment_status: "courierNotFound" }]),
    };
    render(<OrdersPage />);

    await waitFor(() => expect(screen.getByText(/Kursus B/)).toBeInTheDocument());

    const row = screen.getByText(/Kursus B/).closest("tr")!;
    openRowMenu(row);
    expect(await screen.findByRole("menuitem", { name: /refund/i })).toBeTruthy();
  });

  it("counts the rows on screen against the summary total", async () => {
    summaryState = {
      data: {
        buckets: {
          needs_confirm: 1,
          ready_to_ship: 1,
          shipment_failed: 0,
          in_transit: 0,
          created_this_month: 3,
          completed_this_month: 1,
          total: 42,
        },
        top_products: [],
      },
      isLoading: false,
    };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    expect(screen.getByText("Menampilkan 3 dari 42")).toBeInTheDocument();
  });

  it("asks for the next page when load more is clicked", async () => {
    ordersState = { ...ordersState, hasNextPage: true };

    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    fireEvent.click(screen.getByRole("button", { name: /muat lebih banyak/i }));

    expect(mockFetchNextPage).toHaveBeenCalledTimes(1);
  });

  it("hides load more when there is no next page", async () => {
    render(<OrdersPage />);
    await waitFor(() => expect(screen.getByText(/Buku A/)).toBeInTheDocument());

    expect(screen.queryByRole("button", { name: /muat lebih banyak/i })).toBeNull();
  });

  it("surfaces an API error as inline error text", async () => {
    ordersState = {
      ...ordersState,
      data: undefined,
      isError: true,
      error: new Error("gagal memuat"),
    };

    render(<OrdersPage />);

    await waitFor(() => {
      expect(screen.getByText(/gagal memuat/i)).toBeInTheDocument();
    });
  });
});

describe("OrdersPage failed shipments", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    summaryState = { data: undefined, isLoading: false };
    ordersState = {
      data: pages([
        {
          ...sampleOrders[0],
          id: "00000000-0000-0000-0000-0000deadbeef",
          status: "shipped",
          tracking_number: "JP999",
          shipment_status: "courierNotFound",
        } as Order,
      ]),
      isLoading: false,
      isError: false,
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: mockFetchNextPage,
    };
  });

  // The row still says "shipped" — the webhook never walks orders.status back
  // (FR-C-15) — so without this the listing gives an admin no reason to ever
  // open the order whose parcel is dead.
  it("marks a dead shipment in the listing instead of a plain Dikirim", async () => {
    render(<OrdersPage />);

    const status = await screen.findByTestId("row-shipment-status");
    expect(status).toHaveClass("text-destructive");
    // A dead parcel has no waybill worth querying, so the track affordance goes.
    expect(screen.queryByTestId("row-track-button")).toBeNull();
  });

  it("offers a filter for failed shipments", async () => {
    render(<OrdersPage />);
    // Options live inside the select's portal until it is opened, so the
    // presence of the control plus its option list is what is asserted here.
    // The status filter is a select. Its options live in a Radix portal that
    // jsdom will not open without pointer-capture shims, so this asserts the
    // control is present; OrdersToolbar.test.tsx covers the option list.
    expect(await screen.findByTestId("orders-status-filter")).toBeTruthy();
  });
});

describe("OrdersPage shipment status", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    summaryState = { data: undefined, isLoading: false };
  });

  function renderWithStatus(shipment_status: string | null, tracking_number?: string) {
    ordersState = {
      data: pages([
        {
          ...sampleOrders[0],
          id: "00000000-0000-0000-0000-00000000c01a",
          status: "shipped",
          tracking_number,
          shipment_status,
        } as Order,
      ]),
      isLoading: false,
      isError: false,
      error: null,
      hasNextPage: false,
      isFetchingNextPage: false,
      fetchNextPage: mockFetchNextPage,
    };
    render(<OrdersPage />);
  }

  it("shows the courier status in the listing, translated", async () => {
    renderWithStatus("in_transit", "JP777");
    expect((await screen.findByTestId("row-track-button")).textContent).toContain(
      "Dalam perjalanan",
    );
  });

  it("reads the camelCase spelling the same way", async () => {
    renderWithStatus("droppingOff", "JP777");
    expect((await screen.findByTestId("row-track-button")).textContent).toContain(
      "Menuju alamat penerima",
    );
  });

  // An order booked but not yet acknowledged by Biteship has no status at all;
  // an em dash says "nothing yet" without pretending it failed.
  it("shows a placeholder when no status has arrived yet", async () => {
    renderWithStatus(null);
    expect((await screen.findByTestId("row-shipment-status")).textContent).toBe("—");
  });
});
