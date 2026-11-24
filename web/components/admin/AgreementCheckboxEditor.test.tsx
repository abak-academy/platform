import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { AgreementCheckboxEditor } from "./AgreementCheckboxEditor";

describe("AgreementCheckboxEditor", () => {
  it("shows the empty-state hint when there are no checkboxes yet", () => {
    render(<AgreementCheckboxEditor value={[]} onChange={vi.fn()} />);
    expect(screen.getByText(/belum ada checkbox/i)).toBeInTheDocument();
  });

  it("adds a new blank checkbox row on demand", () => {
    const onChange = vi.fn();
    render(<AgreementCheckboxEditor value={[]} onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: /tambah checkbox/i }));

    expect(onChange).toHaveBeenCalledWith([{ id: "", title: "", body: "" }]);
  });

  it("edits title and body for an existing row without touching others", () => {
    const onChange = vi.fn();
    render(
      <AgreementCheckboxEditor
        value={[
          { id: "c1", title: "Syarat 1", body: "Isi 1" },
          { id: "c2", title: "Syarat 2", body: "Isi 2" },
        ]}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByDisplayValue("Syarat 1"), { target: { value: "Syarat 1 diedit" } });

    expect(onChange).toHaveBeenLastCalledWith([
      { id: "c1", title: "Syarat 1 diedit", body: "Isi 1" },
      { id: "c2", title: "Syarat 2", body: "Isi 2" },
    ]);
  });

  it("removes only the targeted row", () => {
    const onChange = vi.fn();
    render(
      <AgreementCheckboxEditor
        value={[
          { id: "c1", title: "Syarat 1", body: "Isi 1" },
          { id: "c2", title: "Syarat 2", body: "Isi 2" },
        ]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Hapus checkbox 1" }));

    expect(onChange).toHaveBeenCalledWith([{ id: "c2", title: "Syarat 2", body: "Isi 2" }]);
  });
});
