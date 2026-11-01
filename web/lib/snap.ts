// Polls for window.snap availability, waiting up to timeoutMs milliseconds.
// Resolves true once available, false on timeout.
export function waitForSnap(timeoutMs = 10000): Promise<boolean> {
  const start = Date.now();
  return new Promise((resolve) => {
    function check() {
      if (typeof window !== "undefined" && window.snap) {
        resolve(true);
        return;
      }
      if (Date.now() - start >= timeoutMs) {
        resolve(false);
        return;
      }
      setTimeout(check, 100);
    }
    check();
  });
}

export type SnapFinished = "success" | "pending" | "error" | "close";

export async function payWithSnap({
  snapToken,
  paymentUrl,
  onFinished,
}: {
  snapToken?: string;
  paymentUrl?: string;
  onFinished?: (event: SnapFinished) => void;
}): Promise<"popup" | "new-tab" | "blocked"> {
  // Admin pages never load snap.js; waiting there only delays the tab past the browser's pop-up allowance.
  const snapExpected = !!window.snap || !!document.querySelector('script[src*="snap.js"]');
  if (snapToken && snapExpected && await waitForSnap(3000) && window.snap) {
    try {
      window.snap.pay(snapToken, {
        onSuccess: () => onFinished?.("success"),
        onPending: () => onFinished?.("pending"),
        onError: () => onFinished?.("error"),
        onClose: () => onFinished?.("close"),
      });
      return "popup";
    } catch {
      // pay() threw before opening: fall through to the new tab.
    }
  }
  if (!paymentUrl) throw new Error("Payment URL unavailable");
  return window.open(paymentUrl, "_blank") ? "new-tab" : "blocked";
}
