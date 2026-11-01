import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { payWithSnap } from "./snap";

const paymentUrl = "https://pay.example/x";
let open: ReturnType<typeof vi.fn>;
beforeEach(() => {
  window.snap = undefined;
  open = vi.fn().mockReturnValue({});
  vi.stubGlobal("open", open);
});
afterEach(() => {
  document.head.querySelectorAll("script[data-test-snap]").forEach((el) => el.remove());
  vi.useRealTimers();
  vi.unstubAllGlobals();
  window.snap = undefined;
});

function addSnapScript() {
  const script = document.createElement("script");
  script.src = "https://app.sandbox.midtrans.com/snap/snap.js";
  script.setAttribute("data-test-snap", "");
  document.head.appendChild(script);
}

describe("payWithSnap", () => {
  it("prefers the popup and reports each completion", async () => {
    const pay = vi.fn();
    window.snap = { pay, embed: vi.fn() };
    const onFinished = vi.fn();
    expect(await payWithSnap({ snapToken: "token", paymentUrl, onFinished })).toBe("popup");
    expect(pay).toHaveBeenCalledWith("token", expect.any(Object));
    expect(open).not.toHaveBeenCalled();
    const callbacks = pay.mock.calls[0][1];
    callbacks.onSuccess({});
    callbacks.onPending({});
    callbacks.onError({});
    callbacks.onClose();
    expect(onFinished.mock.calls).toEqual([["success"], ["pending"], ["error"], ["close"]]);
    expect(open).not.toHaveBeenCalled();
  });

  it.each([false, true])("falls back without a token (blocked: %s)", async (blocked) => {
    window.snap = { pay: vi.fn(), embed: vi.fn() };
    open.mockReturnValue(blocked ? null : {});
    expect(await payWithSnap({ snapToken: "", paymentUrl })).toBe(blocked ? "blocked" : "new-tab");
    expect(open).toHaveBeenCalledWith(paymentUrl, "_blank");
    expect(window.snap.pay).not.toHaveBeenCalled();
  });

  it("falls back at once when the page never loads snap.js", async () => {
    vi.useFakeTimers();
    expect(await payWithSnap({ snapToken: "token", paymentUrl })).toBe("new-tab");
    expect(open).toHaveBeenCalledWith(paymentUrl, "_blank");
  });

  it("waits three seconds before falling back when snap.js is still loading", async () => {
    addSnapScript();
    vi.useFakeTimers();
    const result = payWithSnap({ snapToken: "token", paymentUrl });
    await vi.advanceTimersByTimeAsync(2900);
    expect(open).not.toHaveBeenCalled();
    await vi.advanceTimersByTimeAsync(100);
    expect(await result).toBe("new-tab");
    expect(open).toHaveBeenCalledWith(paymentUrl, "_blank");
  });

  it("uses Snap if the script arrives during the wait", async () => {
    addSnapScript();
    vi.useFakeTimers();
    const result = payWithSnap({ snapToken: "token", paymentUrl });
    await vi.advanceTimersByTimeAsync(500);
    window.snap = { pay: vi.fn(), embed: vi.fn() };
    await vi.advanceTimersByTimeAsync(100);
    expect(await result).toBe("popup");
    expect(window.snap.pay).toHaveBeenCalledWith("token", expect.any(Object));
    expect(open).not.toHaveBeenCalled();
  });

  it("falls back when Snap throws before opening", async () => {
    window.snap = { pay: vi.fn(() => { throw new Error("unavailable"); }), embed: vi.fn() };
    expect(await payWithSnap({ snapToken: "token", paymentUrl })).toBe("new-tab");
    expect(open).toHaveBeenCalledWith(paymentUrl, "_blank");
  });

  it("reports unavailable payment when the fallback has no URL", async () => {
    await expect(payWithSnap({ snapToken: "", paymentUrl: "" })).rejects.toThrow();
    expect(open).not.toHaveBeenCalled();
  });
});
