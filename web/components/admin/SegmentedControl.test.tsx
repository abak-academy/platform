import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { SegmentedControl } from "./SegmentedControl";

const options = [
  { value: "single", label: "Satu akun" },
  { value: "bulk", label: "Banyak akun" },
] as const;

describe("SegmentedControl", () => {
  it("fills the chosen option with the primary colour and leaves the others plain", () => {
    render(<SegmentedControl value="bulk" onChange={vi.fn()} options={[...options]} label="Mode" />);

    const single = screen.getByRole("radio", { name: "Satu akun" });
    const bulk = screen.getByRole("radio", { name: "Banyak akun" });
    expect(bulk).toBeChecked();
    expect(bulk.className).toContain("bg-primary");
    expect(single).not.toBeChecked();
    expect(single.className).not.toContain("bg-primary");
  });

  it("hovers differently on the chosen option than on the others", () => {
    render(<SegmentedControl value="bulk" onChange={vi.fn()} options={[...options]} label="Mode" />);

    const hover = (name: string) => screen.getByRole("radio", { name }).className.split(" ").filter((c) => c.startsWith("hover:"));
    expect(hover("Banyak akun")).toEqual(["hover:bg-brand-700"]);
    expect(hover("Satu akun")).toEqual(["hover:bg-brand-50", "hover:text-brand-700"]);
  });

  it("joins the options in one bordered bar", () => {
    render(<SegmentedControl value="single" onChange={vi.fn()} options={[...options]} label="Mode" />);

    expect(screen.getByRole("radiogroup").className).toContain("border");
    expect(screen.getByRole("radiogroup").className).toContain("overflow-hidden");
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
