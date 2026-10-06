import { describe, it, expect, vi, beforeAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act, within } from "@testing-library/react";
import { OrdersToolbar } from "./OrdersToolbar";
import { useUIStore } from "@/stores/ui";
import type { AdminOrderQuery, OrderBucketCounts } from "@/lib/types";

// Radix Select drives its trigger through Pointer Events APIs that jsdom does
// not implement. Without these the listbox never opens and the option list
// cannot be asserted at all.
beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

const counts: OrderBucketCounts = {
  needs_confirm: 4,
  ready_to_ship: 7,
  shipment_failed: 1,
  in_transit: 12,
  completed: 23,
  payment_expired: 5,
  cancelled: 6,
  created_this_month: 128,
  completed_this_month: 96,
  total: 1284,
};

function renderToolbar(value: AdminOrderQuery = { status: "all" }, onChange = vi.fn()) {
  render(<OrdersToolbar value={value} onChange={onChange} counts={counts} />);
  return { onChange };
}

async function openStatusMenu() {
  fireEvent.pointerDown(screen.getByTestId("orders-status-filter"), {
    button: 0,
    ctrlKey: false,
    pointerType: "mouse",
  });
  return screen.findByRole("listbox");
}

afterEach(() => act(() => useUIStore.setState({ lang: "id" })));

const filterCases = [
  ["all", "Semua", "All", 1284],
  ["pending", "Menunggu pembayaran", "Awaiting payment", 4],
  ["ready_to_ship", "Siap kirim", "Ready to ship", 7],
  ["shipment_failed", "Pengiriman bermasalah", "Delivery problem", 1],
  ["in_transit", "Dalam pengiriman", "In transit", 12],
  ["completed", "Selesai", "Completed", 23],
  ["payment_expired", "Kedaluwarsa", "Payment expired", 5],
  ["cancelled", "Dibatalkan", "Cancelled", 6],
] as const;

describe("OrdersToolbar status filter", () => {
  it.each(["id", "en"] as const)("offers exactly eight ordered options with counts in %s", async (lang) => {
    useUIStore.setState({ lang });
    renderToolbar();
    const listbox = await openStatusMenu();
    expect(within(listbox).getAllByRole("option").map((option) => option.textContent))
      .toEqual(filterCases.map(([, id, en, count]) => `${lang === "id" ? id : en}${count}`));
  });

  it.each(filterCases)("selecting %s clears the other filter dimension", async (option, label) => {
    const onChange = vi.fn();
    const isQueue = ["ready_to_ship", "shipment_failed", "in_transit"].includes(option);
    renderToolbar(isQueue ? { status: "pending" } : { status: "all", queue: "shipment_failed" }, onChange);
    await openStatusMenu();
    fireEvent.click(screen.getByRole("option", { name: new RegExp(label) }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining(
      isQueue ? { status: "all", queue: option } : { status: option, queue: undefined },
    ));
  });

  it("shows the queue-selected filter as the trigger's selected value", () => {
    renderToolbar({ status: "all", queue: "ready_to_ship" });
    expect(screen.getByTestId("orders-status-filter")).toHaveTextContent("Siap kirim");
  });
});

describe("OrdersToolbar date mode", () => {
  it("defaults to single-date and emits the same day for from and to", async () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    expect(screen.getByTestId("orders-date-mode-single")).toHaveAttribute("aria-pressed", "true");

    fireEvent.change(document.getElementById("orders-date-single")!, { target: { value: "2026-08-02" } });

    // One calendar day is from === to; the API's upper bound is exclusive.
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: "2026-08-02", to: "2026-08-02" }),
    );
  });

  it("clearing the single date drops both bounds", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all", from: "2026-08-02", to: "2026-08-02" }, onChange);

    fireEvent.change(document.getElementById("orders-date-single")!, { target: { value: "" } });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: undefined, to: undefined }),
    );
  });

  it("range mode exposes both bounds independently", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    fireEvent.click(screen.getByTestId("orders-date-mode-range"));

    fireEvent.change(document.getElementById("orders-date-from")!, { target: { value: "2026-08-01" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ from: "2026-08-01" }));

    fireEvent.change(document.getElementById("orders-date-to")!, { target: { value: "2026-08-31" } });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ to: "2026-08-31" }));
  });

  // An open-ended range has no single day, so switching back must not leave a
  // `to` behind that the single input cannot show.
  it("collapses a range to its start date when switching back to single", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all", from: "2026-08-01", to: "2026-08-31" }, onChange);

    // A two-date value opens in range mode rather than silently collapsing.
    expect(screen.getByTestId("orders-date-mode-range")).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByTestId("orders-date-mode-single"));
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: "2026-08-01", to: "2026-08-01" }),
    );
  });
});

