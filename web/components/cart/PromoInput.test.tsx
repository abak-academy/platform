import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PromoInput } from "./PromoInput";

describe("PromoInput", () => {
  it("calls onValidate with the trimmed code when Pakai is pressed", async () => {
    const user = userEvent.setup();
    const onValidate = vi.fn();

    render(<PromoInput onValidate={onValidate} onClear={() => {}} />);

    await user.type(screen.getByPlaceholderText("Masukkan kode promo"), "  HEMAT10  ");
    await user.click(screen.getByRole("button", { name: "Pakai" }));

    expect(onValidate).toHaveBeenCalledWith("HEMAT10");
  });

  it("does not call onValidate when the input is empty", async () => {
    const user = userEvent.setup();
    const onValidate = vi.fn();

    render(<PromoInput onValidate={onValidate} onClear={() => {}} />);

    expect(screen.getByRole("button", { name: "Pakai" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Pakai" }));

    expect(onValidate).not.toHaveBeenCalled();
  });

  // applied and discount both come from the order (cart.discount /
  // cart.promo_code_id), never from the validate response — the caller is
  // responsible for that wiring, this component just renders what it's given.
  it("shows the applied badge and discount only when applied is true", () => {
    const { rerender } = render(
      <PromoInput onValidate={() => {}} onClear={() => {}} applied={false} discount={0} />
    );
    expect(screen.queryByText(/Promo diterapkan/)).not.toBeInTheDocument();

    rerender(<PromoInput onValidate={() => {}} onClear={() => {}} applied discount={15000} />);
    expect(screen.getByText(/Promo diterapkan/)).toBeInTheDocument();
    expect(screen.getByText(/15.000|15,000/)).toBeInTheDocument();
  });

  it("calls onClear when the clear action is pressed", async () => {
    const user = userEvent.setup();
    const onClear = vi.fn();

    render(<PromoInput onValidate={() => {}} onClear={onClear} applied discount={15000} />);
    await user.click(screen.getByRole("button", { name: "Hapus" }));

    expect(onClear).toHaveBeenCalledTimes(1);
  });

  // A rejected promo must never render an optimistic discount — the error and
  // the "no discount applied" state are the only things shown.
  it("shows the error and no applied badge when the promo is rejected", () => {
    render(
      <PromoInput
        onValidate={() => {}}
        onClear={() => {}}
        applied={false}
        discount={0}
        error="Kode promo tidak valid"
      />
    );

    expect(screen.getByText("Kode promo tidak valid")).toBeInTheDocument();
    expect(screen.queryByText(/Promo diterapkan/)).not.toBeInTheDocument();
  });
});
