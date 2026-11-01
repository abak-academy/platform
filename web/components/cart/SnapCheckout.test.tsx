import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SnapCheckout } from "./SnapCheckout";
import { ApiError } from "@/lib/api";
import { ordersKeys } from "@/lib/hooks/orders";
import { useUIStore } from "@/stores/ui";

const push = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

const toastError = vi.fn();
const toastInfo = vi.fn();
vi.mock("sonner", () => ({
  toast: { error: (...a: unknown[]) => toastError(...a), success: vi.fn(), info: (...a: unknown[]) => toastInfo(...a) },
}));

const authFetch = vi.fn();
vi.mock("@/lib/api", async (orig) => {
  const actual = await orig<typeof import("@/lib/api")>();
  return { ...actual, authFetch: (...a: unknown[]) => authFetch(...a) };
});

const TARGET = "0b6f7c1e-3a52-4c1d-9d0e-6f1b1f2f7a11";

function conflictError(reason: string, kind = "course") {
  return new ApiError("digital_target_conflict", "digital product already owned or being purchased", 409, {
    code: "digital_target_conflict",
    message: "digital product already owned or being purchased",
    details: { conflicts: [{ product_id: "p1", kind, target_id: TARGET, reason }] },
  });
}

let client: QueryClient;
function renderCheckout(basePath?: string) {
  client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SnapCheckout orderId="order-1" basePath={basePath} />
    </QueryClientProvider>,
  );
}

let open: ReturnType<typeof vi.fn>;
let writeText: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.clearAllMocks();
  window.snap = undefined;
  authFetch.mockReset();
  useUIStore.setState({ lang: "id" });
  open = vi.fn();
  writeText = vi.fn();
  vi.stubGlobal("open", open);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
});

async function pay() {
  await userEvent.click(screen.getByRole("button", { name: /^(bayar|pay)$/i }));
}

describe("SnapCheckout conflicts", () => {
  it("routes to the order with a blocked-popup toast when the payment popup is blocked", async () => {
    open.mockReturnValue(null);
    authFetch.mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /^(bayar|pay)$/i })).toBeEnabled());
    expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
    expect(push).toHaveBeenCalledWith("/orders/order-1");
    expect(toastInfo).toHaveBeenCalledWith("Pop-up pembayaran diblokir browser. Lanjutkan pembayaran dari halaman pesanan.");
    expect(toastInfo).not.toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
  });

  it("keeps the new-tab toast without navigation when the payment popup opens", async () => {
    open.mockReturnValue({});
    authFetch.mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /^(bayar|pay)$/i })).toBeEnabled());
    expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
    expect(push).not.toHaveBeenCalled();
    expect(toastInfo).toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
    expect(toastInfo).not.toHaveBeenCalledWith("Pop-up pembayaran diblokir browser. Lanjutkan pembayaran dari halaman pesanan.");
  });

  it("does not route admin checkout to student orders when the popup is blocked", async () => {
    open.mockReturnValue(null);
    authFetch.mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/x" });
    renderCheckout("/admin/orders");

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /^(bayar|pay)$/i })).toBeEnabled());
    expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
    expect(push).not.toHaveBeenCalled();
  });

  it.each([
    ["owned", /sudah memiliki kursus ini/i],
    ["claimed", /sedang dibeli lewat pesanan lain/i],
    ["duplicate_in_cart", /hapus salah satunya/i],
  ])("shows a persistent %s reason next to the pay button and opens no payment", async (reason, text) => {
    authFetch.mockRejectedValueOnce(conflictError(reason));
    renderCheckout();

    await pay();

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(text);
    expect(alert.textContent).not.toContain(TARGET);
    expect(open).not.toHaveBeenCalled();
    expect(writeText).not.toHaveBeenCalled();
    expect(push).not.toHaveBeenCalled();
    expect(toastError).toHaveBeenCalledTimes(1);
  });

  it("explains an unlinked digital product", async () => {
    authFetch.mockRejectedValueOnce(
      new ApiError("digital_product_unavailable", "digital product has no targets", 409),
    );
    renderCheckout();

    await pay();

    expect(await screen.findByRole("alert")).toHaveTextContent(/belum tersedia.*hubungi admin/i);
    expect(open).not.toHaveBeenCalled();
    expect(writeText).not.toHaveBeenCalled();
  });

  it("clears the reason when the next attempt starts", async () => {
    authFetch.mockRejectedValueOnce(conflictError("owned"));
    let resolve!: (v: unknown) => void;
    authFetch.mockReturnValueOnce(new Promise((r) => (resolve = r)));
    renderCheckout();

    await pay();
    await screen.findByRole("alert");

    await pay();
    await waitFor(() => expect(screen.queryByRole("alert")).toBeNull());

    resolve({ snap_token: "", payment_url: "https://pay.example/x" });
    await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
  });

  it.each([
    ["id", "Pilih ulang kurir pengiriman sebelum membayar."],
    ["en", "Choose a shipping courier again before paying."],
  ] as const)("explains shipping_required in %s", async (lang, message) => {
    useUIStore.setState({ lang });
    authFetch.mockRejectedValueOnce(new ApiError("shipping_required", "order requires a shipping selection before checkout", 400));
    renderCheckout();
    await pay();
    await waitFor(() => expect(toastError).toHaveBeenCalledWith(message));
    expect(open).not.toHaveBeenCalled();
    expect(writeText).not.toHaveBeenCalled();
  });

  it("keeps the toast-only path for other errors", async () => {
    authFetch.mockRejectedValueOnce(new ApiError("payment_failed", "gateway down", 502));
    renderCheckout();

    await pay();

    await waitFor(() => expect(toastError).toHaveBeenCalledWith("gateway down"));
    expect(screen.queryByRole("alert")).toBeNull();
  });

  it("still opens the payment link when checkout succeeds", async () => {
    authFetch.mockResolvedValueOnce({ snap_token: "", payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
    expect(screen.queryByRole("alert")).toBeNull();
  });
});


