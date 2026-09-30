import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Suspense } from "react";
import OrderDetailPage from "./page";
import { ApiError } from "@/lib/api";
import { ordersKeys } from "@/lib/hooks/orders";
import { useUIStore } from "@/stores/ui";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

const { toastError, toastInfo } = vi.hoisted(() => ({ toastError: vi.fn(), toastInfo: vi.fn() }));
vi.mock("sonner", () => ({
  toast: { error: toastError, info: toastInfo, success: vi.fn() },
}));

const authFetch = vi.fn();
vi.mock("@/lib/api", async (orig) => {
  const actual = await orig<typeof import("@/lib/api")>();
  return { ...actual, authFetch: (...a: unknown[]) => authFetch(...a) };
});

const ORDER_ID = "order-1";
const RAW = "digital product already owned or being purchased";

const PENDING_ORDER = {
  id: ORDER_ID,
  student_id: "student-1",
  status: "payment_pending",
  subtotal: 50000,
  discount: 0,
  shipping_cost: 0,
  total: 50000,
  items: [{ id: "item-1", product_id: "p1", product_type: "course", name: "Kursus A", unit_price: 50000, qty: 1 }],
};

function apiError(code: string, details?: unknown) {
  return new ApiError(code, RAW, 409, { code, message: RAW, details });
}

let client: QueryClient;
let open: ReturnType<typeof vi.fn>;

async function renderPageAndRetry(retryOutcome: () => Promise<unknown>) {
  authFetch.mockImplementation((path: string) =>
    path.endsWith("/retry") ? retryOutcome() : Promise.resolve(PENDING_ORDER),
  );
  await act(async () => {
    render(
      <QueryClientProvider client={client}>
        <Suspense fallback={null}>
          <OrderDetailPage params={Promise.resolve({ id: ORDER_ID })} />
        </Suspense>
      </QueryClientProvider>,
    );
  });
  await userEvent.click(await screen.findByRole("button", { name: /lanjutkan pembayaran/i }));
}

beforeEach(() => {
  vi.clearAllMocks();
  authFetch.mockReset();
  useUIStore.setState({ lang: "id" });
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  open = vi.fn();
  vi.stubGlobal("open", open);
});

describe("OrderDetailPage retry payment", () => {
  it("explains a digital conflict in the student's language, refetches the order and opens no payment window", async () => {
    const invalidate = vi.spyOn(client, "invalidateQueries");

    await renderPageAndRetry(() =>
      Promise.reject(
        apiError("digital_target_conflict", {
          conflicts: [{ product_id: "p1", kind: "course", target_id: "t1", reason: "owned" }],
        }),
      ),
    );

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    const [, options] = toastError.mock.calls[0];
    expect(options.description).toContain("Kamu sudah memiliki kursus ini.");
    expect(options.description).not.toContain(RAW);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ordersKeys.detail(ORDER_ID) });
    expect(open).not.toHaveBeenCalled();
  });

  it("explains an unavailable digital product without the raw backend message", async () => {
    await renderPageAndRetry(() => Promise.reject(apiError("digital_product_unavailable")));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    const [, options] = toastError.mock.calls[0];
    expect(options.description).toContain("belum tersedia untuk dibeli");
    expect(options.description).not.toContain(RAW);
  });

  it("refetches an order refused as expired so the page stops offering payment", async () => {
    const invalidate = vi.spyOn(client, "invalidateQueries");

    await renderPageAndRetry(() => Promise.reject(apiError("payment_expired")));

    await waitFor(() => expect(invalidate).toHaveBeenCalledWith({ queryKey: ordersKeys.detail(ORDER_ID) }));
    const [, options] = toastError.mock.calls[0];
    expect(options.description).not.toContain(RAW);
    expect(open).not.toHaveBeenCalled();
  });

  it("opens the payment window only when the retry succeeds", async () => {
    await renderPageAndRetry(() => Promise.resolve({ payment_url: "https://pay.example/x" }));

    await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
    expect(toastError).not.toHaveBeenCalled();
  });
});
