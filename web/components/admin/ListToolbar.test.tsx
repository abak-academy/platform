import { describe, it, expect, vi, beforeAll } from "vitest";
import { useState } from "react";
import { render, screen, fireEvent, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FilterSelect, ListToolbar, type ListToolbarProps, type ListToolbarValue } from "./ListToolbar";

beforeAll(() => {
  globalThis.ResizeObserver ??= class { observe() {} unobserve() {} disconnect() {} };
  Element.prototype.hasPointerCapture = vi.fn(() => false);
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

const filters = [
  { key: "status", label: "Status", options: [{ value: "draft", label: "Draf" }, { value: "live", label: "Terbit" }] },
  { key: "kind", label: "Jenis", options: [{ value: "a", label: "Alpha" }, { value: "b", label: "Beta" }] },
];

function Harness({ initial, onChange, ...rest }: { initial?: ListToolbarValue; onChange?: (v: ListToolbarValue) => void } & Partial<ListToolbarProps>) {
  const [value, setValue] = useState<ListToolbarValue>(initial ?? { q: "", filters: {} });
  return (
    <ListToolbar
      value={value}
      onChange={(v) => { setValue(v); onChange?.(v); }}
      filters={filters}
      count={12}
      unit="pesanan"
      variant="standalone"
      {...rest}
    />
  );
}

const openPanel = () => fireEvent.click(screen.getByRole("button", { name: /^Filter/ }));
const pick = async (field: string, name: string) => {
  await userEvent.click(screen.getByRole("combobox", { name: field }));
  await userEvent.click(await screen.findByRole("option", { name }));
};

describe("ListToolbar", () => {
  it("shows the result count with its unit", () => {
    render(<Harness />);
    expect(screen.getByText("12 pesanan")).toBeInTheDocument();
  });

  it("shows the active-filter count on the Filter button and a chip per filter", () => {
    render(<Harness initial={{ q: "", filters: { status: "live", kind: "b" } }} />);
    expect(screen.getByRole("button", { name: "Filter (2)" })).toBeInTheDocument();
    const chips = screen.getByRole("group", { name: "Filter aktif" });
    expect(within(chips).getByText("Status: Terbit")).toBeInTheDocument();
    expect(within(chips).getByText("Jenis: Beta")).toBeInTheDocument();
  });

  it("shows no count or chips when nothing is filtered", () => {
    render(<Harness initial={{ q: "abc", filters: {}, sort: "x" }} />);
    expect(screen.getByRole("button", { name: "Filter" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
  });

  it("removes only the clicked chip's filter", () => {
    const onChange = vi.fn();
    render(<Harness initial={{ q: "abc", filters: { status: "live", kind: "b" } }} onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Hapus filter: Status: Terbit" }));
    expect(onChange).toHaveBeenLastCalledWith({ q: "abc", filters: { kind: "b" }, sort: undefined });
    expect(screen.queryByText("Status: Terbit")).not.toBeInTheDocument();
    expect(screen.getByText("Jenis: Beta")).toBeInTheDocument();
  });

  it("Hapus semua clears every filter but keeps search and sort", () => {
    const onChange = vi.fn();
    render(<Harness initial={{ q: "abc", filters: { status: "live", kind: "b" }, sort: "s" }} onChange={onChange} />);
    fireEvent.click(screen.getByRole("button", { name: "Hapus semua" }));
    expect(onChange).toHaveBeenLastCalledWith({ q: "abc", filters: {}, sort: "s" });
  });

  it("Reset filter in the panel clears filters", () => {
    const onChange = vi.fn();
    render(<Harness initial={{ q: "", filters: { status: "live" } }} onChange={onChange} />);
    openPanel();
    fireEvent.click(screen.getByRole("button", { name: "Reset filter" }));
    expect(onChange).toHaveBeenLastCalledWith({ q: "", filters: {}, sort: undefined });
  });

  it("picking a dropdown option updates the filter and keeps the panel open", async () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    openPanel();
    await pick("Status", "Terbit");
    expect(onChange).toHaveBeenLastCalledWith({ q: "", filters: { status: "live" }, sort: undefined });
    expect(screen.getByRole("button", { name: "Tampilkan 12 hasil" })).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: "Status" })).toHaveTextContent("Terbit");
    expect(within(screen.getByRole("group", { name: "Filter aktif" })).getByText("Status: Terbit")).toBeInTheDocument();
  });

  it("picking Semua removes that filter", async () => {
    render(<Harness initial={{ q: "", filters: { status: "live" } }} />);
    openPanel();
    await pick("Status", "Semua");
    expect(screen.queryByRole("group", { name: "Filter aktif" })).not.toBeInTheDocument();
  });

  it("Tampilkan N hasil closes the panel", async () => {
    render(<Harness />);
    openPanel();
    fireEvent.click(screen.getByRole("button", { name: "Tampilkan 12 hasil" }));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Tampilkan 12 hasil" })).not.toBeInTheDocument());
  });

  it("Escape closes the panel", async () => {
    render(<Harness />);
    openPanel();
    fireEvent.keyDown(screen.getByRole("button", { name: "Tampilkan 12 hasil" }), { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("button", { name: "Tampilkan 12 hasil" })).not.toBeInTheDocument());
  });

  it("outside click closes the panel", async () => {
    render(<Harness />);
    openPanel();
    await userEvent.click(document.body);
    await waitFor(() => expect(screen.queryByRole("button", { name: "Tampilkan 12 hasil" })).not.toBeInTheDocument());
  });

  it("typing in search reports q and the clear button resets it", () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} searchPlaceholder="Cari event" />);
    fireEvent.change(screen.getByRole("searchbox", { name: "Cari event" }), { target: { value: "abc" } });
    expect(onChange).toHaveBeenLastCalledWith({ q: "abc", filters: {}, sort: undefined });
    fireEvent.click(screen.getByRole("button", { name: "Hapus pencarian" }));
    expect(onChange).toHaveBeenLastCalledWith({ q: "", filters: {}, sort: undefined });
  });

  it("hides the sort select unless there is more than one sort", () => {
    const { rerender } = render(<Harness sorts={[{ value: "a", label: "A" }]} />);
    expect(screen.queryByRole("combobox", { name: "Urutkan" })).not.toBeInTheDocument();
    rerender(<Harness sorts={[{ value: "a", label: "A" }, { value: "b", label: "B" }]} />);
    expect(screen.getByRole("combobox", { name: "Urutkan" })).toBeInTheDocument();
  });
});

