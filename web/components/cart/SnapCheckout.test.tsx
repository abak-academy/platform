import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { SnapCheckout } from "./SnapCheckout";
import { ApiError } from "@/lib/api";
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

function renderCheckout(basePath?: string) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
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
  authFetch.mockReset();
  useUIStore.setState({ lang: "id" });
  open = vi.fn();
  writeText = vi.fn();
  vi.stubGlobal("open", open);
  Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
});

async function pay() {
  await userEvent.click(screen.getByRole("button", { name: /bayar di tab baru/i }));
}

describe("SnapCheckout conflicts", () => {
  it("routes to the order with a blocked-popup toast when the payment popup is blocked", async () => {
    open.mockReturnValue(null);
    authFetch.mockResolvedValueOnce({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /bayar di tab baru/i })).toBeEnabled());
    expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
    expect(push).toHaveBeenCalledWith("/orders/order-1");
    expect(toastInfo).toHaveBeenCalledWith("Pop-up pembayaran diblokir browser. Lanjutkan pembayaran dari halaman pesanan.");
    expect(toastInfo).not.toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
  });

  it("keeps the new-tab toast without navigation when the payment popup opens", async () => {
    open.mockReturnValue({});
    authFetch.mockResolvedValueOnce({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /bayar di tab baru/i })).toBeEnabled());
    expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank");
    expect(push).not.toHaveBeenCalled();
    expect(toastInfo).toHaveBeenCalledWith("Selesaikan pembayaran di tab baru, lalu refresh halaman ini.");
    expect(toastInfo).not.toHaveBeenCalledWith("Pop-up pembayaran diblokir browser. Lanjutkan pembayaran dari halaman pesanan.");
  });

  it("does not route admin checkout to student orders when the popup is blocked", async () => {
    open.mockReturnValue(null);
    authFetch.mockResolvedValueOnce({ snap_token: "test-token", payment_url: "https://pay.example/x" });
    renderCheckout("/admin/orders");

    await pay();

    await waitFor(() => expect(screen.getByRole("button", { name: /bayar di tab baru/i })).toBeEnabled());
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

    resolve({ payment_url: "https://pay.example/x" });
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
    authFetch.mockResolvedValueOnce({ payment_url: "https://pay.example/x" });
    renderCheckout();

    await pay();

    await waitFor(() => expect(open).toHaveBeenCalledWith("https://pay.example/x", "_blank"));
    expect(screen.queryByRole("alert")).toBeNull();
  });
});
