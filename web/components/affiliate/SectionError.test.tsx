import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { SectionError } from "./SectionError";

describe("SectionError", () => {
  it("shows the load error and lets the user retry", () => {
    const reset = vi.fn();
    vi.spyOn(console, "error").mockImplementation(() => {});
    render(<SectionError error={new Error("boom")} reset={reset} />);

    expect(screen.getByRole("alert")).toHaveTextContent("Gagal memuat data");
    fireEvent.click(screen.getByRole("button"));
    expect(reset).toHaveBeenCalled();
  });
});
