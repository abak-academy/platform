const API_BASE = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";

// fileUrl turns a stored object key (e.g. "avatars/<user>/<uuid>-pic.png") into
// the avatar read-proxy URL. Values that are already absolute or root-relative
// URLs (legacy rows) pass through unchanged.
export function fileUrl(key?: string | null): string | undefined {
  if (!key) return undefined;
  if (key.startsWith("http") || key.startsWith("/")) return key;
  return `${API_BASE}/files/${key}`;
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly body?: Record<string, unknown>;

  constructor(code: string, message: string, status: number, body?: Record<string, unknown>) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.body = body;
  }
}

interface ErrorBody {
  code?: string;
  message?: string;
  error?: string;
  [key: string]: unknown;
}

async function parseError(res: Response): Promise<ApiError> {
  const status = res.status;
  let body: ErrorBody | null = null;
  try {
    const text = await res.text();
    if (text) {
      body = JSON.parse(text) as ErrorBody;
    }
  } catch {
    // non-JSON body; fall through to status text
  }
  const message =
    body?.message ??
    body?.error ??
    res.statusText ??
    `Request failed with status ${status}`;
  const code = body?.code ?? `HTTP_${status}`;
  return new ApiError(code, message, status, body ?? undefined);
}

function withJsonHeaders(init?: RequestInit): HeadersInit {
  return { "Content-Type": "application/json", ...(init?.headers ?? {}) };
}

export { API_BASE };

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: withJsonHeaders(init),
  });
  if (!res.ok) {
    throw await parseError(res);
  }
  return res.json() as Promise<T>;
}

async function tryRefresh(): Promise<string | null> {
  const { useAuthStore } = await import("@/stores/auth");
  const { refreshToken, user } = useAuthStore.getState();
  // A stored session with no user is not renewable: nothing downstream can tell
  // who it belongs to, so the guards fall open. Refusing to refresh it lets the
  // 401 below clear the session instead of sliding it forward indefinitely.
  if (!refreshToken || !user) return null;
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refresh_token: refreshToken }),
    });
    if (!res.ok) return null;
    const data = (await res.json()) as { access_token: string; refresh_token: string };
    useAuthStore.getState().setSession(data.access_token, data.refresh_token, user);
    return data.access_token;
  } catch {
    return null;
  }
}

async function doAuthFetch<T>(
  path: string,
  init: RequestInit | undefined,
  baseHeaders: HeadersInit
): Promise<T> {
  const { useAuthStore } = await import("@/stores/auth");
  const token = useAuthStore.getState().token;

  const buildHeaders = (t: string | null): HeadersInit => ({
    ...baseHeaders,
    ...(init?.headers ?? {}),
    ...(t ? { Authorization: `Bearer ${t}` } : {}),
  });

  let res = await fetch(`${API_BASE}${path}`, { ...init, headers: buildHeaders(token) });

  if (res.status === 401) {
    const newToken = await tryRefresh();
    if (newToken) {
      res = await fetch(`${API_BASE}${path}`, { ...init, headers: buildHeaders(newToken) });
    }
  }

  if (!res.ok) {
    if (res.status === 401) {
      useAuthStore.getState().clear();
      if (typeof window !== "undefined") {
        window.location.href = "/login";
      }
    }
    throw await parseError(res);
  }
  if (res.status === 204 || res.status === 205) return undefined as T;
  return res.json() as Promise<T>;
}

export async function authFetch<T>(path: string, init?: RequestInit): Promise<T> {
  return doAuthFetch<T>(path, init, { "Content-Type": "application/json" });
}

export async function authFetchMultipart<T>(path: string, init?: RequestInit): Promise<T> {
  return doAuthFetch<T>(path, init, {});
}