describe("ListToolbar — prototype chrome", () => {
  it("renders the standalone variant as a card and card-header as a bottom-bordered strip", () => {
    const { container, rerender } = render(<Harness variant="standalone" />);
    expect(container.firstElementChild).toHaveClass("rounded-[var(--radius-card)]", "shadow-card");
    rerender(<Harness variant="card-header" />);
    expect(container.firstElementChild).toHaveClass("border-b");
    expect(container.firstElementChild).not.toHaveClass("shadow-card");
  });

  it("uses a 42px pill search, a 42px filter button with a count pill and a mono count", () => {
    render(<Harness initial={{ q: "", filters: { status: "live" } }} />);
    expect(screen.getByRole("searchbox")).toHaveClass("h-[var(--spacing-control-md)]", "rounded-full");
    const filter = screen.getByRole("button", { name: "Filter (1)" });
    expect(filter).toHaveClass("rounded-[var(--radius-pill)]");
    expect(within(filter).getByText("1")).toHaveClass("bg-brand-600", "rounded-full");
    expect(screen.getByText("12 pesanan")).toHaveClass("font-mono", "text-xs", "uppercase");
  });

  it("lays the panel filters out as a 1-col / 2-col grid of dropdowns, with a dark result button", () => {
    const { container } = render(<Harness initial={{ q: "", filters: { status: "live" } }} />);
    expect(container.querySelector(".md-lchip")).not.toBeNull();
    openPanel();
    const status = screen.getByRole("combobox", { name: "Status" });
    expect(status).toHaveTextContent("Terbit");
    expect(screen.getByRole("combobox", { name: "Jenis" })).toHaveTextContent("Semua");
    expect(status.closest(".grid")).toHaveClass("grid-cols-1", "sm:grid-cols-2", "gap-3");
    expect(document.querySelector(".md-fchip")).toBeNull();
    expect(screen.getByRole("button", { name: "Tampilkan 12 hasil" })).toHaveClass("bg-ink-900");
  });

  it("renders the sort as a pill trigger", () => {
    render(<Harness sorts={[{ value: "a", label: "A" }, { value: "b", label: "B" }]} />);
    expect(screen.getByRole("combobox", { name: "Urutkan" })).toHaveClass("rounded-full");
  });
});

describe("FilterSelect allowAll", () => {
  it("offers Semua by default and leaves it out for a mode switch", async () => {
    const opts = [{ value: "a", label: "Satu" }, { value: "b", label: "Dua" }];
    const { unmount } = render(<FilterSelect label="Jenis" value="a" options={opts} onChange={() => {}} />);
    await userEvent.click(screen.getByRole("combobox", { name: "Jenis" }));
    expect(await screen.findByRole("option", { name: "Semua" })).toBeInTheDocument();
    unmount();
    render(<FilterSelect label="Mode" value="a" options={opts} onChange={() => {}} allowAll={false} />);
    await userEvent.click(screen.getByRole("combobox", { name: "Mode" }));
    expect(await screen.findByRole("option", { name: "Dua" })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Semua" })).toBeNull();
  });
});

describe("FilterSelect disabled", () => {
  it("disables the trigger", () => {
    render(<FilterSelect label="Topik" value="" options={[]} onChange={() => {}} disabled />);
    expect(screen.getByRole("combobox", { name: "Topik" })).toBeDisabled();
  });
});
