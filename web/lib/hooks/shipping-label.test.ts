import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

vi.mock("@/lib/api", () => ({
  API_BASE: "http://localhost:8080/api/v1",
  ApiError: class extends Error {
    code: string;
    status: number;
    constructor(code: string, message: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock("@/stores/auth", () => ({
  useAuthStore: {
    getState: () => ({ token: "test-token" }),
  },
}));

import { downloadShippingLabel } from "./shipping-label";

describe("downloadShippingLabel", () => {
  beforeEach(() => {
    URL.createObjectURL = vi.fn().mockReturnValue("blob:test-url");
    URL.revokeObjectURL = vi.fn();
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("fetches the packing-slip PDF with an Authorization header", async () => {
    const mockBlob = new Blob(["%PDF-1.4"], { type: "application/pdf" });
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      blob: () => Promise.resolve(mockBlob),
    });

    await downloadShippingLabel("order-1");

    expect(global.fetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/v1/admin/orders/order-1/label",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer test-token" }),
      }),
    );
    expect(URL.createObjectURL).toHaveBeenCalledWith(mockBlob);
  });

  it("throws on a non-2xx response (e.g. the 422 no_tracking_number refusal) instead of downloading an error body", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 422,
    });

    await expect(downloadShippingLabel("order-1")).rejects.toThrow();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });
});
