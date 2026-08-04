import { describe, expect, it } from "vitest";

import {
  SHIPMENT_FAILURE_STATUSES,
  isShipmentFailure,
  normaliseShipmentStatus,
  shipmentStatusLabel,
} from "./shipment-status";

describe("normaliseShipmentStatus", () => {
  // Biteship's docs render these camelCase while payloads are snake_case, and
  // we have never seen a real one for most of them. Accept either rather than
  // guess, so a status we mapped does not fall through to a raw code.
  it("folds camelCase and snake_case onto the same key", () => {
    expect(normaliseShipmentStatus("courierNotFound")).toBe("courier_not_found");
    expect(normaliseShipmentStatus("courier_not_found")).toBe("courier_not_found");
    expect(normaliseShipmentStatus("droppingOff")).toBe("dropping_off");
    expect(normaliseShipmentStatus("  DELIVERED  ")).toBe("delivered");
  });
});

describe("shipmentStatusLabel", () => {
  it("translates every status Biteship documents", () => {
    const all = [
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
    ];
    for (const status of all) {
      const label = shipmentStatusLabel(status, "id");
      expect(label).not.toBe(status);
      expect(label).not.toBe("");
    }
  });

  it("reads the same for the camelCase spelling", () => {
    expect(shipmentStatusLabel("droppingOff", "id")).toBe(
      shipmentStatusLabel("dropping_off", "id"),
    );
  });

  // A status we have never seen must still be readable rather than blank —
  // Biteship can add one without telling us.
  it("falls back to the raw code for an unknown status", () => {
    expect(shipmentStatusLabel("teleported", "id")).toBe("teleported");
  });
});

describe("isShipmentFailure", () => {
  it("flags the statuses that mean the parcel is not moving", () => {
    for (const status of SHIPMENT_FAILURE_STATUSES) {
      expect(isShipmentFailure(status)).toBe(true);
    }
    expect(isShipmentFailure("courierNotFound")).toBe(true);
  });

  it("does not flag a healthy status", () => {
    for (const status of ["confirmed", "allocated", "in_transit", "delivered"]) {
      expect(isShipmentFailure(status)).toBe(false);
    }
  });

  it("treats an on-hold shipment as still in flight, not failed", () => {
    // on_hold resolves on its own; it is a warning, not a dead parcel.
    expect(isShipmentFailure("on_hold")).toBe(false);
  });
});
