export type CardState =
  | { kind: "start" }
  | { kind: "locked"; opensAt: Date }
  | { kind: "checkin" }
  | { kind: "in_progress" }
  | { kind: "expired" }
  | { kind: "submitted"; sessionId: string | null; hasAttemptsLeft: boolean };

export interface CardStateInput {
  status: string;
  session_id?: string | null;
  max_attempts: number | null;
  attempts_used: number;
  requires_checkin: boolean;
  scheduled_at: string | null;
  scheduled_end_at: string | null;
  check_in_window_minutes: number | null;
}

// Mirrors design-app-abak's PkgCard state machine (stateMeta: free/locked/
// checkin/checkedin/inprogress/expired/submitted), driven by real fields
// instead of mock data.
export function computeCardState(reg: CardStateInput, now: number): CardState {
  if (reg.status === "submitted") {
    // nil max_attempts is unlimited; 0 or 1 is a single sitting.
    const maxAttempts = reg.max_attempts;
    const unlimited = maxAttempts == null;
    const ceiling = maxAttempts != null && maxAttempts > 0 ? maxAttempts : 1;
    return {
      kind: "submitted",
      sessionId: reg.session_id ?? null,
      hasAttemptsLeft: unlimited || reg.attempts_used < ceiling,
    };
  }
  if (reg.status === "in_progress") return { kind: "in_progress" };
  if (reg.status === "checked_in") {
    const startsAt = reg.requires_checkin && reg.scheduled_at ? new Date(reg.scheduled_at).getTime() : null;
    if (startsAt !== null && now < startsAt) return { kind: "locked", opensAt: new Date(startsAt) };
    return { kind: "start" };
  }
  if (!reg.requires_checkin) {
    // Even without check-in, a set scheduled_end_at turns scheduled_at into an
    // availability window (backend gates StartSession on it regardless of
    // requires_checkin) — mirror that here so "Start" isn't shown outside it.
    if (reg.scheduled_end_at) {
      const start = reg.scheduled_at ? new Date(reg.scheduled_at).getTime() : null;
      const end = new Date(reg.scheduled_end_at).getTime();
      if (start !== null && now < start) return { kind: "locked", opensAt: new Date(start) };
      if (now > end) return { kind: "expired" };
    }
    return { kind: "start" };
  }
  if (!reg.scheduled_at) return { kind: "checkin" };
  const scheduledAt = new Date(reg.scheduled_at).getTime();
  const windowMin = reg.check_in_window_minutes ?? 15;
  const opensAt = scheduledAt - windowMin * 60_000;
  if (now < opensAt) return { kind: "locked", opensAt: new Date(opensAt) };
  // Without an availability window, check-in closes the instant scheduled_at
  // is reached (windowEnd collapses to scheduledAt) — with one, it stays open
  // through scheduled_end_at, matching the backend CheckIn windowing.
  const windowEnd = reg.scheduled_end_at ? new Date(reg.scheduled_end_at).getTime() : scheduledAt;
  if (now < windowEnd) return { kind: "checkin" };
  return { kind: "expired" };
}
