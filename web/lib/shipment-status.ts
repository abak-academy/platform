import { t, type Lang } from "@/lib/i18n";

/**
 * Every status Biteship documents, in happy-path order first and then the
 * branches that mean something went wrong.
 */
export const SHIPMENT_STATUSES = [
  "confirmed",
  "scheduled",
  "allocated",
  "picking_up",
  "picked",
  "in_transit",
  "dropping_off",
  "delivered",
  "on_hold",
  "rejected",
  "courier_not_found",
  "cancelled",
  "return_in_transit",
  "returned",
  "disposed",
] as const;

export type ShipmentStatus = (typeof SHIPMENT_STATUSES)[number];

/**
 * Statuses that mean the parcel is not going to arrive without someone
 * intervening. `on_hold` is deliberately absent: it resolves on its own and
 * flagging it would train admins to ignore the badge.
 */
export const SHIPMENT_FAILURE_STATUSES: readonly ShipmentStatus[] = [
  "rejected",
  "courier_not_found",
  "cancelled",
  "return_in_transit",
  "returned",
  "disposed",
];

/**
 * Biteship's docs render these camelCase (`courierNotFound`) while payloads
 * use snake_case (`courier_not_found`), and most have never been seen on the
 * wire here. Folding both onto one key costs nothing and avoids a status
 * silently falling through to its raw code.
 */
export function normaliseShipmentStatus(raw: string): string {
  return raw
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase();
}

export function shipmentStatusLabel(raw: string, lang: Lang): string {
  const key = normaliseShipmentStatus(raw);
  if (!(SHIPMENT_STATUSES as readonly string[]).includes(key)) return raw;
  // The dictionary is keyed by literal strings, so a computed key needs the
  // cast; the includes() guard above is what makes it safe.
  return t(lang, `shipment_status_${key}` as Parameters<typeof t>[1]);
}

export function isShipmentFailure(raw: string | null | undefined): boolean {
  if (!raw) return false;
  return (SHIPMENT_FAILURE_STATUSES as readonly string[]).includes(
    normaliseShipmentStatus(raw),
  );
}
