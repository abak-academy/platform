import { describe, expect, it } from "vitest";
import { partnerOrderErrorMessage } from "./partner-order-errors";
import { ApiError } from "@/lib/api";
import { t } from "@/lib/i18n";

const tr = (key: Parameters<typeof t>[1]) => t("id", key);

describe("partnerOrderErrorMessage", () => {
  it("names the students who already own the product", () => {
    const err = new ApiError("participant_already_owns", "x", 409, { details: { students: [{ name: "Ani" }, { name: "Budi" }] } });
    expect(partnerOrderErrorMessage(err, tr)).toBe("Siswa berikut sudah memiliki produk ini: Ani, Budi.");
  });

  it("says how much voucher quota is left", () => {
    const err = new ApiError("promo_quota_insufficient", "x", 422, { details: { remaining: 4 } });
    expect(partnerOrderErrorMessage(err, tr)).toBe("Sisa kuota voucher 4, tidak cukup untuk pesanan ini.");
  });

  it("explains scope and product refusals", () => {
    expect(partnerOrderErrorMessage(new ApiError("student_not_allowed", "x", 403), tr)).toMatch(/di luar sekolah/);
    expect(partnerOrderErrorMessage(new ApiError("product_not_allowed", "x", 403), tr)).toMatch(/tidak dibuka untuk Anda/);
    expect(partnerOrderErrorMessage(new ApiError("invalid_promo", "x", 422), tr)).toMatch(/Voucher tidak berlaku/);
  });

  it("falls back to the server message, then to a generic one", () => {
    expect(partnerOrderErrorMessage(new ApiError("something_else", "pesan server", 500), tr)).toBe("pesan server");
    expect(partnerOrderErrorMessage(new Error("boom"), tr)).toBe("Terjadi kesalahan. Coba lagi.");
  });
});
