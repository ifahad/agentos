package rbac

import "testing"

func TestValidRole(t *testing.T) {
	for _, r := range []string{RoleOwner, RoleAdmin, RoleMember, RoleViewer} {
		if !ValidRole(r) {
			t.Errorf("ValidRole(%q) = false, want true", r)
		}
	}
	for _, r := range []string{"", "root", "superuser", "Owner"} {
		if ValidRole(r) {
			t.Errorf("ValidRole(%q) = true, want false", r)
		}
	}
}

func TestCanCapabilityMatrix(t *testing.T) {
	tests := []struct {
		role   string
		action Action
		want   bool
	}{
		// Org create/list are root-only: no role grants them.
		{RoleOwner, ActCreateOrg, false},
		{RoleOwner, ActListOrgs, false},
		{RoleAdmin, ActCreateOrg, false},

		// update_org: owner and admin yes; member and viewer no.
		{RoleOwner, ActUpdateOrg, true},
		{RoleAdmin, ActUpdateOrg, true},
		{RoleMember, ActUpdateOrg, false},
		{RoleViewer, ActUpdateOrg, false},

		// Owner manages users and keys, views everything.
		{RoleOwner, ActCreateUser, true},
		{RoleOwner, ActDeleteUser, true},
		{RoleOwner, ActListUsers, true},
		{RoleOwner, ActCreateKey, true},
		{RoleOwner, ActListKeys, true},
		{RoleOwner, ActViewUsage, true},
		{RoleOwner, ActViewAudit, true},

		// Admin mirrors owner on these capabilities (role scope enforced separately).
		{RoleAdmin, ActCreateUser, true},
		{RoleAdmin, ActDeleteUser, true},
		{RoleAdmin, ActListUsers, true},
		{RoleAdmin, ActCreateKey, true},
		{RoleAdmin, ActListKeys, true},
		{RoleAdmin, ActViewUsage, true},

		// Member: create/list own keys, list users, view usage/audit; no user mgmt.
		{RoleMember, ActCreateUser, false},
		{RoleMember, ActDeleteUser, false},
		{RoleMember, ActListUsers, true},
		{RoleMember, ActCreateKey, true},
		{RoleMember, ActListKeys, true},
		{RoleMember, ActViewUsage, true},
		{RoleMember, ActViewAudit, true},

		// Viewer: read-only usage/audit only.
		{RoleViewer, ActViewUsage, true},
		{RoleViewer, ActViewAudit, true},
		{RoleViewer, ActListKeys, false},
		{RoleViewer, ActListUsers, false},
		{RoleViewer, ActCreateKey, false},
		{RoleViewer, ActCreateUser, false},

		// Unknown role holds nothing.
		{"nobody", ActViewUsage, false},
	}
	for _, tt := range tests {
		if got := Can(tt.role, tt.action); got != tt.want {
			t.Errorf("Can(%q, %q) = %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}

func TestCanManageRole(t *testing.T) {
	tests := []struct {
		actor  string
		target string
		want   bool
	}{
		{RoleOwner, RoleOwner, true},
		{RoleOwner, RoleAdmin, true},
		{RoleOwner, RoleMember, true},
		{RoleOwner, RoleViewer, true},
		{RoleAdmin, RoleOwner, false}, // admin cannot manage owners
		{RoleAdmin, RoleAdmin, true},
		{RoleAdmin, RoleMember, true},
		{RoleAdmin, RoleViewer, true},
		{RoleMember, RoleViewer, false},
		{RoleViewer, RoleViewer, false},
		{RoleOwner, "bogus", false},
	}
	for _, tt := range tests {
		if got := CanManageRole(tt.actor, tt.target); got != tt.want {
			t.Errorf("CanManageRole(%q, %q) = %v, want %v", tt.actor, tt.target, got, tt.want)
		}
	}
}
