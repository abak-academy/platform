import { describe, it, expect, beforeEach } from "vitest";
import {
  loadQueue,
  saveQueue,
  clearQueue,
  backoffDelayMs,
  type QueuedAnswer,
} from "./exam-session-queue";

beforeEach(() => {
  localStorage.clear();
});

describe("exam-session-queue", () => {
  it("returns an empty array when nothing has been queued for a session (NFR-R5)", () => {
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("round-trips a saved payload through loadQueue (FR-33)", () => {
    const entries: QueuedAnswer[] = [
      { question_id: "q1", answer: "A", flagged_for_review: false },
      { question_id: "q2", answer: "B", flagged_for_review: true },
    ];
    saveQueue("session-1", entries);
    expect(loadQueue("session-1")).toEqual(entries);
  });

  it("clearQueue removes the payload so a later load returns empty (FR-33)", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false },
    ]);
    clearQueue("session-1");
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("saveQueue overwrites the previous payload for the same session", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false },
    ]);
    saveQueue("session-1", [
      { question_id: "q1", answer: "Z", flagged_for_review: false },
    ]);
    expect(loadQueue("session-1")).toEqual([
      { question_id: "q1", answer: "Z", flagged_for_review: false },
    ]);
  });

  it("keys the queue per session id — one session's queue never leaks into another's", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false },
    ]);
    expect(loadQueue("session-2")).toEqual([]);
  });

  it("returns an empty array instead of throwing on malformed JSON (NFR-R5 durability)", () => {
    localStorage.setItem("exam-session-queue:session-1", "{not json");
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("returns an empty array when the stored value is not an array", () => {
    localStorage.setItem(
      "exam-session-queue:session-1",
      JSON.stringify({ not: "an array" }),
    );
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("backoffDelayMs grows exponentially per attempt (FR-32)", () => {
    const d0 = backoffDelayMs(0);
    const d1 = backoffDelayMs(1);
    const d2 = backoffDelayMs(2);
    expect(d1).toBeGreaterThan(d0);
    expect(d2).toBeGreaterThan(d1);
    expect(d1).toBe(d0 * 2);
    expect(d2).toBe(d0 * 4);
  });

  it("backoffDelayMs is capped so retries never grow unbounded (FR-32)", () => {
    const capped = backoffDelayMs(20);
    expect(backoffDelayMs(21)).toBe(capped);
  });
});
