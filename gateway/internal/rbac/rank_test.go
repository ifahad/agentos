package rbac

import "testing"

// The ranking exists to be derived from the capability matrix, not asserted
// alongside it. This test is what makes that true: if someone grants member an
// action admin lacks, or reorders the roles, the containment below breaks and
// the build fails rather than RoleRank silently becoming a claim nobody checks.
func TestRoleRankIsDerivedFromCapabilities(t *testing.T) {
	roles := []string{RoleViewer, RoleMember, RoleAdmin, RoleOwner}

	// Collect every action any role holds, so the subset check cannot pass by
	// simply not knowing about an action.
	actions := map[Action]bool{}
	for _, caps := range capabilities {
		for a := range caps {
			actions[a] = true
		}
	}
	if len(actions) == 0 {
		t.Fatal("no actions found; the containment check below would be vacuous")
	}

	for _, lo := range roles {
		for _, hi := range roles {
			if RoleRank(lo) >= RoleRank(hi) {
				continue
			}
			for a := range actions {
				if Can(lo, a) && !Can(hi, a) {
					t.Errorf("%s ranks below %s but holds %q which %s does not: "+
						"the ranking no longer follows the capability matrix", lo, hi, a, hi)
				}
			}
		}
	}

	// owner and admin hold identical action sets, so containment alone cannot
	// separate them. The one power that does is management of owners.
	if !CanManageRole(RoleOwner, RoleOwner) {
		t.Error("owner must manage owners; that is what ranks it above admin")
	}
	if CanManageRole(RoleAdmin, RoleOwner) {
		t.Error("admin must not manage owners; without that, owner > admin is arbitrary")
	}
	if RoleRank(RoleOwner) <= RoleRank(RoleAdmin) {
		t.Error("owner must outrank admin")
	}
}

func TestRoleRankPutsUnknownRolesBelowEverything(t *testing.T) {
	// Operator config supplies these strings, so a typo must fail closed rather
	// than outranking a real role.
	for _, unknown := range []string{"", "Admin", "superuser", "owner ", "root"} {
		if got := RoleRank(unknown); got >= RoleRank(RoleViewer) {
			t.Errorf("RoleRank(%q) = %d, must rank below viewer (%d)",
				unknown, got, RoleRank(RoleViewer))
		}
	}
}
