import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { RefundOrderModal } from "./RefundOrderModal";

let presignState: { mutateAsync: ReturnType<typeof vi.fn>; isPending: boolean };

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => presignState,
}));

vi.mock("sonner", () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

describe("RefundOrderModal", () => {
  beforeEach(() => {
    presignState = {
      mutateAsync: vi.fn().mockResolvedValue({
        url: "https://upload.example.com/put-here",
        key: "refund_proof/admin-1/trf.jpg",
      }),
      isPending: false,
    };
    global.URL.createObjectURL = vi.fn(() => "blob:local-preview");
    global.URL.revokeObjectURL = vi.fn();
  });

  // The whole point of this modal: the system does not move money, and an
  // admin must not be able to mark an order refunded believing it did.
  it("states that money and stock are not returned automatically", () => {
    render(
      <RefundOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="ORD-1"
        onRefund={vi.fn()}
        isPending={false}
      />
    );

    // Radix renders the dialog in a portal, so read from document.body — the
    // render container itself is empty and would match anything. The copy is
    // also broken up by <strong>, hence the flattened text.
    const text = document.body.textContent ?? "";
    expect(text).toMatch(/tidak\s*mengembalikan uang secara otomatis/i);
    expect(text).toMatch(/stok barang juga\s*tidak\s*dikembalikan/i);
  });

  it("cannot be submitted before a transfer receipt is uploaded", () => {
    const onRefund = vi.fn();
    render(
      <RefundOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="ORD-1"
        onRefund={onRefund}
        isPending={false}
      />
    );

    const submit = screen.getByRole("button", { name: /^tandai sudah direfund$/i });
    expect(submit).toBeDisabled();
    fireEvent.click(submit);
    expect(onRefund).not.toHaveBeenCalled();
  });

  it("uploads under the refund_proof kind and hands the key back on submit", async () => {
    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);
    const onRefund = vi.fn();

    render(
      <RefundOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="ORD-1"
        onRefund={onRefund}
        isPending={false}
      />
    );

    const fileInput = document.querySelector(
      'input[data-testid="refund-order-proof-input"]'
    ) as HTMLInputElement;
    const file = new File(["trf bytes"], "trf.jpg", { type: "image/jpeg" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(presignState.mutateAsync).toHaveBeenCalledWith({
        filename: "trf.jpg",
        content_type: "image/jpeg",
        kind: "refund_proof",
      });
    });

    await waitFor(() =>
      expect(screen.getByRole("button", { name: /^tandai sudah direfund$/i })).not.toBeDisabled()
    );

    fireEvent.click(screen.getByRole("button", { name: /^tandai sudah direfund$/i }));
    expect(onRefund).toHaveBeenCalledWith("refund_proof/admin-1/trf.jpg");

    vi.unstubAllGlobals();
  });

  // refund_proof is not served by the public /files/* proxy, so the preview
  // must come from the local File — a key-based URL would 404.
  it("previews the receipt locally, never through a key-based URL", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));

    render(
      <RefundOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="ORD-1"
        onRefund={vi.fn()}
        isPending={false}
      />
    );

    const fileInput = document.querySelector(
      'input[data-testid="refund-order-proof-input"]'
    ) as HTMLInputElement;
    fireEvent.change(fileInput, {
      target: { files: [new File(["x"], "trf.jpg", { type: "image/jpeg" })] },
    });

    await waitFor(() => expect(global.URL.createObjectURL).toHaveBeenCalled());
    // document.body, not the render container: the dialog lives in a portal, so
    // asserting on the container would pass against empty markup.
    const link = screen.getByRole("link", { name: /lihat bukti/i }) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("blob:local-preview");
    expect(document.body.innerHTML).not.toContain("refund_proof/admin-1/trf.jpg");

    vi.unstubAllGlobals();
  });
});
