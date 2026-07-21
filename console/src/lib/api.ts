// Typed same-origin API client. All requests go through /api/gateway/... and
// /api/runtime/... — nginx (prod) or the vite dev proxy maps them onto the
// actual services, so the SPA never needs CORS.
//
// Request construction is kept in pure functions (buildRequest) so it can be
// unit-tested without a DOM or network.

export const GATEWAY_BASE = "/api/gateway";
export const RUNTIME_BASE = "/api/runtime";

const ADMIN_KEY_STORAGE = "agentos.adminKey";

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

/** Gateway /admin/* request — always carries the admin key as Bearer. */
export function gatewayAdminRequest(path: string, adminKey: string, body?: unknown): RequestSpec {
  return buildRequest(GATEWAY_BASE, path, { bearer: adminKey, body });
}

/** Runtime request — no auth (runtime is internal; nginx fronts it). */
export function runtimeRequest(path: string, body?: unknown): RequestSpec {
  return buildRequest(RUNTIME_BASE, path, { body });
}

// ---- Admin key persistence ----

export function getAdminKey(): string {
  try {
    return window.localStorage.getItem(ADMIN_KEY_STORAGE) ?? "";
  } catch {
    return "";
  }
}

export function saveAdminKey(key: string): void {
  try {
    if (key) {
      window.localStorage.setItem(ADMIN_KEY_STORAGE, key);
    } else {
      window.localStorage.removeItem(ADMIN_KEY_STORAGE);
    }
  } catch {
    // storage unavailable — key lives only in memory for this page load
  }
}

// ---- Error handling ----

export class ApiError extends Error {
  readonly status: number;
  readonly type: string;

  constructor(status: number, type: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.type = type;
  }
}

/**
 * Extract a human-readable error from a response body. The gateway answers
 * {"error":{"type","message"}}; the runtime (FastAPI) answers {"detail": ...}.
 */
export function parseErrorBody(status: number, body: unknown): ApiError {
  if (body && typeof body === "object") {
    const rec = body as Record<string, unknown>;
    const err = rec["error"];
    if (err && typeof err === "object") {
      const e = err as Record<string, unknown>;
      return new ApiError(
        status,
        typeof e["type"] === "string" ? e["type"] : "error",
        typeof e["message"] === "string" ? e["message"] : `HTTP ${status}`,
      );
    }
    const detail = rec["detail"];
    if (typeof detail === "string") {
      return new ApiError(status, "error", detail);
    }
    if (detail !== undefined) {
      return new ApiError(status, "error", JSON.stringify(detail));
    }
  }
  return new ApiError(status, "error", `HTTP ${status}`);
}

/** Fetch + JSON-decode, throwing ApiError on non-2xx. */
export async function apiFetch<T>(spec: RequestSpec): Promise<T> {
  const res = await fetch(spec.url, spec.init);
  const body = await safeJson(res);
  if (!res.ok) {
    throw parseErrorBody(res.status, body);
  }
  return body as T;
}

/** Fetch + JSON-decode keeping the status code (Playground needs 200 vs 202). */
export async function apiFetchRaw(spec: RequestSpec): Promise<{ status: number; body: unknown }> {
  const res = await fetch(spec.url, spec.init);
  const body = await safeJson(res);
  if (!res.ok && res.status !== 202) {
    throw parseErrorBody(res.status, body);
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
