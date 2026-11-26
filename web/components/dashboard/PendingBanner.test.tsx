import { beforeEach, describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { PendingBanner } from "./PendingBanner";
import { useUIStore } from "@/stores/ui";

describe("PendingBanner", () => {
  beforeEach(() => useUIStore.setState({ lang: "en" }));

  it("renders English strings and links to the order", () => {
    render(<PendingBanner id="o1" product="Tryout" amount={50000} />);
    expect(screen.getByText("Payment pending")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Pay now" })).toHaveAttribute("href", "/orders/o1");
  });

  it("renders Indonesian strings", () => {
    useUIStore.setState({ lang: "id" });
    render(<PendingBanner id="o1" amount={50000} />);
    expect(screen.getByText("Pembayaran tertunda")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Bayar Sekarang" })).toBeInTheDocument();
  });
});
