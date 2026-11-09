import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RetryPaymentButton } from "./RetryPaymentButton";

const authFetch = vi.fn();
vi.mock("@/lib/api", async (orig) => {
  const actual = await orig<typeof import("@/lib/api")>();
  return { ...actual, authFetch: (...a: unknown[]) => authFetch(...a) };
});

const payWithSnap = vi.fn();
vi.mock("@/lib/snap", () => ({ payWithSnap: (...a: unknown[]) => payWithSnap(...a) }));

const toastError = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a), success: vi.fn(), info: vi.fn() },
}));

function renderButton() {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RetryPaymentButton orderId="order-1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("RetryPaymentButton", () => {
  it("restarts payment through the Mitra retry route and opens Snap", async () => {
    authFetch.mockResolvedValue({ snap_token: "tok", payment_url: "https://pay.test/x" });
    payWithSnap.mockResolvedValue("popup");
    renderButton();

    await userEvent.click(screen.getByRole("button", { name: "Bayar" }));

    await waitFor(() => expect(payWithSnap).toHaveBeenCalled());
    expect(authFetch).toHaveBeenCalledWith(
      "/partners/me/orders/order-1/retry",
      expect.objectContaining({ method: "POST" }),
    );
    expect(payWithSnap.mock.calls[0][0]).toMatchObject({ snapToken: "tok", paymentUrl: "https://pay.test/x" });
  });

  it("shows an error toast and frees the button when the retry is refused", async () => {
    authFetch.mockRejectedValue(new Error("expired"));
    renderButton();

    await userEvent.click(screen.getByRole("button", { name: "Bayar" }));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    expect(payWithSnap).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled();
  });
});
