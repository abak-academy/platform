import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TablePager } from "./TablePager";

vi.mock("@/components/ui/select", async () => await import("@/__tests__/helpers/native-select"));

function setup(props: Partial<React.ComponentProps<typeof TablePager>> = {}) {
  const handlers = { onLimitChange: vi.fn(), onPrev: vi.fn(), onNext: vi.fn() };
  render(<TablePager total={57} offset={20} count={20} limit={20} canPrev canNext {...handlers} {...props} />);
  return handlers;
}

describe("TablePager", () => {
  it("says which rows are shown out of the total", () => {
    setup();
    expect(screen.getByTestId("pager-range")).toHaveTextContent("21–40 dari 57");
  });

  it("shows 0–0 for an empty page", () => {
    setup({ total: 0, offset: 0, count: 0, canPrev: false, canNext: false });
    expect(screen.getByTestId("pager-range")).toHaveTextContent("0–0 dari 0");
  });

  it("disables the buttons that cannot move and calls the ones that can", () => {
    const h = setup({ canPrev: false });
    expect(screen.getByRole("button", { name: "Sebelumnya" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Berikutnya" }));
    expect(h.onNext).toHaveBeenCalled();
  });

  it("offers 20, 50 and 100 rows per page and reports the choice", () => {
    const h = setup();
    const select = screen.getByRole("combobox", { name: "Baris per halaman" });
    expect([...select.querySelectorAll("option")].map((o) => o.textContent).filter(Boolean)).toEqual(["20", "50", "100"]);

    fireEvent.change(select, { target: { value: "50" } });
    expect(h.onLimitChange).toHaveBeenCalledWith(50);
  });
});
