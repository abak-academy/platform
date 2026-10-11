import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
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

async function renderPage(retryOutcome: () => Promise<unknown>) {
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
}

async function renderPageAndRetry(retryOutcome: () => Promise<unknown>) {
  await renderPage(retryOutcome);
  await userEvent.click(await screen.findByRole("button", { name: /lanjutkan pembayaran/i }));
}

beforeEach(() => {
  vi.clearAllMocks();
  window.snap = undefined;
  authFetch.mockReset();
  useUIStore.setState({ lang: "id" });
  client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  open = vi.fn();
  vi.stubGlobal("open", open);
});

describe("OrderDetailPage retry payment", () => {
  it("offers a payment link without the new-tab toast when the popup is blocked", async () => {
    open.mockReturnValue(null);
    await renderPageAndRetry(() => Promise.resolve({ snap_token: "", payment_url: "https://pay.example/first" }));

    const link = await screen.findByRole("link", { name: /buka halaman pembayaran/i });
    expect(link).toHaveAttribute("href", "https://pay.example/first");
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByText("Halaman pembayaran belum terbuka?")).toBeVisible();
    expect(open).toHaveBeenCalledWith("https://pay.example/first", "_blank");
    expect(toastInfo).not.toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
  });

  it("offers a payment link and the new-tab toast when the popup opens", async () => {
    open.mockReturnValue({});
    await renderPageAndRetry(() => Promise.resolve({ snap_token: "", payment_url: "https://pay.example/first" }));

    const link = await screen.findByRole("link", { name: /buka halaman pembayaran/i });
    expect(link).toHaveAttribute("href", "https://pay.example/first");
    expect(toastInfo).toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
  });

  it("shows no payment link before retry and replaces it after another successful retry", async () => {
    const retryOutcome = vi.fn()
      .mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/first" })
      .mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/second" });
    await renderPage(retryOutcome);
    const button = await screen.findByRole("button", { name: /lanjutkan pembayaran/i });
    expect(screen.queryByRole("link", { name: /buka halaman pembayaran/i })).toBeNull();

    await userEvent.click(button);
    expect(await screen.findByRole("link", { name: /buka halaman pembayaran/i })).toHaveAttribute("href", "https://pay.example/first");

    await waitFor(() => expect(button).toBeEnabled());
    await userEvent.click(button);
    await waitFor(() => expect(screen.getByRole("link", { name: /buka halaman pembayaran/i })).toHaveAttribute("href", "https://pay.example/second"));
  });

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

  it.each([
    ["payment_deadline_too_close", "terlalu dekat"],
    ["order_not_editable", "tidak menunggu pembayaran"],
  ])("explains %s in the student's language instead of the raw backend message", async (code, phrase) => {
    await renderPageAndRetry(() => Promise.reject(apiError(code)));

    await waitFor(() => expect(toastError).toHaveBeenCalled());
    const [, options] = toastError.mock.calls[0];
    expect(options.description).toContain(phrase);
    expect(options.description).not.toContain(RAW);
    expect(open).not.toHaveBeenCalled();
  });

  it("opens the payment window only when the retry succeeds", async () => {
    await renderPageAndRetry(() => Promise.resolve({ snap_token: "", payment_url: "https://pay.example/x" }));

    await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
    expect(toastError).not.toHaveBeenCalled();
  });
});

