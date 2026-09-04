import { describe, it, expect, beforeEach, vi } from "vitest";
import {
  loadQueue,
  saveQueue,
  clearQueue,
  queueAnswerDelta,
  acknowledgeQueueRevisions,
  queueToSavePayload,
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
      { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
      { question_id: "q2", answer: "B", flagged_for_review: true, revision: 2 },
    ];
    saveQueue("session-1", entries);
    expect(loadQueue("session-1")).toEqual(entries);
  });

  it("clearQueue removes the payload so a later load returns empty (FR-33)", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
    ]);
    clearQueue("session-1");
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("saveQueue overwrites the previous payload for the same session", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
    ]);
    saveQueue("session-1", [
      { question_id: "q1", answer: "Z", flagged_for_review: false, revision: 2 },
    ]);
    expect(loadQueue("session-1")).toEqual([
      { question_id: "q1", answer: "Z", flagged_for_review: false, revision: 2 },
    ]);
  });

  it("keys the queue per session id — one session's queue never leaks into another's", () => {
    saveQueue("session-1", [
      { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
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

  it("returns only valid entries when storage contains malformed queue rows", () => {
    localStorage.setItem(
      "exam-session-queue:session-1",
      JSON.stringify({
        next_revision: 3,
        entries: [
          { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
          { question_id: "q2", answer: "B", flagged_for_review: true },
        ],
      }),
    );

    expect(loadQueue("session-1")).toEqual([
      { question_id: "q1", answer: "A", flagged_for_review: false, revision: 1 },
    ]);
  });

  it("surfaces storage write failures so callers cannot report a non-durable edit as saved", () => {
    const setItem = vi
      .spyOn(localStorage, "setItem")
      .mockImplementation(() => {
        throw new Error("quota exceeded");
      });

    try {
      expect(() =>
        queueAnswerDelta("session-1", {
          question_id: "q1",
          answer: "A",
          flagged_for_review: false,
        }),
      ).toThrow("quota exceeded");
    } finally {
      setItem.mockRestore();
    }
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("stores edits as latest per-question values with session-local monotonic revisions", () => {
    const first = queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "A",
      flagged_for_review: false,
    });
    const second = queueAnswerDelta("session-1", {
      question_id: "q2",
      answer: "",
      flagged_for_review: true,
    });
    const third = queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "C",
      flagged_for_review: false,
    });

    expect([first.revision, second.revision, third.revision]).toEqual([1, 2, 3]);
    expect(loadQueue("session-1")).toEqual([
      { question_id: "q2", answer: "", flagged_for_review: true, revision: 2 },
      { question_id: "q1", answer: "C", flagged_for_review: false, revision: 3 },
    ]);
  });

  it("keeps revision counters isolated per session", () => {
    queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "A",
      flagged_for_review: false,
    });

    expect(
      queueAnswerDelta("session-2", {
        question_id: "q1",
        answer: "B",
        flagged_for_review: false,
      }),
    ).toMatchObject({ revision: 1 });
  });

  it("acknowledges only the exact carried revision for each question", () => {
    const revisionOne = queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "A",
      flagged_for_review: false,
    });
    const revisionTwo = queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "B",
      flagged_for_review: false,
    });
    const otherQuestion = queueAnswerDelta("session-1", {
      question_id: "q2",
      answer: "",
      flagged_for_review: true,
    });

    expect(acknowledgeQueueRevisions("session-1", [revisionOne])).toEqual([
      revisionTwo,
      otherQuestion,
    ]);
    expect(acknowledgeQueueRevisions("session-1", [revisionTwo])).toEqual([
      otherQuestion,
    ]);
    expect(acknowledgeQueueRevisions("session-1", [otherQuestion])).toEqual([]);
    expect(loadQueue("session-1")).toEqual([]);
  });

  it("continues monotonic revisions after an acknowledged queue is emptied", () => {
    const first = queueAnswerDelta("session-1", {
      question_id: "q1",
      answer: "A",
      flagged_for_review: false,
    });
    acknowledgeQueueRevisions("session-1", [first]);

    expect(
      queueAnswerDelta("session-1", {
        question_id: "q1",
        answer: "B",
        flagged_for_review: false,
      }),
    ).toMatchObject({ revision: 2 });
  });

  it("strips local revisions from the existing save endpoint payload", () => {
    expect(
      queueToSavePayload([
        {
          question_id: "q1",
          answer: "A",
          flagged_for_review: false,
          revision: 1,
        },
      ]),
    ).toEqual([
      { question_id: "q1", answer: "A", flagged_for_review: false },
    ]);
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
