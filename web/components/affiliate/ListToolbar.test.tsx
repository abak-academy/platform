import { render, screen, fireEvent, act, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeAll, beforeEach, afterEach } from "vitest";
import { ListToolbar } from "./ListToolbar";
import type { ListParams } from "@/lib/hooks/use-list-params";

const base: ListParams = { q: "", status: "", type: "", from: "", to: "", page: 1, per_page: 20 };
const filters = [
  {
    key: "status" as const,
    label: "Status",
    options: [
      { value: "pending", label: "Menunggu" },
      { value: "available", label: "Tersedia" },
    ],
  },
];

function setup(params: Partial<ListParams> = {}, props: Partial<React.ComponentProps<typeof ListToolbar>> = {}) {
  const onChange = vi.fn();
  const onReset = vi.fn();
  const utils = render(
    <ListToolbar
      params={{ ...base, ...params }}
      onChange={onChange}
      onReset={onReset}
      searchPlaceholder="Cari siswa atau produk"
      filters={filters}
      dateLabel="Tanggal bayar"
      {...props}
    />,
  );
  return { onChange, onReset, ...utils };
}

beforeAll(() => {
  globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

describe("ListToolbar", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("reports a search once typing pauses, not on every keystroke", () => {
    const { onChange } = setup();
    const box = screen.getByRole("searchbox");

    fireEvent.change(box, { target: { value: "s" } });
    fireEvent.change(box, { target: { value: "sa" } });
    fireEvent.change(box, { target: { value: "sari" } });
    expect(onChange).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith({ q: "sari" });
  });

  it("clears the search with the x button", () => {
    const { onChange } = setup({ q: "sari" });
    expect(screen.getByRole("searchbox")).toHaveValue("sari");

    fireEvent.click(screen.getByRole("button", { name: "Hapus pencarian" }));
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(onChange).toHaveBeenCalledWith({ q: "" });
  });

  it("follows a search that changed from outside, without echoing it back", () => {
    const { onChange, rerender } = setup({ q: "sari" });
    rerender(
      <ListToolbar params={{ ...base, q: "" }} onChange={onChange} onReset={vi.fn()} searchPlaceholder="Cari siswa atau produk" filters={filters} dateLabel="Tanggal bayar" />,
    );
    act(() => {
      vi.advanceTimersByTime(400);
    });
    expect(screen.getByRole("searchbox")).toHaveValue("");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("shows an active filter and date range as removable chips with a count", () => {
    const { onChange } = setup({ status: "pending", from: "2026-10-01", to: "2026-10-31" });

    const group = screen.getByRole("group", { name: "Filter aktif" });
    expect(group).toHaveTextContent("Status: Menunggu");
    expect(group).toHaveTextContent("Tanggal bayar:");
    expect(screen.getByRole("button", { name: "Filter (2)" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Hapus filter: Status: Menunggu" }));
    expect(onChange).toHaveBeenCalledWith({ status: "" });
    fireEvent.click(screen.getByRole("button", { name: /Hapus filter: Tanggal bayar/ }));
    expect(onChange).toHaveBeenCalledWith({ from: "", to: "" });
  });

  it("draws no filter button or chips when the list has neither filters nor dates", () => {
    setup({}, { filters: [], dateLabel: undefined });
    expect(screen.queryByRole("button", { name: /^Filter/ })).toBeNull();
    expect(screen.queryByRole("group", { name: "Filter aktif" })).toBeNull();
  });

  it("omits the search box when the list has nothing to search", () => {
    setup({}, { searchPlaceholder: undefined });
    expect(screen.queryByRole("searchbox")).toBeNull();
  });

  it("picks a filter through its dropdown, clears it with Semua, and resets from the footer", async () => {
    vi.useRealTimers();
    const { onChange, onReset } = setup({ status: "pending" });
    fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
    const status = screen.getByRole("combobox", { name: "Status" });
    expect(status).toHaveTextContent("Menunggu");
    expect(status.closest(".grid")).toHaveClass("grid-cols-1", "sm:grid-cols-2");

    await userEvent.click(status);
    await userEvent.click(await screen.findByRole("option", { name: "Tersedia" }));
    expect(onChange).toHaveBeenLastCalledWith({ status: "available" });

    await userEvent.click(screen.getByRole("combobox", { name: "Status" }));
    await userEvent.click(await screen.findByRole("option", { name: "Semua" }));
    expect(onChange).toHaveBeenLastCalledWith({ status: "" });

    fireEvent.click(screen.getByRole("button", { name: "Atur ulang" }));
    expect(onReset).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Tampilkan hasil" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Tampilkan hasil" })).toBeNull());
  });
});
