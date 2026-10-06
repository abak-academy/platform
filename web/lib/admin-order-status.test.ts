import { describe, expect, it } from "vitest";
import { adminOrderStatusLabel } from "./admin-order-status";
import { t } from "./i18n";
import type { Order } from "./types";

const order: Order = {
  id: "order-1", student_id: "student-1", status: "payment_pending",
  subtotal: 10000, discount: 0, shipping_cost: 0, total: 10000,
  items: [{ id: "item-1", order_id: "order-1", product_id: "product-1", product_type: "book", name: "Book", unit_price: 10000, qty: 1, jumlah: 10000 }],
};

const cases = [
  ["payment_pending", true, null, "Menunggu pembayaran", "Awaiting payment"],
  ["paid", true, null, "Siap kirim", "Ready to ship"],
  ["processing", true, null, "Siap kirim", "Ready to ship"],
  ["paid", false, null, "Dibayar", "Paid"],
  ["processing", false, null, "Diproses", "Processing"],
  ["shipped", true, "courierNotFound", "Pengiriman bermasalah", "Delivery problem"],
  ["shipped", true, "courier_not_found", "Pengiriman bermasalah", "Delivery problem"],
  ["shipped", true, null, "Dalam pengiriman", "In transit"],
  ["shipped", true, "on_hold", "Dalam pengiriman", "In transit"],
  ["completed", false, null, "Selesai", "Completed"],
  ["payment_expired", true, null, "Kedaluwarsa", "Payment expired"],
  ["cancelled", true, null, "Dibatalkan", "Cancelled"],
] as const;

describe.each(["id", "en"] as const)("admin operator labels in %s", (lang) => {
  it.each(cases)("labels %s/physical=%s/shipment=%s", (status, physical, shipmentStatus, id, en) => {
    const value = { ...order, status, shipment_status: shipmentStatus,
      items: order.items!.map((item) => ({ ...item, product_type: physical ? "book" : "course" })) };
    expect(adminOrderStatusLabel(value, (key) => t(lang, key))).toBe(lang === "id" ? id : en);
  });
});
