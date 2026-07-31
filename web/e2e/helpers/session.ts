import { expect, type BrowserContext, type APIRequestContext } from "@playwright/test";

/**
 * Shared session plumbing for every real-backend spec. Extracted before the
 * suite grows (2026-08-01): question-editor.spec.ts and e2-walkthrough.spec.ts
 * had already duplicated all of this, and the suite is planned to expand to
 * full student + admin flows.
 *
 * Accounts are provisioned manually — there is no seed script or bootstrap
 * endpoint, and registration+OTP is explicitly OUT of e2e scope (user
 * decision, 2026-08-01). Set:
 *   E2E_ADMIN_IDENTIFIER   / E2E_ADMIN_PASSWORD    — admin_exam or super_admin
 *   E2E_STUDENT_IDENTIFIER / E2E_STUDENT_PASSWORD  — a registered student
 */

export const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

export interface SeededSession {
  token: string;
  refreshToken: string;
  user: Record<string, unknown>;
}

// Mirrors the zustand persist shape stores/auth.ts writes under "abak-auth".
export async function seedSession(context: BrowserContext, session: SeededSession) {
  await context.addInitScript((s) => {
    window.localStorage.setItem(
      "abak-auth",
      JSON.stringify({ state: { token: s.token, refreshToken: s.refreshToken, user: s.user }, version: 0 })
    );
  }, session);
}

async function loginReal(
  context: BrowserContext,
  request: APIRequestContext,
  identifier: string | undefined,
  password: string | undefined,
  role: string
): Promise<string> {
  if (!identifier || !password) {
    throw new Error(
      `this case exercises the real backend and needs a real ${role} session: ` +
        `set E2E_${role.toUpperCase()}_IDENTIFIER and E2E_${role.toUpperCase()}_PASSWORD to a local account. ` +
        "This is an environment/credentials gap, not a repro."
    );
  }
  const loginRes = await request.post(`${API_BASE}/auth/login`, { data: { identifier, password } });
  if (!loginRes.ok()) {
    throw new Error(
      `real-backend setup: ${role} login failed (${loginRes.status()}) — ` +
        `check the E2E_${role.toUpperCase()}_* variables. Body: ${await loginRes.text()}`
    );
  }
  const session = (await loginRes.json()) as {
    access_token: string;
    refresh_token?: string;
    user: Record<string, unknown>;
  };
  await seedSession(context, {
    token: session.access_token,
    refreshToken: session.refresh_token ?? "",
    user: session.user,
  });
  return session.access_token;
}

/** Logs in the throwaway admin and seeds the browser session. Returns the bearer token. */
export function loginRealAdmin(context: BrowserContext, request: APIRequestContext): Promise<string> {
  return loginReal(context, request, process.env.E2E_ADMIN_IDENTIFIER, process.env.E2E_ADMIN_PASSWORD, "admin");
}

/** Logs in the manually-registered student account. Returns the bearer token. */
export function loginRealStudent(context: BrowserContext, request: APIRequestContext): Promise<string> {
  return loginReal(
    context,
    request,
    process.env.E2E_STUDENT_IDENTIFIER,
    process.env.E2E_STUDENT_PASSWORD,
    "student"
  );
}

/**
 * Reads a saved bank question back over the wire by a unique marker in its
 * body — the same list payload the edit dialog consumes, so anything visible
 * here is what the admin sees on reopen.
 */
export async function fetchSavedQuestion(
  request: APIRequestContext,
  token: string,
  marker: string
): Promise<Record<string, any>> {
  const res = await request.get(`${API_BASE}/admin/questions?search=${encodeURIComponent(marker)}`, {
    headers: { Authorization: `Bearer ${token}` },
  });
  expect(res.ok(), `list lookup for ${marker} should succeed`).toBeTruthy();
  const body = (await res.json()) as { data: Array<Record<string, any>> };
  expect(body.data.length, `exactly one question should match ${marker}`).toBe(1);
  return body.data[0];
}