describe("OrdersToolbar search", () => {
  it("debounces before emitting, so a keystroke is not a request", async () => {
    vi.useFakeTimers();
    const onChange = vi.fn();
    render(<OrdersToolbar value={{ status: "all" }} onChange={onChange} counts={counts} />);

    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "rani" } });
    expect(onChange).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining({ q: "rani" }));
    vi.useRealTimers();
  });
});


it("leaves page size to the table footer", () => {
  renderToolbar();
  expect(screen.queryByRole("combobox", { name: "Pesanan per halaman" })).not.toBeInTheDocument();
});

describe("OrdersToolbar product, courier and sort", () => {
  it("selects a product type without dropping the other filters", async () => {
    const { onChange } = renderToolbar({ status: "pending", courier: "jne" });
    fireEvent.pointerDown(screen.getByRole("combobox", { name: "Jenis produk" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
    await screen.findByRole("listbox");
    fireEvent.click(screen.getByRole("option", { name: "Medali" }));
    expect(onChange).toHaveBeenCalledWith({ status: "pending", courier: "jne", type: "medal" });
  });
});

it.each([
  ["book", "Buku"], ["merchandise", "Merchandise"], ["medal", "Medali"], ["course", "Kursus"], ["exam", "Ujian"], [undefined, "Semua jenis"],
] as const)("product option %s sends the product type", async (type, label) => {
  const { onChange } = renderToolbar({ status: "all", type: "medal" });
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Jenis produk" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  await screen.findByRole("listbox");
  fireEvent.click(screen.getByRole("option", { name: label }));
  if (type !== "medal") expect(onChange).toHaveBeenCalledWith({ status: "all", type });
});

it.each(["jne", "jnt", undefined])("courier %s sends the original code", async (courier) => {
  const onChange = vi.fn();
  render(<OrdersToolbar value={{ status: "all", courier: "old_courier" }} onChange={onChange} couriers={["jne", "jnt"]} />);
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Kurir" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual(["Semua kurir", "JNE", "JNT"]);
  fireEvent.click(screen.getByRole("option", { name: courier?.toUpperCase() ?? "Semua kurir" }));
  expect(onChange).toHaveBeenCalledWith({ status: "all", courier });
});

it.each([
  ["order_at", "Tanggal pesanan"], ["paid_at", "Tanggal bayar"], ["item_qty", "Jumlah barang"],
] as const)("sort %s sends a fixed key", async (sort, label) => {
  const { onChange } = renderToolbar({ status: "all", sort: sort === "order_at" ? "paid_at" : "order_at", dir: "asc" });
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Urutkan pesanan" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  await screen.findByRole("listbox");
  fireEvent.click(screen.getByRole("option", { name: label }));
  expect(onChange).toHaveBeenCalledWith({ status: "all", sort, dir: "asc" });
});

it.each([undefined, "asc", "desc"] as const)("toggles sort direction from %s", (dir) => {
  const { onChange } = renderToolbar({ status: "all", sort: "item_qty", dir });
  const button = screen.getByRole("button", { name: dir === "asc" ? "Arah urutan: Naik" : "Arah urutan: Turun" });
  expect(button).toHaveTextContent(dir === "asc" ? "↑ Naik" : "↓ Turun");
  fireEvent.click(button);
  expect(onChange).toHaveBeenCalledWith({ status: "all", sort: "item_qty", dir: dir === "asc" ? "desc" : "asc" });
});

it.each(["id", "en"] as const)("has accessible translated controls and options in %s", async (lang) => {
  useUIStore.setState({ lang });
  renderToolbar();
  const labels = lang === "id"
    ? ["Jenis produk", "Kurir", "Urutkan pesanan", "Arah urutan"]
    : ["Product type", "Courier", "Sort orders", "Sort direction"];
  labels.slice(0, 3).forEach((label) => expect(screen.getByRole("combobox", { name: label })).toBeInTheDocument());
  expect(screen.getByRole("combobox", { name: labels[0] })).toHaveTextContent(lang === "id" ? "Semua jenis" : "All types");
  expect(screen.getByRole("combobox", { name: labels[1] })).toHaveTextContent(lang === "id" ? "Semua kurir" : "All couriers");
  expect(screen.getByRole("button", { name: `${labels[3]}: ${lang === "id" ? "Turun" : "Descending"}` })).toHaveTextContent(lang === "id" ? "Turun" : "Descending");
  fireEvent.pointerDown(screen.getByRole("combobox", { name: labels[2] }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual(lang === "id"
    ? ["Tanggal pesanan", "Tanggal bayar", "Jumlah barang"] : ["Order date", "Paid date", "Item count"]);
});
