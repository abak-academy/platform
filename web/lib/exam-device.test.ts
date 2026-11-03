import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";

import { getExamDeviceId } from "./exam-device";

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

describe("getExamDeviceId", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
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
    const stored = "11111111-1111-4111-8111-111111111111";
    localStorage.setItem("exam-device-id", stored);

    expect(getExamDeviceId()).toBe(stored);
  });

  it("replaces a stored value that is not a uuid", () => {
    localStorage.setItem("exam-device-id", "stored-id");

    const id = getExamDeviceId();

    expect(id).toMatch(UUID_RE);
    expect(localStorage.getItem("exam-device-id")).toBe(id);
  });

  it("returns null when storage throws", () => {
    vi.spyOn(Object.getPrototypeOf(localStorage), "getItem").mockImplementation(() => {
      throw new Error("denied");
    });

    expect(getExamDeviceId()).toBeNull();
  });
});
