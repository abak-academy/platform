import { describe, it, expect, vi, beforeAll } from "vitest";
import { useState } from "react";
import { render, screen, fireEvent, within, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ListToolbar, type ListToolbarProps, type ListToolbarValue } from "./ListToolbar";

beforeAll(() => {
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
const pick = (group: string, name: string) =>
  fireEvent.click(within(screen.getByRole("group", { name: group })).getByRole("button", { name }));

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

  it("picking a chip updates the filter and keeps the panel open", () => {
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);
    openPanel();
    pick("Status", "Terbit");
    expect(onChange).toHaveBeenLastCalledWith({ q: "", filters: { status: "live" }, sort: undefined });
    expect(screen.getByRole("button", { name: "Tampilkan 12 hasil" })).toBeInTheDocument();
    expect(within(screen.getByRole("group", { name: "Status" })).getByRole("button", { name: "Terbit" })).toHaveAttribute("aria-pressed", "true");
  });

  it("picking Semua removes that filter", () => {
    render(<Harness initial={{ q: "", filters: { status: "live" } }} />);
    openPanel();
    pick("Status", "Semua");
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

  it("renders panel options as md-fchip and active filters as md-lchip", () => {
    const { container } = render(<Harness initial={{ q: "", filters: { status: "live" } }} />);
    expect(container.querySelector(".md-lchip")).not.toBeNull();
    openPanel();
    const on = within(screen.getByRole("group", { name: "Status" })).getByRole("button", { name: "Terbit" });
    expect(on).toHaveClass("md-fchip", "on");
    expect(within(screen.getByRole("group", { name: "Status" })).getByRole("button", { name: "Draf" })).not.toHaveClass("on");
  });

  it("renders the sort as a pill trigger", () => {
    render(<Harness sorts={[{ value: "a", label: "A" }, { value: "b", label: "B" }]} />);
    expect(screen.getByRole("combobox", { name: "Urutkan" })).toHaveClass("rounded-full");
  });
});
