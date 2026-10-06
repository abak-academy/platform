import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { OrderStatusBadge } from "./OrderStatusBadge";

describe("OrderStatusBadge", () => {
  it.each([
    ["cart", "Keranjang"],
    ["payment_pending", "Menunggu Pembayaran"],
    ["paid", "Dibayar"],
    ["processing", "Diproses"],
    ["shipped", "Dikirim"],
    ["completed", "Selesai"],
    ["payment_expired", "Kadaluarsa"],
    ["cancelled", "Dibatalkan"],
  ] as const)("preserves the student label for %s", (status, label) => {
    render(<OrderStatusBadge status={status} />);
    expect(screen.getByText(label)).toBeInTheDocument();
  });

  it("allows an operator label while preserving the status styling", () => {
    render(<OrderStatusBadge status="shipped" label="Dalam pengiriman" />);
    expect(screen.getByText("Dalam pengiriman")).toHaveClass("text-brand-700", "bg-brand-50");
    expect(screen.queryByText("Dikirim")).not.toBeInTheDocument();
  });
});
