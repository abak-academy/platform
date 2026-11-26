import { describe, it, expect, vi } from "vitest";
import { createPortal } from "react-dom";
import { fireEvent, render, screen } from "@testing-library/react";
import { DataTable, type DataTableColumn } from "./data-table";

interface Row {
  id: string;
  name: string;
}

const rows: Row[] = [
  { id: "1", name: "Alpha" },
  { id: "2", name: "Beta" },
];

const columns: DataTableColumn<Row>[] = [
  { key: "name", header: "Name", cell: (row) => row.name },
  { key: "id", header: "ID", cell: (row) => row.id, align: "right" },
];

describe("DataTable", () => {
  it("renders headers and cells", () => {
    render(<DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" />);
    expect(screen.getByText("Name")).toBeInTheDocument();
    expect(screen.getByText("ID")).toBeInTheDocument();
    expect(screen.getByText("Alpha")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });

  it("renders the empty state as one row spanning all columns", () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} empty="No rows" />);
    const cell = screen.getByText("No rows").closest("td")!;
    expect(cell.getAttribute("colSpan")).toBe(String(columns.length));
  });

  it("renders a non-string empty node as is", () => {
    render(<DataTable columns={columns} rows={[]} rowKey={(r) => r.id} empty={<b>Custom</b>} />);
    expect(screen.getByText("Custom")).toBeInTheDocument();
  });

  it("renders rows as plain table rows with no row-level interactivity", () => {
    render(<DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" />);
    const dataRows = screen.getAllByRole("row").slice(1);
    expect(dataRows).toHaveLength(2);
    // A focusable row with a non-interactive role tells a screen reader nothing
    // about the action, so call sites render an explicit control in a cell.
    for (const row of dataRows) {
      expect(row).not.toHaveAttribute("role");
      expect(row).not.toHaveAttribute("tabIndex");
    }
  });

  it("exposes an in-cell control as a real button", () => {
    const onSelect = vi.fn();
    const inCellColumns: DataTableColumn<Row>[] = [
      ...columns,
      { key: "actions", header: "", cell: (r) => <button onClick={() => onSelect(r)}>Open</button> },
    ];
    render(<DataTable columns={inCellColumns} rows={rows} rowKey={(r) => r.id} empty="No rows" />);
    const buttons = screen.getAllByRole("button", { name: "Open" });
    expect(buttons).toHaveLength(2);
    buttons[1].click();
    expect(onSelect).toHaveBeenCalledWith(rows[1]);
  });
  it("does not make rows focusable without onRowClick", () => {
    const { container } = render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" />
    );
    const dataRows = container.querySelectorAll("tbody tr");
    expect(dataRows).toHaveLength(2);
    for (const row of dataRows) {
      expect(row).not.toHaveAttribute("role", "button");
      expect(row).not.toHaveAttribute("tabIndex");
    }
  });

  it("wraps the table in an overflow-x-auto scroller inside a card", () => {
    const { container } = render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" data-testid="my-table" />
    );
    const card = container.querySelector('[data-testid="my-table"]');
    expect(card).not.toBeNull();
    expect(card?.className).toContain("rounded-[var(--radius-card)]");
    expect(card?.className).toContain("overflow-hidden");
    expect(card?.className).not.toMatch(/\bp-\d/);
    const scroller = card?.querySelector(".overflow-x-auto");
    expect(scroller).not.toBeNull();
    expect(scroller?.querySelector("table")).not.toBeNull();
  });

  it("can render as a plain table without the card shell", () => {
    render(
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => r.id}
        empty="No rows"
        surface="plain"
        data-testid="plain-table"
      />
    );

    expect(screen.getByTestId("plain-table")).not.toHaveClass("shadow-card");
  });

  it("styles headers as mono caps on the sunken surface and cells with a top divider", () => {
    const { container } = render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" />
    );
    const th = container.querySelector("th")!;
    for (const c of ["font-mono", "text-[10.5px]", "tracking-[.1em]", "uppercase", "bg-surface-sunken", "py-3", "px-4"]) {
      expect(th).toHaveClass(c);
    }
    const td = container.querySelector("td")!;
    for (const c of ["border-t", "border-line", "text-ink-700"]) expect(td).toHaveClass(c);
    expect(container.querySelector("table")).toHaveClass("text-[13.5px]");
  });

  it("makes rows clickable and keyboard-activatable with onRowClick", async () => {
    const onRowClick = vi.fn();
    const { container } = render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" onRowClick={onRowClick} />
    );
    const first = container.querySelector("tbody tr") as HTMLElement;
    expect(first).toHaveAttribute("tabindex", "0");
    expect(first).toHaveClass("cursor-pointer");
    fireEvent.click(screen.getByText("Alpha"));
    expect(onRowClick).toHaveBeenLastCalledWith(rows[0]);
    first.focus();
    fireEvent.keyDown(first, { key: "Enter" });
    expect(onRowClick).toHaveBeenCalledTimes(2);
  });

  it("does not fire onRowClick for clicks on controls inside a cell", () => {
    const onRowClick = vi.fn();
    const onOpen = vi.fn();
    render(
      <DataTable
        columns={columns}
        rows={rows}
        rowKey={(r) => r.id}
        empty="No rows"
        onRowClick={onRowClick}
        rowActions={(r) => <button onClick={() => onOpen(r)}>Edit</button>}
      />
    );
    fireEvent.click(screen.getAllByRole("button", { name: "Edit" })[0]);
    expect(onOpen).toHaveBeenCalledWith(rows[0]);
    expect(onRowClick).not.toHaveBeenCalled();
  });

  it("renders a trailing right-aligned action cell per row", () => {
    const { container } = render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" rowActions={() => <i>act</i>} />
    );
    expect(container.querySelectorAll("tbody tr:first-child td")).toHaveLength(columns.length + 1);
    expect(container.querySelector("tbody tr td:last-child")).toHaveClass("text-right");
  });

  it("wraps cells by default and keeps nowrap for opted-in, strong and right-aligned columns", () => {
    const cols: DataTableColumn<Row>[] = [
      { key: "name", header: "Name", cell: (r) => r.name },
      { key: "n", header: "N", cell: (r) => r.id, nowrap: true },
      { key: "s", header: "S", cell: () => "strong", strong: true },
      { key: "r", header: "R", cell: () => "right", align: "right" },
    ];
    render(<DataTable columns={cols} rows={rows} rowKey={(r) => r.id} empty="x" />);
    expect(screen.getAllByText("Alpha")[0].closest("td")).not.toHaveClass("whitespace-nowrap");
    expect(screen.getAllByText("1")[0].closest("td")).toHaveClass("whitespace-nowrap");
    expect(screen.getAllByText("strong")[0].closest("td")).toHaveClass("whitespace-nowrap");
    expect(screen.getAllByText("right")[0].closest("td")).toHaveClass("whitespace-nowrap");
  });

  it("labels the actions column and ignores clicks from outside the row", () => {
    const onRowClick = vi.fn();
    render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="x" onRowClick={onRowClick} rowActions={() => createPortal(<i>portal</i>, document.body)} />,
    );
    expect(screen.getByText("Aksi")).toHaveClass("sr-only");
    fireEvent.click(screen.getAllByText("portal")[0]);
    expect(onRowClick).not.toHaveBeenCalled();
    fireEvent.click(screen.getByText("Alpha"));
    expect(onRowClick).toHaveBeenCalledTimes(1);
  });
});
