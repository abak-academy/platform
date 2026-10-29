import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { SegmentedControl } from "./SegmentedControl";

const options = [
  { value: "single", label: "Satu akun" },
  { value: "bulk", label: "Banyak akun" },
] as const;

describe("SegmentedControl", () => {
  it("fills the chosen option and outlines the others", () => {
    render(<SegmentedControl value="bulk" onChange={vi.fn()} options={[...options]} label="Mode" />);

    const single = screen.getByRole("radio", { name: "Satu akun" });
    const bulk = screen.getByRole("radio", { name: "Banyak akun" });
    expect(bulk).toHaveAttribute("data-variant", "default");
    expect(bulk).toBeChecked();
    expect(single).toHaveAttribute("data-variant", "outline");
    expect(single).not.toBeChecked();
  });

  it("reports the option that was clicked", () => {
    const onChange = vi.fn();
    render(<SegmentedControl value="single" onChange={onChange} options={[...options]} label="Mode" />);

    fireEvent.click(screen.getByRole("radio", { name: "Banyak akun" }));

    expect(onChange).toHaveBeenCalledWith("bulk");
  });

  it("is a labelled radio group", () => {
    render(<SegmentedControl value="single" onChange={vi.fn()} options={[...options]} label="Cara membuat akun" />);

    expect(screen.getByRole("radiogroup", { name: "Cara membuat akun" })).toBeInTheDocument();
  });
});