describe("OrderDetailPage payment countdown", () => {
  const deadlineIn = (ms: number) => new Date(Date.now() + ms).toISOString();
  const pendingWith = (expiresAt: string | null) => ({ ...PENDING_ORDER, payment_expires_at: expiresAt });

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-11T10:00:00Z"));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function renderOrder(order: object, refetched: () => object = () => order, retried: object = {}) {
    let calls = 0;
    authFetch.mockImplementation((path: string) =>
      path.endsWith("/retry")
        ? Promise.resolve(retried)
        : Promise.resolve((calls += 1) === 1 ? order : refetched()),
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
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
  }

  async function advance(ms: number) {
    await act(async () => {
      vi.advanceTimersByTime(ms);
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
  }

  it("shows the remaining time and ticks down each second", async () => {
    await renderOrder(pendingWith(deadlineIn(19 * 60_000 + 42_000)));

    expect(screen.getByText("Sisa waktu sebelum pesanan kedaluwarsa")).toBeInTheDocument();
    expect(screen.getByText("19:42")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1000);
    });

    expect(screen.getByText("19:41")).toBeInTheDocument();
  });

  it("does not poll the order before the deadline", async () => {
    await renderOrder(pendingWith(deadlineIn(60_000)));
    const calls = authFetch.mock.calls.length;

    await advance(30_000);

    expect(authFetch.mock.calls.length).toBe(calls);
    expect(screen.getByRole("button", { name: /lanjutkan pembayaran/i })).toBeInTheDocument();
  });

  it("hides the payment link from an earlier retry once the time is up", async () => {
    await renderOrder(pendingWith(deadlineIn(60_000)), undefined, { snap_token: "t", payment_url: "https://pay.example/first" });
    fireEvent.click(screen.getByRole("button", { name: /lanjutkan pembayaran/i }));
    await advance(0);
    expect(screen.getByRole("link", { name: /buka halaman pembayaran/i })).toBeInTheDocument();

    await advance(60_000);

    expect(screen.queryByRole("link", { name: /buka halaman pembayaran/i })).toBeNull();
  });

  it("hides the button at the deadline, says time is up, and keeps refetching while pending", async () => {
    await renderOrder(pendingWith(deadlineIn(0)));

    expect(screen.queryByRole("button", { name: /lanjutkan pembayaran/i })).toBeNull();
    expect(screen.getByText("Waktu pembayaran habis. Menunggu pesanan ditutup.")).toBeInTheDocument();

    const first = authFetch.mock.calls.length;
    await advance(5000);
    expect(authFetch.mock.calls.length).toBeGreaterThan(first);

    const second = authFetch.mock.calls.length;
    await advance(5000);
    expect(authFetch.mock.calls.length).toBeGreaterThan(second);
  });

  it("stops refetching and hides the card once the order expires", async () => {
    const pending = pendingWith(deadlineIn(0));
    await renderOrder(pending, () => ({ ...pending, status: "payment_expired" }));

    expect(screen.getByText("Waktu pembayaran habis. Menunggu pesanan ditutup.")).toBeInTheDocument();

    await advance(5000);

    expect(screen.queryByText("Pembayaran tertunda")).toBeNull();
    expect(screen.queryByText("Waktu pembayaran habis. Menunggu pesanan ditutup.")).toBeNull();

    const calls = authFetch.mock.calls.length;
    await advance(15_000);
    expect(authFetch.mock.calls.length).toBe(calls);
  });

  it("keeps the button and shows no countdown when the deadline is null", async () => {
    await renderOrder(pendingWith(null));

    expect(screen.getByRole("button", { name: /lanjutkan pembayaran/i })).toBeInTheDocument();
    expect(screen.queryByText("Sisa waktu sebelum pesanan kedaluwarsa")).toBeNull();
  });

  it.each(["paid", "payment_expired", "cancelled", "completed"])(
    "shows no pending card or countdown for status %s",
    async (status) => {
      await renderOrder({ ...PENDING_ORDER, status, payment_expires_at: deadlineIn(60_000) });

      expect(screen.queryByText("Pembayaran tertunda")).toBeNull();
      expect(screen.queryByText("Sisa waktu sebelum pesanan kedaluwarsa")).toBeNull();
      expect(screen.queryByRole("button", { name: /lanjutkan pembayaran/i })).toBeNull();
    },
  );
});

