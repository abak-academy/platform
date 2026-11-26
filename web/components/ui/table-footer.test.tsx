import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TableFooter } from "./table-footer";

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({ t: (k: string) => k, lang: "id" }),
}));

describe("TableFooter", () => {
  it("shows the summary and enables only the available directions", async () => {
    const onNext = vi.fn();
    const onPrev = vi.fn();
    render(<TableFooter summary="1-10 of 25" hasPrev={false} hasNext onPrev={onPrev} onNext={onNext} />);
    expect(screen.getByText("1-10 of 25")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "pagination_prev" })).toBeDisabled();
    await userEvent.click(screen.getByRole("button", { name: "pagination_next" }));
    expect(onNext).toHaveBeenCalledTimes(1);
    expect(onPrev).not.toHaveBeenCalled();
  });

  it("renders without buttons when no handlers are given and passes extra controls through", () => {
    render(<TableFooter summary="25 rows"><span>size</span></TableFooter>);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText("size")).toBeInTheDocument();
  });
});
