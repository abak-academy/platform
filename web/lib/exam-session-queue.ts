// Durable local buffer for exam session answers (FR-31..FR-34, FR-37, NFR-R5).
// A browser-storage safety net only: the server is always the source of truth,
// and every entry here is cleared as soon as the server acknowledges it.

export interface QueuedAnswer {
  question_id: string;
  answer: string;
  flagged_for_review: boolean;
  revision: number;
}

const KEY_PREFIX = "exam-session-queue:";

export const AUTOSAVE_DEBOUNCE_MS = 2000;
const RETRY_BASE_MS = 2000;
const RETRY_MAX_MS = 30000;

function storageKey(sessionId: string): string {
  return `${KEY_PREFIX}${sessionId}`;
}

function isQueuedAnswer(value: unknown): value is QueuedAnswer {
  if (!value || typeof value !== "object") return false;
  const entry = value as Partial<QueuedAnswer>;
  return (
    typeof entry.question_id === "string" &&
    typeof entry.answer === "string" &&
    typeof entry.flagged_for_review === "boolean" &&
    typeof entry.revision === "number" &&
    Number.isInteger(entry.revision) &&
    entry.revision > 0
  );
}

function normalizeEntries(value: unknown): QueuedAnswer[] {
  if (!Array.isArray(value)) return [];
  return value.filter(isQueuedAnswer);
}

function loadState(sessionId: string): {
  entries: QueuedAnswer[];
  next_revision: number;
} {
  try {
    const raw = localStorage.getItem(storageKey(sessionId));
    if (!raw) return { entries: [], next_revision: 1 };
    const parsed = JSON.parse(raw);
    const storedEntries = Array.isArray(parsed)
      ? parsed.map((entry, index) =>
          isQueuedAnswer(entry) ? entry : { ...entry, revision: index + 1 },
        )
      : parsed?.entries;
    const entries = normalizeEntries(storedEntries);
    const maxRevision = entries.reduce(
      (max, entry) => Math.max(max, entry.revision),
      0,
    );
    const parsedNext = Array.isArray(parsed) ? undefined : parsed?.next_revision;
    const nextRevision =
      typeof parsedNext === "number" &&
      Number.isInteger(parsedNext) &&
      parsedNext > maxRevision
        ? parsedNext
        : maxRevision + 1;
    return { entries, next_revision: Math.max(1, nextRevision) };
  } catch {
    return { entries: [], next_revision: 1 };
  }
}

function saveState(
  sessionId: string,
  state: { entries: QueuedAnswer[]; next_revision: number },
): void {
  localStorage.setItem(storageKey(sessionId), JSON.stringify(state));
}

export function loadQueue(sessionId: string): QueuedAnswer[] {
  return loadState(sessionId).entries;
}

export function saveQueue(sessionId: string, entries: QueuedAnswer[]): void {
  const current = loadState(sessionId);
  const maxRevision = entries.reduce(
    (max, entry) => Math.max(max, entry.revision),
    0,
  );
  saveState(sessionId, {
    entries,
    next_revision: Math.max(current.next_revision, maxRevision + 1),
  });
}

export function clearQueue(sessionId: string): void {
  try {
    localStorage.removeItem(storageKey(sessionId));
  } catch {
    /* best-effort */
  }
}

export function queueAnswerDelta(
  sessionId: string,
  entry: Omit<QueuedAnswer, "revision">,
): QueuedAnswer {
  const state = loadState(sessionId);
  const queued = { ...entry, revision: state.next_revision };
  const entries = state.entries.filter(
    (item) => item.question_id !== entry.question_id,
  );
  entries.push(queued);
  saveState(sessionId, {
    entries,
    next_revision: state.next_revision + 1,
  });
  return queued;
}

export function acknowledgeQueueRevisions(
  sessionId: string,
  acknowledged: QueuedAnswer[],
): QueuedAnswer[] {
  const state = loadState(sessionId);
  const acked = new Map(
    acknowledged.map((entry) => [entry.question_id, entry.revision]),
  );
  const remaining = state.entries.filter(
    (entry) => acked.get(entry.question_id) !== entry.revision,
  );
  saveState(sessionId, {
    entries: remaining,
    next_revision: state.next_revision,
  });
  return remaining;
}

export function queueToSavePayload(entries: QueuedAnswer[]) {
  return entries.map(({ question_id, answer, flagged_for_review }) => ({
    question_id,
    answer,
    flagged_for_review,
  }));
}

// Exponential backoff for retry attempt N (0-indexed), capped at RETRY_MAX_MS.
export function backoffDelayMs(attempt: number): number {
  return Math.min(RETRY_MAX_MS, RETRY_BASE_MS * 2 ** attempt);
}
