import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
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
    const cell = screen.getByText("No rows");
    expect(cell.tagName).toBe("TD");
    expect(cell.getAttribute("colSpan")).toBe(String(columns.length));
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
    expect(card?.className).toContain("md-card-outlined");
    const scroller = card?.querySelector(".overflow-x-auto");
    expect(scroller).not.toBeNull();
    expect(scroller?.querySelector("table")).not.toBeNull();
  });
});
