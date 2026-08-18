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

  it("makes every row keyboard-accessible when onRowClick is set", () => {
    const onRowClick = vi.fn();
    render(
      <DataTable columns={columns} rows={rows} rowKey={(r) => r.id} empty="No rows" onRowClick={onRowClick} />
    );
    // role="button" on the <tr> overrides its implicit "row" role, so the
    // data rows surface under getAllByRole("button") instead.
    const dataRows = screen.getAllByRole("button");
    expect(dataRows).toHaveLength(2);
    for (const row of dataRows) {
      expect(row).toHaveAttribute("role", "button");
      expect(row).toHaveAttribute("tabIndex", "0");
    }

    dataRows[0].click();
    expect(onRowClick).toHaveBeenCalledWith(rows[0]);

    onRowClick.mockClear();
    dataRows[1].dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, cancelable: true }));
    expect(onRowClick).toHaveBeenCalledWith(rows[1]);

    onRowClick.mockClear();
    dataRows[1].dispatchEvent(new KeyboardEvent("keydown", { key: " ", bubbles: true, cancelable: true }));
    expect(onRowClick).toHaveBeenCalledWith(rows[1]);
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
