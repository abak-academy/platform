import { describe, it, expect, vi, afterEach, beforeEach } from "vitest";
import { apiFetch, authFetch, ApiError } from "./api";
import { useAuthStore } from "@/stores/auth";
import type { User } from "@/lib/types";

describe("apiFetch error body", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("retains the parsed error JSON on ApiError.body", async () => {
    const payload = {
      code: "verification_pending",
      message: "verify your email",
      otp_required: true,
      pending_token: "tok-123",
      id: "user@example.com",
    };
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify(payload), {
          status: 403,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(apiFetch("/auth/login", { method: "POST" })).rejects.toMatchObject({
      code: "verification_pending",
      status: 403,
      body: payload,
    });
  });

  it("leaves body undefined when the response has no JSON body", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(new Response("", { status: 500, statusText: "Server Error" })),
    );

    try {
      await apiFetch("/whatever");
      throw new Error("expected apiFetch to reject");
    } catch (err) {
      expect(err).toBeInstanceOf(ApiError);
      expect((err as ApiError).body).toBeUndefined();
    }
  });
});

describe("authFetch session refresh", () => {
  const student = { id: "u1", name: "Siswa", role: "student" } as User;

  beforeEach(() => {
    useAuthStore.setState({ token: null, refreshToken: null, user: null });
    vi.stubGlobal("location", { href: "" });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    useAuthStore.setState({ token: null, refreshToken: null, user: null });
  });

  it("renews an expired access token when the session has a user", async () => {
    useAuthStore.setState({ token: "stale", refreshToken: "rt", user: student });
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("", { status: 401 }))
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ access_token: "fresh", refresh_token: "rt2" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ok: true }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      );
    vi.stubGlobal("fetch", fetchMock);

    await expect(authFetch("/students/profile")).resolves.toEqual({ ok: true });

    expect(fetchMock.mock.calls[1][0]).toContain("/auth/refresh");
    expect(useAuthStore.getState().token).toBe("fresh");
    expect(useAuthStore.getState().user).toEqual(student);
  });

  it("refuses to renew a session that has no user, and clears it instead", async () => {
    // The shape a pre-`user` persisted blob rehydrates into: token present,
    // identity absent. Sliding it forward is what kept an unidentifiable
    // session alive indefinitely.
    useAuthStore.setState({ token: "orphan", refreshToken: "rt", user: null });
    const fetchMock = vi.fn().mockResolvedValue(new Response("", { status: 401 }));
    vi.stubGlobal("fetch", fetchMock);

    await expect(authFetch("/students/profile")).rejects.toBeInstanceOf(ApiError);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls.every(([url]) => !String(url).includes("/auth/refresh"))).toBe(true);
    expect(useAuthStore.getState().token).toBeNull();
    expect(window.location.href).toBe("/login");
  });
});