it.each(["onSuccess", "onPending", "onClose", "onError"] as const)("retry handles %s without deciding payment status", async (event) => {
  const snapPay = vi.fn();
  window.snap = { pay: snapPay, embed: vi.fn() };
  const outcome = vi.fn().mockResolvedValue({ snap_token: "same-token", payment_url: "https://pay.example/x" });
  await renderPageAndRetry(outcome);
  const button = screen.getByRole("button", { name: /lanjutkan pembayaran/i });
  await waitFor(() => expect(button).toBeDisabled());
  expect(snapPay).toHaveBeenCalledWith("same-token", expect.any(Object));
  await userEvent.click(button);
  expect(outcome).toHaveBeenCalledTimes(1);
  authFetch.mockClear();
  act(() => snapPay.mock.calls[0][1][event]({ transaction_status: "settlement" }));
  await waitFor(() => expect(button).toBeEnabled());
  if (event !== "onError") expect(authFetch).toHaveBeenCalledWith("/orders/order-1");
  if (event === "onError") expect(toastError).toHaveBeenCalledWith("Pembayaran gagal. Coba lagi atau pilih metode pembayaran lain.");
  if (event === "onSuccess" || event === "onPending") expect(toastInfo).toHaveBeenCalledWith(expect.stringContaining("menunggu konfirmasi"));
  expect(client.getQueryData(ordersKeys.detail(ORDER_ID))).toEqual(PENDING_ORDER);
  expect(open).not.toHaveBeenCalled();
  await userEvent.click(button);
  await waitFor(() => expect(button).toBeDisabled());
  expect(snapPay).toHaveBeenCalledTimes(2);
});


function addSnapScript() {
  const script = document.createElement("script");
  script.src = "https://app.sandbox.midtrans.com/snap/snap.js";
  script.setAttribute("data-test-snap", "");
  document.head.appendChild(script);
}

afterEach(() => {
  document.head.querySelectorAll("script[data-test-snap]").forEach((el) => el.remove());
  vi.useRealTimers();
  window.snap = undefined;
});

it("retry waits three seconds then preserves the link when the fallback tab is blocked", async () => {
  addSnapScript();
  await renderPage(() => Promise.resolve({ snap_token: "token", payment_url: "https://pay.example/x" }));
  const button = await screen.findByRole("button", { name: /lanjutkan pembayaran/i });
  vi.useFakeTimers();
  open.mockReturnValue(null);
  await act(async () => { fireEvent.click(button); await vi.advanceTimersByTimeAsync(0); });
  expect(button).toBeDisabled();
  await act(async () => { await vi.advanceTimersByTimeAsync(2900); });
  expect(open).not.toHaveBeenCalled();
  await act(async () => { await vi.advanceTimersByTimeAsync(100); });
  expect(button).toBeEnabled();
  expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
  expect(screen.getByRole("link", { name: /buka halaman pembayaran/i })).toHaveAttribute("href", "https://pay.example/x");
  expect(toastInfo).not.toHaveBeenCalled();
});

it("retry falls back when Snap throws", async () => {
  window.snap = { pay: vi.fn(() => { throw new Error("unavailable"); }), embed: vi.fn() };
  open.mockReturnValue({});
  await renderPageAndRetry(() => Promise.resolve({ snap_token: "token", payment_url: "https://pay.example/x" }));
  await waitFor(() => expect(screen.getByRole("button", { name: /lanjutkan pembayaran/i })).toBeEnabled());
  expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
  expect(toastInfo).toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
});

it("guards synchronous double clicks while retry is pending", async () => {
  const outcome = vi.fn(() => new Promise(() => {}));
  await renderPage(outcome);
  const button = await screen.findByRole("button", { name: /lanjutkan pembayaran/i });
  act(() => { fireEvent.click(button); fireEvent.click(button); });
  await waitFor(() => expect(button).toBeDisabled());
  expect(outcome).toHaveBeenCalledTimes(1);
  expect(open).not.toHaveBeenCalled();
});
