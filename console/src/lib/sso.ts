// Pure OIDC/SSO helpers — no DOM, no I/O, so they unit-test in isolation.
//
// After a successful SSO login the gateway 302-redirects the browser back to
// the console with the minted credentials in the URL *fragment*:
//
//   https://console/#token=agu-…&email=me@acme.com&role=member
//
// The fragment never reaches a server, so it's the safe channel for the SPA to
// pick up the token. parseAuthFragment turns that fragment into a plain object;
// App reads it on load, stores the token, clears the fragment, and calls whoami.

import type { AuthRole } from "./rbac";
import type { WhoAmI } from "./types";

export interface AuthFragment {
  token?: string;
  email?: string;
  role?: string;
}

/**
 * Parse a `window.location.hash` (`#token=…&email=…&role=…`) into its parts.
 * Tolerant of a leading `#`, an empty/absent hash, and partial fragments —
 * only the keys we understand are returned, each present only when non-empty.
 */
export function parseAuthFragment(hash: string): AuthFragment {
  const raw = hash.startsWith("#") ? hash.slice(1) : hash;
  if (!raw) return {};
  const params = new URLSearchParams(raw);
  const out: AuthFragment = {};
  const token = params.get("token");
  const email = params.get("email");
  const role = params.get("role");
  if (token) out.token = token;
  if (email) out.email = email;
  if (role) out.role = role;
  return out;
}

// The identity the console drives its UI from once whoami has resolved.
export interface Identity {
  role: AuthRole;
  orgId: string;
  email: string;
}

/**
 * Derive the console identity from a whoami response. The root admin key maps
 * to the `root` superuser (no org, no email); a user token carries its own
 * role/org/email. This is the single source of truth for the caller's role —
 * the console no longer asks the user to type it.
 */
export function identityFromWhoAmI(who: WhoAmI): Identity {
  if (who.root) return { role: "root", orgId: "", email: "" };
  return { role: who.role, orgId: who.org_id, email: who.email };
}
