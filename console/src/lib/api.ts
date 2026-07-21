// Typed same-origin API client. All requests go through /api/gateway/... and
// /api/runtime/... — nginx (prod) or the vite dev proxy maps them onto the
// actual services, so the SPA never needs CORS.
//
// Request construction is kept in pure functions (buildRequest) so it can be
// unit-tested without a DOM or network.

export const GATEWAY_BASE = "/api/gateway";
export const RUNTIME_BASE = "/api/runtime";

// Where the SSO button sends the browser. The gateway 302s to the provider and,
// after callback, 302s back to the console with the token in the URL fragment.
export const SSO_LOGIN_URL = `${GATEWAY_BASE}/auth/oidc/login`;

const ADMIN_KEY_STORAGE = "agentos.adminKey";
const AUTH_ROLE_STORAGE = "agentos.authRole";
const ORG_ID_STORAGE = "agentos.orgId";
const EMAIL_STORAGE = "agentos.email";

export interface RequestSpec {
  url: string;
  init: RequestInit;
}

export interface BuildOptions {
  method?: string;
  body?: unknown;
  bearer?: string;
}

/** Pure builder for a fetch request: URL joining, JSON body, auth header. */
export function buildRequest(base: string, path: string, opts: BuildOptions = {}): RequestSpec {
  const headers: Record<string, string> = {};
  const init: RequestInit = {
    method: opts.method ?? (opts.body === undefined ? "GET" : "POST"),
    headers,
  };
  if (opts.bearer) {
    headers["Authorization"] = `Bearer ${opts.bearer}`;
  }
  if (opts.body !== undefined) {
    headers["Content-Type"] = "application/json";
    init.body = JSON.stringify(opts.body);
  }
  const cleanPath = path.startsWith("/") ? path : `/${path}`;
  return { url: `${base}${cleanPath}`, init };
}

/**
 * Gateway /admin/* request — always carries the caller's token as Bearer. The
 * same header carries either the root admin key or an `agu-…` user token; the
 * gateway resolves which. Pass `{ method }` for verbs beyond GET/POST (DELETE).
 */
export function gatewayAdminRequest(
  path: string,
  adminKey: string,
  body?: unknown,
  opts: { method?: string } = {},
): RequestSpec {
  return buildRequest(GATEWAY_BASE, path, { bearer: adminKey, body, method: opts.method });
}

/** Runtime request — no auth (runtime is internal; nginx fronts it). */
export function runtimeRequest(path: string, body?: unknown): RequestSpec {
  return buildRequest(RUNTIME_BASE, path, { body });
}

/** GET /admin/whoami — resolves the caller's Bearer token to an identity. */
export function whoamiRequest(adminKey: string): RequestSpec {
  return gatewayAdminRequest("/admin/whoami", adminKey);
}

/** GET /auth/oidc/status — public; the console uses it to show/hide SSO. */
export function oidcStatusRequest(): RequestSpec {
  return buildRequest(GATEWAY_BASE, "/auth/oidc/status");
}

/**
 * POST /admin/secrets/reload (root only) — forces the active secret source to
 * re-fetch and returns the fresh /admin/secrets/status array. Sent with an
 * empty JSON body so it is a POST.
 */
export function reloadSecretsRequest(adminKey: string): RequestSpec {
  return gatewayAdminRequest("/admin/secrets/reload", adminKey, {});
}

// ---- Credential persistence ----
//
// The caller authenticates with a single Bearer token that is EITHER the root
// admin key OR an `agu-…` user token — the gateway resolves which. Alongside it
// we persist a derived role (so the UI can gate actions) and, for user-token
// callers, their org id (so the Users page knows which org to scope to).

function readStorage(key: string): string {
  try {
    return window.localStorage.getItem(key) ?? "";
  } catch {
    return "";
  }
}

function writeStorage(key: string, value: string): void {
  try {
    if (value) {
      window.localStorage.setItem(key, value);
    } else {
      window.localStorage.removeItem(key);
    }
  } catch {
    // storage unavailable — value lives only in memory for this page load
  }
}

export function getAdminKey(): string {
  return readStorage(ADMIN_KEY_STORAGE);
}

export function saveAdminKey(key: string): void {
  writeStorage(ADMIN_KEY_STORAGE, key);
}

