// Durable local buffer for exam session answers (FR-31..FR-34, FR-37, NFR-R5).
// A browser-storage safety net only: the server is always the source of truth,
// and every entry here is cleared as soon as the server acknowledges it.

export interface QueuedAnswer {
  question_id: string;
  answer: string;
  flagged_for_review: boolean;
}

const KEY_PREFIX = "exam-session-queue:";

export const AUTOSAVE_DEBOUNCE_MS = 2000;
const RETRY_BASE_MS = 2000;
const RETRY_MAX_MS = 30000;

function storageKey(sessionId: string): string {
  return `${KEY_PREFIX}${sessionId}`;
}

export function loadQueue(sessionId: string): QueuedAnswer[] {
  try {
    const raw = localStorage.getItem(storageKey(sessionId));
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export function saveQueue(sessionId: string, entries: QueuedAnswer[]): void {
  try {
    localStorage.setItem(storageKey(sessionId), JSON.stringify(entries));
  } catch {
    /* storage unavailable (private mode, quota) — best-effort durability only */
  }
}

export function clearQueue(sessionId: string): void {
  try {
    localStorage.removeItem(storageKey(sessionId));
  } catch {
    /* best-effort */
  }
}

// Exponential backoff for retry attempt N (0-indexed), capped at RETRY_MAX_MS.
export function backoffDelayMs(attempt: number): number {
  return Math.min(RETRY_MAX_MS, RETRY_BASE_MS * 2 ** attempt);
}
