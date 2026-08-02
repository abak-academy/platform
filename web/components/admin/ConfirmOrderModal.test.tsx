import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { ConfirmOrderModal } from "./ConfirmOrderModal";

type PresignInput = { filename: string; content_type: string; kind?: string };
type PresignOutput = { url: string; method: "PUT"; key: string };
type PresignFn = (input: PresignInput) => Promise<PresignOutput>;

let presignState: {
  mutateAsync: PresignFn;
  isPending: boolean;
} = {
  mutateAsync: vi.fn(),
  isPending: false,
};

vi.mock("@/lib/hooks/students", () => ({
  usePresignUpload: () => presignState,
}));

beforeEach(() => {
  presignState = {
    // FR-26 wire shape pinned by students.test.tsx's usePresignUpload cases:
    // { url, method, key } with the key under the requested kind's prefix.
    mutateAsync: vi.fn().mockResolvedValue({
      url: "https://upload.example.com/put-here",
      method: "PUT",
      key: "payment_proof/admin-1/proof.jpg",
    }),
    isPending: false,
  };
});

describe("ConfirmOrderModal", () => {
  it("disables Konfirmasi until a proof has been uploaded", () => {
    render(
      <ConfirmOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="#ABCD1234"
        onConfirm={vi.fn()}
        isPending={false}
      />
    );

    expect(screen.getByRole("button", { name: "Konfirmasi" })).toBeDisabled();
  });

  it("does not call onConfirm when the disabled submit is clicked without a proof", () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="#ABCD1234"
        onConfirm={onConfirm}
        isPending={false}
      />
    );

    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it("uploads the proof then submits the uploaded object key", async () => {
    const onConfirm = vi.fn();
    render(
      <ConfirmOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="#ABCD1234"
        onConfirm={onConfirm}
        isPending={false}
      />
    );

    const fetchSpy = vi.fn().mockResolvedValue({ ok: true });
    vi.stubGlobal("fetch", fetchSpy);

    const fileInput = document.querySelector(
      'input[data-testid="confirm-order-proof-input"]'
    ) as HTMLInputElement;
    const file = new File(["proof bytes"], "kwitansi.jpg", { type: "image/jpeg" });
    fireEvent.change(fileInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(presignState.mutateAsync).toHaveBeenCalledWith({
        filename: "kwitansi.jpg",
        content_type: "image/jpeg",
        kind: "payment_proof",
      });
    });

    await waitFor(() => {
      expect(fetchSpy).toHaveBeenCalledWith(
        "https://upload.example.com/put-here",
        expect.objectContaining({ method: "PUT", body: file })
      );
    });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Konfirmasi" })).not.toBeDisabled();
    });

    fireEvent.click(screen.getByRole("button", { name: "Konfirmasi" }));
    expect(onConfirm).toHaveBeenCalledWith("payment_proof/admin-1/proof.jpg");

    vi.unstubAllGlobals();
  });

  // payment_proof is not served by the public /files/* proxy, so a key-based
  // preview URL would 404. The preview must come from the local File.
  it("previews the uploaded proof locally, never through a key-based URL", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: true }));
    const createObjectURL = vi.fn(() => "blob:local-preview");
    global.URL.createObjectURL = createObjectURL;
    global.URL.revokeObjectURL = vi.fn();

    render(
      <ConfirmOrderModal
        open
        onOpenChange={vi.fn()}
        orderNumber="ORD-1"
        onConfirm={vi.fn()}
        isPending={false}
      />
    );

    const fileInput = document.querySelector(
      'input[data-testid="confirm-order-proof-input"]'
    ) as HTMLInputElement;
    fireEvent.change(fileInput, {
      target: { files: [new File(["x"], "kwitansi.jpg", { type: "image/jpeg" })] },
    });

    await waitFor(() => expect(createObjectURL).toHaveBeenCalled());

    // Radix portals the dialog, so read document.body — the render container
    // is empty and would satisfy any negative assertion.
    const link = screen.getByRole("link", { name: /lihat bukti/i }) as HTMLAnchorElement;
    expect(link.getAttribute("href")).toBe("blob:local-preview");
    expect(document.body.innerHTML).not.toContain("payment_proof/admin-1/proof.jpg");

    vi.unstubAllGlobals();
  });
});
