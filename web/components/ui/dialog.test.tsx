import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Dialog, DialogContent, DialogTitle } from "./dialog";

function open(size?: "default" | "wide") {
  render(
    <Dialog open>
      <DialogContent size={size}>
        <DialogTitle>Title</DialogTitle>
      </DialogContent>
    </Dialog>,
  );
  return screen.getByRole("dialog");
}

describe("DialogContent", () => {
  it("defaults to the 480px width", () => {
    const c = open().className;
    expect(c).toContain("max-w-[var(--container-dialog)]");
    expect(c).not.toContain("max-w-[var(--container-dialog-wide)]");
  });

  it("size=wide uses the 820px width", () => {
    expect(open("wide").className).toContain("max-w-[var(--container-dialog-wide)]");
  });

  it("uses modal radius, pop shadow and the dialog z-index", () => {
    const c = open().className;
    expect(c).toContain("rounded-[var(--radius-modal)]");
    expect(c).toContain("shadow-pop");
    expect(c).toContain("z-[var(--z-dialog)]");
  });

  it("close button is labelled Tutup", () => {
    open();
    expect(screen.getByRole("button", { name: "Tutup" })).toBeInTheDocument();
  });
});