/** Persisted role string ("root" | "owner" | "admin" | "member" | "viewer"). */
export function getStoredRole(): string {
  return readStorage(AUTH_ROLE_STORAGE);
}

export function saveStoredRole(role: string): void {
  writeStorage(AUTH_ROLE_STORAGE, role);
}

/** Persisted org id for user-token callers (empty for root). From whoami. */
export function getStoredOrgId(): string {
  return readStorage(ORG_ID_STORAGE);
}

export function saveStoredOrgId(orgId: string): void {
  writeStorage(ORG_ID_STORAGE, orgId);
}

/** Persisted email for user-token callers (empty for root). From whoami. */
export function getStoredEmail(): string {
  return readStorage(EMAIL_STORAGE);
}

export function saveStoredEmail(email: string): void {
  writeStorage(EMAIL_STORAGE, email);
}

// ---- Error handling ----

export class ApiError extends Error {
  readonly status: number;
  readonly type: string;
  // Seconds until a retry may succeed, read from a 429's `Retry-After` header.
  readonly retryAfter?: number;

  constructor(status: number, type: string, message: string, retryAfter?: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.type = type;
    if (retryAfter !== undefined) this.retryAfter = retryAfter;
  }
}

/**
 * Parse a `Retry-After` header (delta-seconds or an HTTP-date) into seconds.
 * Pure: returns undefined for a missing/unparseable header.
 */
export function parseRetryAfter(header: string | null | undefined): number | undefined {
  if (!header) return undefined;
  const trimmed = header.trim();
  if (trimmed === "") return undefined;
  const secs = Number(trimmed);
  if (Number.isFinite(secs) && secs >= 0) return secs;
  const when = Date.parse(trimmed);
  if (!Number.isNaN(when)) {
    const diff = Math.round((when - Date.now()) / 1000);
    return diff > 0 ? diff : 0;
  }
  return undefined;
}

/** "3s" from a seconds count; "a moment" when unknown/non-positive. */
export function formatRetryAfter(seconds: number | undefined): string {
  if (seconds === undefined || !Number.isFinite(seconds) || seconds <= 0) return "a moment";
  return `${Math.ceil(seconds)}s`;
}

/**
 * Extract a human-readable error from a response body. The gateway answers
 * {"error":{"type","message"}}; the runtime (FastAPI) answers {"detail": ...}.
 * `retryAfter` (from a 429's header) is carried through when present.
 */
export function parseErrorBody(status: number, body: unknown, retryAfter?: number): ApiError {
  if (body && typeof body === "object") {
    const rec = body as Record<string, unknown>;
    const err = rec["error"];
    if (err && typeof err === "object") {
      const e = err as Record<string, unknown>;
      return new ApiError(
        status,
        typeof e["type"] === "string" ? e["type"] : "error",
        typeof e["message"] === "string" ? e["message"] : `HTTP ${status}`,
        retryAfter,
      );
    }
    const detail = rec["detail"];
    if (typeof detail === "string") {
      return new ApiError(status, "error", detail, retryAfter);
    }
    if (detail !== undefined) {
      return new ApiError(status, "error", JSON.stringify(detail), retryAfter);
    }
  }
  return new ApiError(status, "error", `HTTP ${status}`, retryAfter);
}

/** Fetch + JSON-decode, throwing ApiError on non-2xx. */
export async function apiFetch<T>(spec: RequestSpec): Promise<T> {
  const res = await fetch(spec.url, spec.init);
  const body = await safeJson(res);
  if (!res.ok) {
    throw parseErrorBody(res.status, body, parseRetryAfter(res.headers.get("Retry-After")));
  }
  return body as T;
}

/** Fetch + JSON-decode keeping the status code (Playground needs 200 vs 202). */
export async function apiFetchRaw(spec: RequestSpec): Promise<{ status: number; body: unknown }> {
  const res = await fetch(spec.url, spec.init);
  const body = await safeJson(res);
  if (!res.ok && res.status !== 202) {
    throw parseErrorBody(res.status, body, parseRetryAfter(res.headers.get("Retry-After")));
  }
  return { status: res.status, body };
}

async function safeJson(res: Response): Promise<unknown> {
  try {
    return await res.json();
  } catch {
    return undefined;
  }
}
