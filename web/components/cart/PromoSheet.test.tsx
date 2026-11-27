import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { PromoSheet } from "./PromoSheet";

const promo = (code: string) => ({
  code,
  discount_percent: 10,
  discount_amount: null,
  min_order_amount: null,
  max_discount_amount: null,
  expires_at: null,
});

describe("PromoSheet", () => {
  it("lets an applied promo be swapped directly through Ganti", async () => {
    const onApply = vi.fn();
    render(
      <PromoSheet
        promos={[promo("HEMAT10"), promo("BARU10")]}
        subtotal={100000}
        applied
        appliedCode="HEMAT10"
        discount={10000}
        onApply={onApply}
        onClear={() => {}}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Ganti" }));
    expect(await screen.findByText("Terpakai")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Pakai" }));
    expect(onApply).toHaveBeenCalledWith("BARU10");
  });
});
