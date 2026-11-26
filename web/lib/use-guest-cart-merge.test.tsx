import { StrictMode, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ApiError } from "@/lib/api";
import { useGuestCart } from "@/stores/guest-cart";
import { useGuestCartMerge, useMergeNoticeStore } from "./guest-cart-merge";

const { authFetch, toastError } = vi.hoisted(() => ({ authFetch: vi.fn(), toastError: vi.fn() }));
vi.mock("@/lib/api", async (orig) => ({ ...(await orig<typeof import("@/lib/api")>()), authFetch }));
vi.mock("sonner", () => ({ toast: { error: toastError } }));

const line = (id: string, over = {}) => ({
  product_id: id,
  qty: 1,
  name: `P-${id}`,
  type: "book" as const,
  unit_price: 1000,
  image_url: "",
  ...over,
});
const cart = (ids: string[], price = 1000) => ({
  id: "o1",
  status: "cart",
  items: ids.map((product_id) => ({ product_id, unit_price: price })),
});

const qc = new QueryClient();

function wrapper({ children }: { children: ReactNode }) {
  return (
    <StrictMode>
      <QueryClientProvider client={qc}>{children}</QueryClientProvider>
    </StrictMode>
  );
}

const posts = () => authFetch.mock.calls.filter(([path]) => /\/items$/.test(path));
const ids = () => useGuestCart.getState().lines.map((l) => l.product_id);

beforeEach(() => {
  localStorage.clear();
  useGuestCart.setState({ lines: [] });
  useMergeNoticeStore.getState().dismiss();
  authFetch.mockReset();
  toastError.mockReset();
});

describe("useGuestCartMerge", () => {
  it("merges once under StrictMode and removes the merged lines", async () => {
    useGuestCart.setState({ lines: [line("a"), line("b")] });
    const added: string[] = [];
    authFetch.mockImplementation(async (path: string, init: { body?: string }) => {
      if (path === "/orders") return cart([]);
      added.push(JSON.parse(init.body!).product_id);
      return cart(added);
    });
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await waitFor(() => expect(ids()).toEqual([]));
    expect(authFetch.mock.calls.filter(([p]) => p === "/orders")).toHaveLength(1);
        expect(posts()).toHaveLength(2);
    expect(useMergeNoticeStore.getState().outcomes).toEqual([]);
  });

  it("does not merge again when enabled toggles off and on", async () => {
    useGuestCart.setState({ lines: [line("a")] });
    authFetch.mockImplementation(async (path: string) => {
      if (path === "/orders") return cart([]);
      throw new ApiError("boom", "x", 500);
    });
    const { rerender } = renderHook(({ on }) => useGuestCartMerge(on), { wrapper, initialProps: { on: true } });
    await waitFor(() => expect(toastError).toHaveBeenCalled());
    rerender({ on: false });
    rerender({ on: true });
    await new Promise((r) => setTimeout(r, 20));
    expect(authFetch.mock.calls.filter(([p]) => p === "/orders")).toHaveLength(1);
  });

  it("puts non-plain outcomes in the notice store", async () => {
    useGuestCart.setState({ lines: [line("a")] });
    authFetch.mockImplementation(async (path: string) => {
      if (path === "/orders") return cart([]);
      throw new ApiError("out_of_stock", "x", 409, { code: "out_of_stock" });
    });
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await waitFor(() => expect(useMergeNoticeStore.getState().outcomes).toHaveLength(1));
    expect(useMergeNoticeStore.getState().outcomes[0]).toMatchObject({ product_id: "a", code: "out_of_stock" });
    expect(ids()).toEqual([]);
  });

  it("skips and removes a line flagged unavailable, reporting it", async () => {
    useGuestCart.setState({ lines: [line("a", { unavailable: true }), line("b")] });
    const added: string[] = [];
    authFetch.mockImplementation(async (path: string, init: { body?: string }) => {
      if (path === "/orders") return cart([]);
      added.push(JSON.parse(init.body!).product_id);
      return cart(added);
    });
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await waitFor(() => expect(ids()).toEqual([]));
    expect(added).toEqual(["b"]);
    expect(useMergeNoticeStore.getState().outcomes.map((o) => o.code)).toEqual(["unavailable"]);
  });

  it("toasts on failure, keeps the lines, and retries on the next mount", async () => {
    useGuestCart.setState({ lines: [line("a")] });
    authFetch.mockImplementation(async (path: string) => {
      if (path === "/orders") return cart([]);
      throw new ApiError("boom", "x", 500);
    });
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await waitFor(() => expect(toastError).toHaveBeenCalledWith("Keranjang belum tersinkron, coba lagi"));
    expect(ids()).toEqual(["a"]);

    authFetch.mockReset();
    authFetch.mockImplementation(async (path: string) => (path === "/orders" ? cart([]) : cart(["a"])));
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await waitFor(() => expect(ids()).toEqual([]));
  });

  it("does nothing when disabled or when the cart is empty", async () => {
    useGuestCart.setState({ lines: [line("a")] });
    renderHook(() => useGuestCartMerge(false), { wrapper });
    useGuestCart.setState({ lines: [] });
    renderHook(() => useGuestCartMerge(true), { wrapper });
    await new Promise((r) => setTimeout(r, 20));
    expect(authFetch).not.toHaveBeenCalled();
  });
});
