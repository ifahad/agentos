// Pure RBAC capability map, mirroring the gateway's frozen Phase 5 contract.
//
// The gateway is the real enforcer (role checks on every /admin/* call); this
// module only drives UI affordances — hiding or disabling actions the current
// caller's role cannot perform, so the console never dangles a button that will
// only ever answer 403.
//
// No DOM / no I/O here: this is unit-tested in isolation.

import type { Role } from "./types";

// The console caller authenticates as EITHER the root admin key (a superuser
// above every org) or a user token (`agu-…`) carrying one org role.
export type AuthRole = Role | "root";

// Console actions map 1:1 onto the contract's capability-checked endpoints.
export type Action =
  | "org.create" // POST   /admin/orgs                     (root only)
  | "org.view" // GET    /admin/orgs                     (root only)
  | "user.invite" // POST   /admin/orgs/{id}/users
  | "user.remove" // DELETE /admin/orgs/{id}/users/{uid}
  | "user.view" // GET    /admin/orgs/{id}/users
  | "key.create" // POST   /admin/keys
  | "key.view" // GET    /admin/keys, /admin/usage
  | "usage.view" // GET    /admin/audit
  | "secret.view" // GET    /admin/secrets/status            (root only)
  | "provisioning.view" // GET /admin/orgs/{id}/users (SCIM view) (root only)
  | "agent.run"; // runtime POST /runs

export const ROLES: Role[] = ["owner", "admin", "member", "viewer"];

export const ALL_ACTIONS: Action[] = [
  "org.create",
  "org.view",
  "user.invite",
  "user.remove",
  "user.view",
  "key.create",
  "key.view",
  "usage.view",
  "secret.view",
  "provisioning.view",
  "agent.run",
];

// Non-root roles → the actions they may perform, straight from the contract:
//   owner  — manage users, keys, budgets, view all
//   admin  — manage users (below owner), keys, view
//   member — create/list own keys, run agents, view own usage
//   viewer — read-only usage/audit
// org.create / org.view / secret.view / provisioning.view are root-exclusive and
// therefore appear in no role's list; root is handled separately in can() as a
// blanket superuser.
const ROLE_ACTIONS: Record<Role, Action[]> = {
  owner: ["user.invite", "user.remove", "user.view", "key.create", "key.view", "usage.view", "agent.run"],
  admin: ["user.invite", "user.remove", "user.view", "key.create", "key.view", "usage.view", "agent.run"],
  member: ["user.view", "key.create", "key.view", "usage.view", "agent.run"],
  viewer: ["key.view", "usage.view"],
};

/** True when a caller with `role` may perform `action`. Root is a superuser. */
export function can(role: AuthRole, action: Action): boolean {
  if (role === "root") return true;
  return ROLE_ACTIONS[role].includes(action);
}

/** Narrow an arbitrary value to a valid org Role. */
export function isRole(x: unknown): x is Role {
  return typeof x === "string" && (ROLES as string[]).includes(x);
}

/**
 * Coerce a stored/typed value into an AuthRole. Unknown values fall back to
 * "root": a pre-Phase-5 install stored only the root admin key with no role,
 * and that caller was, in fact, the superuser.
 */
export function asAuthRole(x: unknown): AuthRole {
  if (x === "root") return "root";
  return isRole(x) ? x : "root";
}

/** Human-readable label for a role (used in the sidebar + settings). */
export function roleLabel(role: AuthRole): string {
  return role === "root" ? "Root admin" : role.charAt(0).toUpperCase() + role.slice(1);
}
