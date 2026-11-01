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

import { downloadShippingLabel, downloadShippingLabels } from "./shipping-label";

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

  it("passes the single label size", async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: true, blob: async () => new Blob(["PDF"]) } as Response);
    await downloadShippingLabel("order-1", "roll80");
    expect(fetch).toHaveBeenCalledWith("http://localhost:8080/api/v1/admin/orders/order-1/label?size=roll80", expect.anything());
  });

  it("posts ordered ids and size and downloads one PDF", async () => {
    const blob = new Blob(["PDF"]);
    vi.mocked(fetch).mockResolvedValueOnce({ ok: true, blob: async () => blob } as Response);
    const click = vi.spyOn(HTMLAnchorElement.prototype, "click").mockImplementation(() => {});
    expect(await downloadShippingLabels(["second", "first"], "roll80")).toBeNull();
    expect(fetch).toHaveBeenCalledWith("http://localhost:8080/api/v1/admin/orders/labels", expect.objectContaining({ method: "POST", headers: expect.objectContaining({ "Content-Type": "application/json", Authorization: "Bearer test-token" }), body: JSON.stringify({ order_ids: ["second", "first"], size: "roll80" }) }));
    expect(URL.createObjectURL).toHaveBeenCalledWith(blob);
    expect(click).toHaveBeenCalledOnce();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:test-url");
  });

  it("returns 409 reasons without a download", async () => {
    const details = [{ order_id: "first", reason: "shipment_failed" }];
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false, status: 409, json: async () => ({ code: "labels_not_printable", details }) } as Response);
    expect(await downloadShippingLabels(["first"], "100x150")).toEqual(details);
    expect(URL.createObjectURL).not.toHaveBeenCalled();
  });

  it("throws a bulk error without downloading it", async () => {
    vi.mocked(fetch).mockResolvedValueOnce({ ok: false, status: 500 } as Response);
    await expect(downloadShippingLabels(["first"], "100x150")).rejects.toThrow();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
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
