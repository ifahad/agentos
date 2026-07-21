// Pure presentation helpers for user provisioning state (Phase 7). No DOM / no
// I/O — unit-tested in isolation and reused by the Users and Provisioning pages.

/** The label + badge className for a user's active state. */
export interface ActiveBadge {
  label: string;
  className: string;
}

/**
 * Badge descriptor for a user's `active` flag: a green "active" badge for live
 * users, a dim "inactive" badge for deactivated ones. className is the full
 * value for the <span>, reusing the shared .badge styling.
 */
export function activeBadge(active: boolean): ActiveBadge {
  return active
    ? { label: "active", className: "badge pass" }
    : { label: "inactive", className: "badge inactive" };
}

/**
 * Human-readable rendering of a user's SCIM `external_id`: "IdP-managed" when
 * the user was provisioned by an identity provider, an em dash otherwise.
 */
export function formatExternalId(externalId: string | undefined | null): string {
  return externalId && externalId.trim() !== "" ? "IdP-managed" : "—";
}
