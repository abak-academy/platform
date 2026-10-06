import { describe, it, expect, vi, beforeAll, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor, act, within } from "@testing-library/react";
import { OrdersToolbar } from "./OrdersToolbar";
import { useUIStore } from "@/stores/ui";
import type { AdminOrderQuery } from "@/lib/types";

// Radix Select drives its trigger through Pointer Events APIs that jsdom does
// not implement. Without these the listbox never opens and the option list
// cannot be asserted at all.
beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

function renderToolbar(value: AdminOrderQuery = { status: "all" }, onChange = vi.fn()) {
  render(<OrdersToolbar value={value} onChange={onChange} />);
  return { onChange };
}

function openPanel() {
  fireEvent.click(screen.getByRole("button", { name: /^Filters?(?: \(|$)/ }));
}

async function openStatusMenu() {
  openPanel();
  fireEvent.pointerDown(screen.getByTestId("orders-status-filter"), {
    button: 0,
    ctrlKey: false,
    pointerType: "mouse",
  });
  return screen.findByRole("listbox");
}

afterEach(() => act(() => useUIStore.setState({ lang: "id" })));

const filterCases = [
  ["all", "Semua", "All"],
  ["pending", "Menunggu pembayaran", "Awaiting payment"],
  ["ready_to_ship", "Siap kirim", "Ready to ship"],
  ["shipped", "Sedang dalam pengiriman", "In transit"],
  ["completed", "Selesai", "Completed"],
  ["payment_expired", "Kedaluwarsa", "Payment expired"],
  ["cancelled", "Dibatalkan", "Cancelled"],
] as const;

describe("OrdersToolbar status filter", () => {
  it.each(["id", "en"] as const)("offers exactly seven ordered options without counts in %s", async (lang) => {
    useUIStore.setState({ lang });
    renderToolbar();
    const listbox = await openStatusMenu();
    expect(within(listbox).getAllByRole("option").map((option) => option.textContent))
      .toEqual(filterCases.map(([, id, en]) => (lang === "id" ? id : en)));
  });

  it.each([
    [{ status: "all", queue: "shipment_failed" }, "Pengiriman bermasalah"],
    [{ status: "all", queue: "in_transit" }, "Sedang dalam pengiriman"],
    [{ status: "paid" }, "Dibayar"],
    [{ status: "processing" }, "Diproses"],
  ] as const)("shows the label of a deep-linked value that has no option: %j", (value, label) => {
    renderToolbar(value as AdminOrderQuery);
    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    expect(screen.getByTestId("orders-status-filter")).toHaveTextContent(label);
  });

  it.each(filterCases)("selecting %s clears the other filter dimension", async (option, label) => {
    const onChange = vi.fn();
    const isQueue = ["ready_to_ship"].includes(option);
    renderToolbar(isQueue ? { status: "pending" } : { status: "all", queue: "shipment_failed" }, onChange);
    await openStatusMenu();
    fireEvent.click(screen.getByRole("option", { name: new RegExp(label) }));
    expect(onChange).toHaveBeenCalledWith(expect.objectContaining(
      isQueue ? { status: "all", queue: option } : { status: option, queue: undefined },
    ));
  });

  it("shows the queue-selected filter as the trigger's selected value", () => {
    renderToolbar({ status: "all", queue: "ready_to_ship" });
    openPanel();
    expect(screen.getByTestId("orders-status-filter")).toHaveTextContent("Siap kirim");
  });
});

describe("OrdersToolbar date mode", () => {
  it("defaults to single-date and emits the same day for from and to", async () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    openPanel();
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

    openPanel();
    fireEvent.change(document.getElementById("orders-date-single")!, { target: { value: "" } });

    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ from: undefined, to: undefined }),
    );
  });

  it("range mode exposes both bounds independently", () => {
    const onChange = vi.fn();
    renderToolbar({ status: "all" }, onChange);

    openPanel();
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

    openPanel();
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
    render(<OrdersToolbar value={{ status: "all" }} onChange={onChange} />);

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
  expect(screen.queryByRole("combobox", { name: "Baris per halaman" })).not.toBeInTheDocument();
});

