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

describe("OrdersPage list", () => {
  it("renders each order as a card row with product name, extra count and a link", async () => {
    const { useOrders } = await import("@/lib/hooks/orders");
    (useOrders as unknown as ReturnType<typeof vi.fn>).mockReturnValueOnce({
      data: [
        {
          id: "order-abcdefgh12345678",
          status: "paid",
          total: 150000,
          created_at: "2026-10-28T03:00:00Z",
          items: [
            { id: "i1", product_type: "course", name: "Kursus UTBK" },
            { id: "i2", product_type: "book", name: "Buku" },
            { id: "i3", product_type: "book", name: "Buku 2" },
          ],
        },
      ],
      isLoading: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    });
    render(<OrdersPage />);
    const row = screen.getByRole("link", { name: /Kursus UTBK/ });
    expect(row).toHaveAttribute("href", "/orders/order-abcdefgh12345678");
    expect(row).toHaveTextContent("+2");
    expect(row).toHaveTextContent("#12345678");
  });
});
