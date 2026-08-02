import { render as rtlRender, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";
import { OrderDetailModal } from "./OrderDetailModal";

// The proof link mints its short-lived URL through a react-query mutation
// rather than rendering a /files/* href, so the modal now needs a client.
function render(ui: ReactElement) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return rtlRender(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>);
}

const physicalOrder = {
  id: "o1",
  status: "paid",
  subtotal: 10000,
  discount: 0,
  total: 10000,
  shipping_cost: 15000,
  selected_courier: "JNE",
  selected_service: "REG",
  student_id: "s1",
  items: [{ id: "i1", order_id: "o1", product_id: "p1", product_type: "book", name: "Buku", unit_price: 1, qty: 1, jumlah: 1 }],
} as any;

describe("OrderDetailModal", () => {
  // The old check inferred the badge from selected_courier; both cases below
  // are where that inference gets it backwards.
  it("flags an estimate even when the courier name looks like a real carrier", () => {
    const estimate = { ...physicalOrder, is_estimate: true, selected_courier: "JNE" };
    render(<OrderDetailModal order={estimate} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Estimasi — bukan tarif kurir")).toBeTruthy();
  });

  it("does not flag a real quote even when the courier name is the fallback label", () => {
    const real = { ...physicalOrder, is_estimate: false, selected_courier: "Ongkir Flat" };
    render(<OrderDetailModal order={real} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Estimasi — bukan tarif kurir")).toBeNull();
  });

  it("shows the Cetak Resi print action when tracking_number is present", () => {
    const shipped = { ...physicalOrder, tracking_number: "JP1234567" };
    render(<OrderDetailModal order={shipped} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Cetak Resi")).toBeTruthy();
  });

  it("hides the Cetak Resi print action when tracking_number is empty", () => {
    const unshipped = { ...physicalOrder, tracking_number: "" };
    render(<OrderDetailModal order={unshipped} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Cetak Resi")).toBeNull();
  });

  it("shows waybill_source next to the resi, admin-only", () => {
    const biteshipShipped = {
      ...physicalOrder,
      tracking_number: "JP1234567",
      waybill_source: "biteship",
    };
    render(<OrderDetailModal order={biteshipShipped} onOpenChange={vi.fn()} />);
    expect(screen.getByText(/Booking otomatis \(Biteship\)/)).toBeTruthy();
  });

  it("renders the shipment timeline with events ordered newest-first", () => {
    const withEvents = {
      ...physicalOrder,
      tracking_number: "JP1234567",
      shipment_events: [
        {
          id: "e1",
          order_id: "o1",
          status: "confirmed",
          occurred_at: "2026-07-20T01:00:00Z",
          created_at: "2026-07-20T01:00:05Z",
        },
        {
          id: "e2",
          order_id: "o1",
          status: "delivered",
          occurred_at: "2026-07-22T01:00:00Z",
          created_at: "2026-07-22T01:00:05Z",
        },
      ],
    };
    render(<OrderDetailModal order={withEvents} onOpenChange={vi.fn()} />);
    const statuses = screen.getAllByTestId("shipment-event-status").map((el) => el.textContent);
    expect(statuses).toEqual(["delivered", "confirmed"]);
  });

  it("renders no timeline heading for an order with no shipment events", () => {
    const noEvents = { ...physicalOrder, tracking_number: "JP1234567", shipment_events: [] };
    render(<OrderDetailModal order={noEvents} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Riwayat Pengiriman")).toBeNull();
  });

  // FR-33: buyer name is primary, student_id stays visible as secondary detail
  // — no truncated-UUID label ("...<last12chars>") anywhere in the output.
  it("renders the buyer's name with student_id as secondary detail, never a truncated-UUID label", () => {
    const withName = { ...physicalOrder, student_id: "11111111-2222-3333-4444-555555555555", student_name: "Rina Ujian" };
    render(<OrderDetailModal order={withName} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Rina Ujian")).toBeTruthy();
    expect(screen.getByText("11111111-2222-3333-4444-555555555555")).toBeTruthy();
    expect(screen.queryByText(/^\.\.\./)).toBeNull();
  });

  // FR-31: an order manually confirmed shows the mark and the proof opens from
  // the detail view — but never as a bare /files/* href. The object key must
  // not be rendered into the page at all; the link is minted on demand, so the
  // control is a button and the raw key never appears in the DOM.
  it("shows the Dikonfirmasi manual mark and opens the proof without exposing the object key", () => {
    const manual = {
      ...physicalOrder,
      payment_method: "manual",
      payment_proof_url: "payment_proof/admin-1/proof.jpg",
    };
    const { container } = render(<OrderDetailModal order={manual} onOpenChange={vi.fn()} />);
    expect(screen.getByText("Dikonfirmasi manual")).toBeTruthy();

    expect(screen.getByRole("button", { name: "Lihat bukti" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Lihat bukti" })).toBeNull();
    expect(container.innerHTML).not.toContain("payment_proof/admin-1/proof.jpg");
  });

  it("hides the manual-confirm mark for a non-manual payment_method", () => {
    const gateway = { ...physicalOrder, payment_method: "gopay" };
    render(<OrderDetailModal order={gateway} onOpenChange={vi.fn()} />);
    expect(screen.queryByText("Dikonfirmasi manual")).toBeNull();
  });
});
