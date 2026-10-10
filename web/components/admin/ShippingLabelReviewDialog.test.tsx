import * as React from "react";
import { render, screen, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ShippingLabelReviewDialog } from "./ShippingLabelReviewDialog";
import { useUIStore } from "@/stores/ui";
import type { Order } from "@/lib/types";

const download = vi.fn();
const downloadSingle = vi.fn();
vi.mock("@/lib/hooks/shipping-label", () => ({ downloadShippingLabel: (...args: unknown[]) => downloadSingle(...args), downloadShippingLabels: (...args: unknown[]) => download(...args) }));
const order = (id: string, tracking = "TEST123", status: string | null = null): Order => ({ id, student_id: "student-test", student_name: "Buyer Test", status: "shipped", subtotal: 100, discount: 0, shipping_cost: 0, total: 100, tracking_number: tracking, shipment_status: status, items: [] });

beforeEach(() => { download.mockReset(); downloadSingle.mockReset(); useUIStore.setState({ lang: "id" }); });

describe("ShippingLabelReviewDialog", () => {
  it.each(["id", "en"] as const)("single mode keeps a failed shipment printable in %s", async (lang) => {
    useUIStore.setState({ lang });
    downloadSingle.mockResolvedValue(undefined);
    const onClose = vi.fn();
    render(<ShippingLabelReviewDialog mode="single" orders={[order("failed", "TEST456", "returned")]} onClose={onClose} />);
    expect(screen.queryByText(lang === "id" ? "Pengiriman gagal" : "Shipment failed")).toBeNull();
    await userEvent.click(screen.getByRole("radio", { name: /80 mm/ }));
    await userEvent.click(screen.getByRole("button", { name: lang === "id" ? "Unduh 1 resi PDF" : "Download 1-slip PDF" }));
    await waitFor(() => expect(screen.getByRole("button", { name: /PDF/ })).toBeEnabled());
    expect(downloadSingle).toHaveBeenCalledWith("failed", "roll80");
    expect(download).not.toHaveBeenCalled();
    expect(onClose).toHaveBeenCalledOnce();
  });
  it("shows buyers and waybills for an all-printable batch", () => {
    render(<ShippingLabelReviewDialog orders={[order("first"), order("second", "TEST456")]} onClose={vi.fn()} />);
    expect(screen.getAllByText(/Buyer Test/)).toHaveLength(2);
    expect(screen.getByText("TEST123")).toBeInTheDocument();
    expect(screen.getByText("TEST456")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Unduh 2 resi PDF" })).toBeEnabled();
    expect(screen.getByRole("radio", { name: /100 × 150/ })).toBeChecked();
    expect(screen.getByText(/78 mm/)).toBeInTheDocument();
  });

  it("lists every skipped row and refuses zero printable", () => {
    render(<ShippingLabelReviewDialog orders={[order("missing", ""), order("failed", "TEST456", "courierNotFound")]} onClose={vi.fn()} />);
    expect(screen.getByText(/^Dipilih:/)).toHaveTextContent("Dipilih: 2");
    expect(screen.getByText(/^Siap:/)).toHaveTextContent("Siap: 0");
    expect(screen.getByText("Nomor resi belum tersedia")).toBeInTheDocument();
    expect(screen.getByText("Pengiriman gagal")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ })).toBeDisabled();
  });

  it("sends printable ids in row order with the chosen size and prevents duplicate requests", async () => {
    let resolve!: (value: null) => void;
    download.mockReturnValue(new Promise<null>((done) => { resolve = done; }));
    const onClose = vi.fn();
    render(<ShippingLabelReviewDialog orders={[order("second"), order("skip", ""), order("first")]} onClose={onClose} />);
    await userEvent.click(screen.getByRole("radio", { name: /80 mm/ }));
    const button = screen.getByRole("button", { name: /Unduh \d+ resi PDF/ });
    await userEvent.click(button);
    expect(button).toBeDisabled();
    await userEvent.click(button);
    const dialog = screen.getByRole("dialog");
    for (let i = 0; i < 4; i++) { await userEvent.tab(); expect(dialog.contains(document.activeElement)).toBe(true); }
    await userEvent.keyboard("{Escape}");
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Batal" })).toBeDisabled();
    expect(download).toHaveBeenCalledTimes(1);
    expect(download).toHaveBeenCalledWith(["second", "first"], "roll80");
    resolve(null);
    await waitFor(() => expect(button).toBeEnabled());
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("keeps the review open after 409 and uses the fresh reasons on retry", async () => {
    download.mockResolvedValueOnce([{ order_id: "second", reason: "not_found" }]).mockResolvedValueOnce(null);
    const onClose = vi.fn();
    render(<ShippingLabelReviewDialog orders={[order("second"), order("first")]} onClose={onClose} />);
    await userEvent.click(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Pesanan berubah. Tinjau kembali sebelum mencetak.");
    expect(screen.getByText(/^Siap:/)).toHaveTextContent("Siap: 1");
    expect(screen.getByText("Pesanan tidak ditemukan")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ }));
    await waitFor(() => expect(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ })).toBeEnabled());
    expect(download).toHaveBeenLastCalledWith(["first"], "100x150");
  });

  it("disables confirmation when a fresh 409 leaves no printable orders", async () => {
    download.mockResolvedValueOnce([{ order_id: "first", reason: "no_tracking_number" }]);
    render(<ShippingLabelReviewDialog orders={[order("first")]} onClose={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ }));
    await waitFor(() => expect(screen.getByText(/^Siap:/)).toHaveTextContent("Siap: 0"));
    expect(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ })).toBeDisabled();
  });

  it("shows a request error and allows retry", async () => {
    download.mockRejectedValueOnce(new Error("unavailable")).mockResolvedValueOnce(null);
    render(<ShippingLabelReviewDialog orders={[order("first")]} onClose={vi.fn()} />);
    await userEvent.click(screen.getByRole("button", { name: /Unduh \d+ resi PDF/ }));
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Coba lagi — Unduh 1 resi PDF" })).toBeEnabled();
    await userEvent.click(screen.getByRole("button", { name: "Coba lagi — Unduh 1 resi PDF" }));
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());
    expect(download).toHaveBeenCalledTimes(2);
  });

  it("renders the English review and reasons", () => {
    useUIStore.setState({ lang: "en" });
    render(<ShippingLabelReviewDialog orders={[order("failed", "TEST456", "courier_not_found"), order("missing", "")]} onClose={vi.fn()} />);
    expect(screen.getByRole("dialog", { name: "Review packing slips" })).toBeInTheDocument();
    expect(screen.getByText(/^Printable:/)).toHaveTextContent("Printable: 0");
    expect(screen.getByText("Shipment failed")).toBeInTheDocument();
    expect(screen.getByText("Tracking number not available")).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Label size" })).toBeInTheDocument();
    useUIStore.setState({ lang: "id" });
  });

  it("traps focus and restores it to the print button", async () => {
    function Harness() {
      const [open, setOpen] = React.useState(false);
      return <><button onClick={() => setOpen(true)}>Print trigger</button>{open && <ShippingLabelReviewDialog orders={[order("first")]} onClose={() => setOpen(false)} />}</>;
    }
    render(<Harness />);
    const trigger = screen.getByRole("button", { name: "Print trigger" });
    await userEvent.click(trigger);
    const dialog = screen.getByRole("dialog");
    for (let i = 0; i < 8; i++) { await userEvent.tab(); expect(dialog.contains(document.activeElement)).toBe(true); }
    await userEvent.click(within(dialog).getByRole("button", { name: "Batal" }));
    await waitFor(() => expect(trigger).toHaveFocus());
  });
});
