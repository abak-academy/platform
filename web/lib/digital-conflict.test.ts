import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api";
import { t as translate, type I18nKey, type Lang } from "@/lib/i18n";
import { digitalConflictMessage } from "./digital-conflict";

const TARGET = "0b6f7c1e-3a52-4c1d-9d0e-6f1b1f2f7a11";
const PRODUCT = "5d3c9a44-8f0b-4a7e-8f6a-2c4e9b1d0a22";

function conflictError(conflicts: unknown[]) {
  return new ApiError("digital_target_conflict", "digital product already owned or being purchased", 409, {
    code: "digital_target_conflict",
    message: "digital product already owned or being purchased",
    details: { conflicts },
  });
}

function messageFor(lang: Lang, err: unknown) {
  return digitalConflictMessage(err, (key: I18nKey) => translate(lang, key));
}

function conflict(kind: string, reason: string) {
  return { product_id: PRODUCT, kind, target_id: TARGET, reason };
}

describe("digitalConflictMessage", () => {
  it("tells the student they already own the target", () => {
    const msg = messageFor("id", conflictError([conflict("course", "owned")]));
    expect(msg).toBe("Kamu sudah memiliki kursus ini.");
  });

  it("tells the student to finish or wait for another pending order when the target is claimed", () => {
    const msg = messageFor("id", conflictError([conflict("exam", "claimed")]));
    expect(msg).toBe(
      "Ujian ini sedang dibeli lewat pesanan lain yang belum selesai. Selesaikan atau tunggu pesanan tersebut, lalu coba lagi.",
    );
  });

  it("tells the student to remove one of two items that grant the same target", () => {
    const msg = messageFor("id", conflictError([conflict("course", "duplicate_in_cart")]));
    expect(msg).toBe("Dua produk di keranjang memberi akses ke kursus yang sama. Hapus salah satunya.");
  });

  it("localizes to English", () => {
    const msg = messageFor("en", conflictError([conflict("exam", "owned")]));
    expect(msg).toBe("You already have this exam.");
  });

  it("reports each distinct reason once and never leaks target or product ids", () => {
    const msg = messageFor(
      "id",
      conflictError([
        conflict("course", "owned"),
        conflict("course", "owned"),
        conflict("exam", "claimed"),
      ]),
    );
    expect(msg).toContain("Kamu sudah memiliki kursus ini.");
    expect(msg).toContain("Ujian ini sedang dibeli");
    expect(msg?.match(/Kamu sudah memiliki/g)).toHaveLength(1);
    expect(msg).not.toContain(TARGET);
    expect(msg).not.toContain(PRODUCT);
  });

  it("falls back to a generic conflict message when the details are missing", () => {
    const err = new ApiError("digital_target_conflict", "raw backend text", 409);
    const msg = messageFor("id", err);
    expect(msg).toBe(
      "Keranjang berisi produk digital yang sudah dimiliki atau sedang dibeli. Periksa keranjang, lalu coba lagi.",
    );
  });

  it("falls back to the generic message for an unknown reason or kind", () => {
    const msg = messageFor("id", conflictError([conflict("workshop", "owned")]));
    expect(msg).toBe(
      "Keranjang berisi produk digital yang sudah dimiliki atau sedang dibeli. Periksa keranjang, lalu coba lagi.",
    );
  });

  it("explains an unlinked digital product as unavailable", () => {
    const err = new ApiError("digital_product_unavailable", "digital product has no targets", 409);
    expect(messageFor("id", err)).toBe(
      "Produk digital ini belum tersedia untuk dibeli. Hapus dari keranjang atau hubungi admin.",
    );
  });

  it("returns null for any other error so callers keep their own handling", () => {
    expect(messageFor("id", new ApiError("invalid_qty", "bad qty", 400))).toBeNull();
    expect(messageFor("id", new Error("network down"))).toBeNull();
    expect(messageFor("id", undefined)).toBeNull();
  });
});
