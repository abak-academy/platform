import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ProductSpecsEditor } from "./ProductSpecsEditor";
import { specRowsForType, SPEC_FIELDS } from "@/lib/product-specs";

describe("specRowsForType", () => {
  it("offers the canonical book fields when nothing is saved yet", () => {
    const rows = specRowsForType("book", []);
    expect(rows.map((r) => r.key)).toEqual(SPEC_FIELDS.book.map((f) => f.key));
    expect(rows.every((r) => r.value === "")).toBe(true);
  });

  it("keeps saved values and appends custom rows after the canonical ones", () => {
    const rows = specRowsForType("book", [
      { key: "penerbit", label: "Perusahaan Penerbit", value: "Yayasan Abak Cendekia" },
      { key: "berat_buku", label: "Berat Buku", value: "300 g" },
    ]);

    const penerbit = rows.find((r) => r.key === "penerbit");
    expect(penerbit?.value).toBe("Yayasan Abak Cendekia");
    expect(rows[rows.length - 1].key).toBe("berat_buku");
  });

  it("gives exam and course no canonical fields", () => {
    expect(specRowsForType("exam", [])).toEqual([]);
    expect(specRowsForType("course", [])).toEqual([]);
  });
});

describe("ProductSpecsEditor", () => {
  it("emits every row including blanks so the parent can drop them at save time", () => {
    const onChange = vi.fn();
    render(<ProductSpecsEditor type="book" value={[]} onChange={onChange} />);

    const inputs = screen.getAllByPlaceholderText("Nilai");
    fireEvent.change(inputs[0], { target: { value: "Yayasan Abak Cendekia" } });

    expect(onChange).toHaveBeenCalled();
    const emitted = onChange.mock.calls[onChange.mock.calls.length - 1][0];
    expect(emitted[0]).toMatchObject({ key: "penerbit", value: "Yayasan Abak Cendekia" });
  });

  it("adds a custom row on demand", () => {
    const onChange = vi.fn();
    render(<ProductSpecsEditor type="medal" value={[]} onChange={onChange} />);

    const before = screen.getAllByPlaceholderText("Nilai").length;
    fireEvent.click(screen.getByRole("button", { name: /tambah baris/i }));
    expect(screen.getAllByPlaceholderText("Nilai").length).toBe(before + 1);
  });
});
