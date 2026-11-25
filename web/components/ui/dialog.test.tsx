import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "./dialog";

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

  it("size=wide scrolls the body only; header and footer stay fixed", () => {
    render(
      <Dialog open>
        <DialogContent size="wide">
          <DialogHeader><DialogTitle>Title</DialogTitle></DialogHeader>
          <div data-testid="body">body</div>
          <DialogFooter>footer</DialogFooter>
        </DialogContent>
      </Dialog>,
    );
    const c = screen.getByRole("dialog").className;
    expect(c).toContain("flex flex-col overflow-hidden");
    expect(c).toContain("[&>:not([data-slot^=dialog-])]:overflow-y-auto");
    expect(c).toContain("[&>:not([data-slot^=dialog-])]:min-h-0");
    expect(c).toContain("[&>[data-slot=dialog-header]]:shrink-0");
    expect(c).toContain("[&>[data-slot=dialog-footer]]:shrink-0");
    const dialog = screen.getByRole("dialog");
    expect(screen.getByTestId("body").matches("[data-slot^=dialog-]")).toBe(false);
    expect(dialog.querySelector("[data-slot=dialog-header]")!.parentElement).toBe(dialog);
    expect(dialog.querySelector("[data-slot=dialog-footer]")!.parentElement).toBe(dialog);
  });

  it("default size scrolls the whole dialog", () => {
    expect(open().className).toContain("overflow-y-auto");
  });
});
