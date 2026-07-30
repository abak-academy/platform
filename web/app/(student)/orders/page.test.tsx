import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import OrdersPage from "./page";

vi.mock("@/lib/hooks/orders", () => ({
  useOrders: vi.fn(() => ({
    data: [],
    isLoading: false,
    isError: false,
    error: null,
    refetch: vi.fn(),
  })),
}));

vi.mock("@/lib/i18n", () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const dict: Record<string, string> = {
        nav_billing: "Billing",
        billing_subtitle: "Your orders",
        billing_no_tx_title: "No transactions yet",
        billing_no_tx_desc: "You haven't made any purchases",
        billing_browse_store: "Browse store",
        billing_payment_methods: "Payment methods",
        billing_secure: "Secure",
      };
      return dict[key] || key;
    },
    lang: "id",
  }),
}));

describe("OrdersPage empty state", () => {
  it("renders only the Browse store link, no handler-less button", () => {
    render(<OrdersPage />);

    expect(screen.getByRole("link", { name: /Browse store/i })).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
