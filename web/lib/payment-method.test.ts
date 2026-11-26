import { describe, expect, it } from "vitest";
import { paymentMethodLabel } from "./payment-method";

describe("paymentMethodLabel", () => {
  it("names Midtrans payment types in the active language", () => {
    expect(paymentMethodLabel("gopay", "id")).toBe("GoPay");
    expect(paymentMethodLabel("bank_transfer", "id")).toBe("Transfer bank (Virtual Account)");
    expect(paymentMethodLabel("bank_transfer", "en")).toBe("Bank transfer (Virtual Account)");
    expect(paymentMethodLabel("credit_card", "id")).toBe("Kartu kredit/debit");
    expect(paymentMethodLabel("credit_card", "en")).toBe("Credit/debit card");
  });

  it("shows an unknown code as is and an empty one as a dash", () => {
    expect(paymentMethodLabel("new_wallet", "id")).toBe("new_wallet");
    expect(paymentMethodLabel("", "id")).toBe("—");
    expect(paymentMethodLabel(undefined, "en")).toBe("—");
  });
});
