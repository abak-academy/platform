import { describe, it, expect, beforeEach } from "vitest";

import { getExamDeviceId } from "./exam-device";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("getExamDeviceId", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("creates a uuid and persists it under exam-device-id", () => {
    const id = getExamDeviceId();

    expect(id).toMatch(UUID_RE);
    expect(localStorage.getItem("exam-device-id")).toBe(id);
  });

  it("returns the same id on later calls", () => {
    expect(getExamDeviceId()).toBe(getExamDeviceId());
  });

  it("reuses an id already in storage", () => {
    localStorage.setItem("exam-device-id", "stored-id");

    expect(getExamDeviceId()).toBe("stored-id");
  });
});