describe("SnapCheckout popup", () => {
  it.each([["id", "Bayar"], ["en", "Pay"]] as const)("renders the payment label in %s", (lang, label) => {
    useUIStore.setState({ lang });
    renderCheckout();
    expect(screen.getByRole("button", { name: label })).toBeEnabled();
  });

  it.each([
    ["onSuccess", "/orders", true],
    ["onPending", "/orders", true],
    ["onClose", "/orders", false],
    ["onError", "/orders", false],
    ["onSuccess", "/admin/bulk-exam-orders", false],
  ] as const)("%s on %s: goes to the order page = %s", async (event, basePath, goes) => {
    const snapPay = vi.fn();
    window.snap = { pay: snapPay, embed: vi.fn() };
    authFetch.mockResolvedValue({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout(basePath);
    await pay();
    await waitFor(() => expect(snapPay).toHaveBeenCalled());
    act(() => snapPay.mock.calls[0][1][event]({ transaction_status: "settlement" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled());
    if (goes) expect(push).toHaveBeenCalledWith("/orders/order-1");
    else expect(push).not.toHaveBeenCalled();
  });

  it("explains a failed payment in English without opening a tab", async () => {
    useUIStore.setState({ lang: "en" });
    const snapPay = vi.fn();
    window.snap = { pay: snapPay, embed: vi.fn() };
    authFetch.mockResolvedValue({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout();
    await pay();
    await waitFor(() => expect(snapPay).toHaveBeenCalled());
    act(() => snapPay.mock.calls[0][1].onError({ transaction_status: "deny" }));
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Payment failed. Try again or choose another payment method."));
    expect(open).not.toHaveBeenCalled();
  });

  it.each(["onSuccess", "onPending", "onClose", "onError"] as const)("handles %s on the page and allows reopening the same token", async (event) => {
    const snapPay = vi.fn();
    window.snap = { pay: snapPay, embed: vi.fn() };
    authFetch.mockResolvedValue({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    await pay();
    const button = screen.getByRole("button", { name: "Bayar" });
    await waitFor(() => expect(button).toBeDisabled());
    expect(snapPay).toHaveBeenCalledWith("test-token", expect.any(Object));
    expect(open).not.toHaveBeenCalled();
    await userEvent.click(button);
    expect(snapPay).toHaveBeenCalledTimes(1);
    invalidate.mockClear();
    act(() => snapPay.mock.calls[0][1][event]({ transaction_status: "settlement" }));
    await waitFor(() => expect(button).toBeEnabled());
    if (event !== "onError") expect(invalidate).toHaveBeenCalledWith({ queryKey: ordersKeys.all });
    if (event === "onError") expect(toastError).toHaveBeenCalledWith("Pembayaran gagal. Coba lagi atau pilih metode pembayaran lain.");
    if (event === "onSuccess" || event === "onPending") expect(toastInfo).toHaveBeenCalledWith(expect.stringContaining("menunggu konfirmasi"));
    expect(open).not.toHaveBeenCalled();
    if (event === "onSuccess" || event === "onPending") expect(push).toHaveBeenCalledWith("/orders/order-1");
    else expect(push).not.toHaveBeenCalled();
    expect(client.getQueryData(ordersKeys.detail("order-1"))).toBeUndefined();
    await pay();
    await waitFor(() => expect(button).toBeDisabled());
    expect(snapPay).toHaveBeenCalledTimes(2);
  });

  it("opens a token-only response without copying it", async () => {
    window.snap = { pay: vi.fn(), embed: vi.fn() };
    authFetch.mockResolvedValueOnce({ snap_token: "token-only" });
    renderCheckout();
    await pay();
    await waitFor(() => expect(screen.getByRole("button", { name: "Bayar" })).toBeDisabled());
    expect(window.snap.pay).toHaveBeenCalledWith("token-only", expect.any(Object));
    expect(writeText).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });

  it("keeps free orders out of the payment gateway", async () => {
    window.snap = { pay: vi.fn(), embed: vi.fn() };
    authFetch.mockResolvedValueOnce({ free: true, snap_token: "", payment_url: "" });
    renderCheckout();
    await pay();
    await waitFor(() => expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled());
    expect(push).toHaveBeenCalledWith("/orders/order-1");
    expect(window.snap.pay).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });

  it("shows the existing error when neither token nor URL is available", async () => {
    authFetch.mockResolvedValueOnce({ snap_token: "", payment_url: "" });
    renderCheckout();
    await pay();
    await waitFor(() => expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled());
    expect(toastError).toHaveBeenCalledWith("Gagal memulai pembayaran. Coba lagi nanti.");
    expect(open).not.toHaveBeenCalled();
  });
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

it("takes the new-tab path at once when the page never loads snap.js (admin bulk order)", async () => {
  open.mockReturnValue({});
  authFetch.mockResolvedValueOnce({ snap_token: "token", payment_url: "https://pay.example/x" });
  renderCheckout("/admin/bulk-exam-orders");
  await pay();
  await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
  expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled();
  expect(push).not.toHaveBeenCalled();
});

it("waits three seconds for snap.js that is still loading before the new-tab path", async () => {
  addSnapScript();
  vi.useFakeTimers();
  open.mockReturnValue({});
  authFetch.mockResolvedValueOnce({ snap_token: "token", payment_url: "https://pay.example/x" });
  renderCheckout();
  const button = screen.getByRole("button", { name: "Bayar" });
  await act(async () => { fireEvent.click(button); await vi.advanceTimersByTimeAsync(0); });
  expect(button).toBeDisabled();
  await act(async () => { await vi.advanceTimersByTimeAsync(2900); });
  expect(open).not.toHaveBeenCalled();
  await act(async () => { await vi.advanceTimersByTimeAsync(100); });
  expect(button).toBeEnabled();
  expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
  expect(toastInfo).toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
  expect(push).not.toHaveBeenCalled();
});

it("routes to the order when Snap throws and the fallback tab is blocked", async () => {
  window.snap = { pay: vi.fn(() => { throw new Error("unavailable"); }), embed: vi.fn() };
  open.mockReturnValue(null);
  authFetch.mockResolvedValueOnce({ snap_token: "token", payment_url: "https://pay.example/x" });
  renderCheckout();
  await pay();
  await waitFor(() => expect(screen.getByRole("button", { name: "Bayar" })).toBeEnabled());
  expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
  expect(push).toHaveBeenCalledWith("/orders/order-1");
});

it("guards synchronous double clicks while checkout is pending", async () => {
  authFetch.mockReturnValue(new Promise(() => {}));
  renderCheckout();
  const button = screen.getByRole("button", { name: "Bayar" });
  act(() => { fireEvent.click(button); fireEvent.click(button); });
  await waitFor(() => expect(button).toBeDisabled());
  expect(authFetch).toHaveBeenCalledTimes(1);
  expect(open).not.toHaveBeenCalled();
});
