// Package rbac holds the pure role/capability model for multi-tenant access
// control. It has no dependencies on the store or HTTP layers so the
// capability matrix can be unit-tested in isolation and mirrored by clients.
package rbac

// Roles from the frozen contract. A user carries exactly one role within its
// org; the root admin key is a global superuser above all roles.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
	RoleViewer = "viewer"
)

// ValidRole reports whether r is one of the four known roles.
func ValidRole(r string) bool {
	switch r {
	case RoleOwner, RoleAdmin, RoleMember, RoleViewer:
		return true
	default:
		return false
	}
}

// Action is a capability that a role may or may not hold.
type Action string

// Actions map onto the admin/RBAC endpoints. Org creation/listing hold no
// role grant: they are reserved for the root superuser.
const (
	ActCreateOrg  Action = "create_org"
	ActListOrgs   Action = "list_orgs"
	ActUpdateOrg  Action = "update_org"
	ActCreateUser Action = "create_user"
	ActListUsers  Action = "list_users"
	ActDeleteUser Action = "delete_user"
	ActCreateKey  Action = "create_key"
	ActListKeys   Action = "list_keys"
	ActViewUsage  Action = "view_usage"
	ActViewAudit  Action = "view_audit"
)

// capabilities is the role → allowed-actions matrix from the contract:
//   - owner:  manage org, users, keys, budgets, view all
//   - admin:  manage users (below owner), keys, view
//   - member: create/list own keys, run agents, view own usage
//   - viewer: read-only usage/audit
var capabilities = map[string]map[Action]bool{
	RoleOwner: {
		ActUpdateOrg:  true,
		ActCreateUser: true, ActListUsers: true, ActDeleteUser: true,
		ActCreateKey: true, ActListKeys: true,
		ActViewUsage: true, ActViewAudit: true,
	},
	RoleAdmin: {
		ActUpdateOrg:  true,
		ActCreateUser: true, ActListUsers: true, ActDeleteUser: true,
		ActCreateKey: true, ActListKeys: true,
		ActViewUsage: true, ActViewAudit: true,
	},
	RoleMember: {
		ActListUsers: true,
		ActCreateKey: true, ActListKeys: true,
		ActViewUsage: true, ActViewAudit: true,
	},
	RoleViewer: {
		ActViewUsage: true, ActViewAudit: true,
	},
}

// Can reports whether a role may perform action. Unknown roles hold nothing.
func Can(role string, action Action) bool {
	return capabilities[role][action]
}

// CanManageRole reports whether an actor with actorRole may create or delete a
// user whose role is targetRole. An owner manages any role; an admin manages
// admins and below but not owners; members and viewers manage no one.
func CanManageRole(actorRole, targetRole string) bool {
	switch actorRole {
	case RoleOwner:
		return ValidRole(targetRole)
	case RoleAdmin:
		return targetRole == RoleAdmin || targetRole == RoleMember || targetRole == RoleViewer
	default:
		return false
	}
}

// RoleRank orders roles by authority for callers that must collapse a SET of
// roles into one. SCIM group mapping is the case that needs it: a user may be
// in several groups that each map to a role, and exactly one role can be
// stored (User.Role is scalar), so the strongest wins.
//
// The order is read off the capability matrix above rather than invented.
// viewer's actions are a strict subset of member's, member's of admin's, and
// owner holds admin's actions plus the one power admin lacks — CanManageRole
// over owners. TestRoleRankIsDerivedFromCapabilities pins that derivation, so
// granting member something admin cannot do fails the build instead of quietly
// making this ordering a lie.
//
// An unknown role ranks BELOW every real one. Operator-supplied role names
// reach this function (AGENTOS_SCIM_GROUP_ROLES), and a typo there must never
// outrank viewer.
func RoleRank(role string) int {
	switch role {
	case RoleOwner:
		return 3
	case RoleAdmin:
		return 2
	case RoleMember:
		return 1
	case RoleViewer:
		return 0
	default:
		return -1
	}
}