describe("OrdersToolbar product, courier and sort", () => {
  it("selects a product type without dropping the other filters", async () => {
    const { onChange } = renderToolbar({ status: "pending", courier: "jne" });
    openPanel();
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
  openPanel();
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Jenis produk" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  await screen.findByRole("listbox");
  fireEvent.click(screen.getByRole("option", { name: label }));
  if (type !== "medal") expect(onChange).toHaveBeenCalledWith({ status: "all", type });
});

it.each(["jne", "jnt", undefined])("courier %s sends the original code", async (courier) => {
  const onChange = vi.fn();
  render(<OrdersToolbar value={{ status: "all", courier: "old_courier" }} onChange={onChange} couriers={["jne", "jnt"]} />);
  openPanel();
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Kurir" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual(["Semua kurir", "JNE", "JNT"]);
  fireEvent.click(screen.getByRole("option", { name: courier?.toUpperCase() ?? "Semua kurir" }));
  expect(onChange).toHaveBeenCalledWith({ status: "all", courier });
});


const sortCases = [
  ["order_at", "desc", "Tanggal pesanan · terbaru", "Order date · newest"],
  ["order_at", "asc", "Tanggal pesanan · terlama", "Order date · oldest"],
  ["paid_at", "desc", "Tanggal bayar · terbaru", "Paid date · newest"],
  ["paid_at", "asc", "Tanggal bayar · terlama", "Paid date · oldest"],
  ["item_qty", "desc", "Jumlah barang · terbanyak", "Item count · most"],
  ["item_qty", "asc", "Jumlah barang · tersedikit", "Item count · fewest"],
] as const;

it.each(sortCases)("sort %s %s sends both query fields", async (sort, dir, label) => {
  const { onChange } = renderToolbar({ status: "all", sort: sort === "paid_at" ? "item_qty" : "paid_at", dir: "asc" });
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Urutkan pesanan" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  await screen.findByRole("listbox");
  fireEvent.click(screen.getByRole("option", { name: label }));
  expect(onChange).toHaveBeenCalledWith({ status: "all", sort, dir });
});

it.each(["id", "en"] as const)("has translated panel actions and six combined sorts in %s", async (lang) => {
  useUIStore.setState({ lang });
  renderToolbar();
  const sortLabel = lang === "id" ? "Urutkan pesanan" : "Sort orders";
  expect(screen.getByRole("combobox", { name: sortLabel })).toHaveTextContent(lang === "id" ? sortCases[0][2] : sortCases[0][3]);
  fireEvent.pointerDown(screen.getByRole("combobox", { name: sortLabel }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  expect(within(listbox).getAllByRole("option").map((option) => option.textContent)).toEqual(sortCases.map((c) => c[lang === "id" ? 2 : 3]));
  fireEvent.keyDown(listbox, { key: "Escape" });
  fireEvent.click(screen.getByRole("button", { name: lang === "id" ? "Filter" : "Filters" }));
  expect(screen.getByRole("dialog", { name: lang === "id" ? "Filter pesanan" : "Filter orders" })).toBeInTheDocument();
  expect(screen.getByRole("combobox", { name: lang === "id" ? "Jenis produk" : "Product type" })).toHaveTextContent(lang === "id" ? "Semua jenis" : "All types");
  expect(screen.getByRole("combobox", { name: lang === "id" ? "Kurir" : "Courier" })).toHaveTextContent(lang === "id" ? "Semua kurir" : "All couriers");
  expect(screen.getByRole("button", { name: lang === "id" ? "Reset filter" : "Reset filters" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: lang === "id" ? "Tampilkan pesanan" : "Show orders" })).toBeInTheDocument();
});

it("keeps filters in a panel outside the primary row", () => {
  renderToolbar({ status: "all", q: "test", sort: "item_qty" });
  expect(screen.getByRole("button", { name: "Filter" })).toHaveTextContent(/^Filter$/);
  expect(screen.queryByRole("combobox", { name: "Status" })).not.toBeInTheDocument();
  expect(screen.getAllByRole("combobox")).toHaveLength(1);
  expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
});

it("chooses a nested select option without dismissing the panel", async () => {
  const { onChange } = renderToolbar();
  openPanel();
  fireEvent.pointerDown(screen.getByRole("combobox", { name: "Jenis produk" }), { button: 0, ctrlKey: false, pointerType: "mouse" });
  const listbox = await screen.findByRole("listbox");
  fireEvent.click(within(listbox).getByRole("option", { name: "Medali" }));
  await waitFor(() => expect(screen.queryByRole("listbox")).not.toBeInTheDocument());
  expect(screen.getByRole("dialog", { name: "Filter pesanan" })).toBeInTheDocument();
  expect(onChange).toHaveBeenCalledWith({ status: "all", type: "medal" });
});

it.each(["escape", "outside", "show"])("closes the panel with %s", async (action) => {
  renderToolbar();
  openPanel();
  const panel = screen.getByRole("dialog");
  if (action === "escape") fireEvent.keyDown(panel, { key: "Escape" });
  if (action === "outside") {
    await act(() => new Promise((resolve) => setTimeout(resolve, 0)));
    fireEvent.pointerDown(document.body, { button: 0, pointerType: "mouse" });
    fireEvent.click(document.body);
  }
  if (action === "show") fireEvent.click(screen.getByRole("button", { name: "Tampilkan pesanan" }));
  await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  expect(screen.getByRole("button", { name: "Filter" })).toHaveAttribute("aria-expanded", "false");
});

const activeQuery: AdminOrderQuery = {
  status: "all", queue: "ready_to_ship", type: "book", courier: "jne",
  from: "2026-08-01", to: "2026-08-31", q: "test", sort: "paid_at", dir: "asc", limit: 50,
};

it.each([
  ["Status: Siap kirim", { status: "all", queue: undefined }],
  ["Jenis produk: Buku", { type: undefined }],
  ["Kurir: JNE", { courier: undefined }],
  ["Tanggal: 2026-08-01 – 2026-08-31", { from: undefined, to: undefined }],
])("removes only the %s chip", (label, cleared) => {
  const { onChange } = renderToolbar(activeQuery);
  expect(screen.getByRole("button", { name: "Filter (4)" })).toHaveTextContent("4");
  const chips = screen.getByRole("group", { name: "Filter aktif" });
  expect(within(chips).getAllByRole("button")).toHaveLength(4);
  fireEvent.click(within(chips).getByRole("button", { name: `Hapus filter: ${label}` }));
  expect(onChange).toHaveBeenCalledWith({ ...activeQuery, ...cleared });
});

it.each([
  [{ status: "pending" as const }, "Status: Menunggu pembayaran"],
  [{ from: "2026-08-02", to: "2026-08-02" }, "Tanggal: 2026-08-02"],
  [{ from: "2026-08-02" }, "Tanggal: 2026-08-02 – …"],
  [{ to: "2026-08-02" }, "Tanggal: … – 2026-08-02"],
])("shows an active status or date chip %s", (fields, label) => {
  renderToolbar({ status: "all", ...fields });
  expect(screen.getByRole("button", { name: `Hapus filter: ${label}` })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Filter (1)" })).toBeInTheDocument();
});

it("resets filters while retaining search, sort and page size", () => {
  const { onChange } = renderToolbar(activeQuery);
  openPanel();
  fireEvent.click(screen.getByRole("button", { name: "Reset filter" }));
  expect(onChange).toHaveBeenCalledWith({
    ...activeQuery, status: "all", queue: undefined, type: undefined, courier: undefined, from: undefined, to: undefined,
  });
});

it("clears search through the existing debounce", () => {
  vi.useFakeTimers();
  const { onChange } = renderToolbar({ status: "pending", q: "test", type: "book" });
  fireEvent.click(screen.getByRole("button", { name: "Hapus pencarian" }));
  expect(screen.getByRole("searchbox")).toHaveValue("");
  expect(screen.queryByRole("button", { name: "Hapus pencarian" })).not.toBeInTheDocument();
  act(() => vi.advanceTimersByTime(400));
  expect(onChange).toHaveBeenCalledWith({ status: "pending", q: undefined, type: "book" });
  vi.useRealTimers();
});

it("uses the latest query and callback without restarting search debounce", () => {
  vi.useFakeTimers();
  const first = vi.fn();
  const latest = vi.fn();
  const { rerender } = render(<OrdersToolbar value={{ status: "all" }} onChange={first} />);
  fireEvent.change(screen.getByRole("searchbox"), { target: { value: "test" } });
  act(() => vi.advanceTimersByTime(200));
  rerender(<OrdersToolbar value={{ status: "pending", type: "book" }} onChange={latest} />);
  act(() => vi.advanceTimersByTime(200));
  expect(first).not.toHaveBeenCalled();
  expect(latest).toHaveBeenCalledWith({ status: "pending", type: "book", q: "test" });
  vi.useRealTimers();
});
