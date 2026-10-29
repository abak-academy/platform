import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { SchoolMultiPicker } from "./SchoolMultiPicker";

vi.mock("@/components/SchoolPicker", () => ({
  SchoolPicker: ({ onChange }: { onChange: (s: { id: string; name: string } | null) => void }) => (
    <div>
      <button type="button" onClick={() => onChange({ id: "s2", name: "SMAN 2" })}>pick s2</button>
      <button type="button" onClick={() => onChange({ id: "s1", name: "SMAN 1" })}>pick s1</button>
    </div>
  ),
}));

describe("SchoolMultiPicker", () => {
  it("adds a picked school to the list", () => {
    const onChange = vi.fn();
    render(<SchoolMultiPicker value={[{ id: "s1", name: "SMAN 1" }]} onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: "pick s2" }));

    expect(onChange).toHaveBeenCalledWith([
      { id: "s1", name: "SMAN 1" },
      { id: "s2", name: "SMAN 2" },
    ]);
  });

  it("ignores a school that is already in the list", () => {
    const onChange = vi.fn();
    render(<SchoolMultiPicker value={[{ id: "s1", name: "SMAN 1" }]} onChange={onChange} />);

    fireEvent.click(screen.getByRole("button", { name: "pick s1" }));

    expect(onChange).not.toHaveBeenCalled();
  });

  it("removes a school from the list", () => {
    const onChange = vi.fn();
    render(
      <SchoolMultiPicker
        value={[{ id: "s1", name: "SMAN 1" }, { id: "s2", name: "SMAN 2" }]}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Hapus sekolah: SMAN 1" }));

    expect(onChange).toHaveBeenCalledWith([{ id: "s2", name: "SMAN 2" }]);
  });

  it("shows no list when nothing is picked", () => {
    render(<SchoolMultiPicker value={[]} onChange={vi.fn()} />);

    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });
});
