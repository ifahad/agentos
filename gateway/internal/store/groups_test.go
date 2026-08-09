package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

// groupFixture makes an org and two users to hang membership off.
func groupFixture(t *testing.T, st Store) (orgID string, a, b *User) {
	t.Helper()
	ctx := context.Background()
	org, err := st.CreateOrg(ctx, "acme", 0)
	if err != nil {
		t.Fatalf("CreateOrg: %v", err)
	}
	a, _, err = st.CreateUser(ctx, org.ID, "a@acme.test", rbac.RoleMember)
	if err != nil {
		t.Fatalf("CreateUser a: %v", err)
	}
	b, _, err = st.CreateUser(ctx, org.ID, "b@acme.test", rbac.RoleViewer)
	if err != nil {
		t.Fatalf("CreateUser b: %v", err)
	}
	return org.ID, a, b
}

func TestGroupLifecycle(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, a, b := groupFixture(t, st)

		g, err := st.CreateGroup(ctx, orgID, "AgentOS-Admins", "idp-guid-1")
		if err != nil {
			t.Fatalf("CreateGroup: %v", err)
		}
		if g.ID == "" || g.DisplayName != "AgentOS-Admins" || g.ExternalID != "idp-guid-1" {
			t.Fatalf("CreateGroup returned %+v", g)
		}

		got, err := st.GroupByID(ctx, orgID, g.ID)
		if err != nil || got.DisplayName != "AgentOS-Admins" {
			t.Fatalf("GroupByID = %+v, %v", got, err)
		}
		// Identity providers do not guarantee casing between a create and a
		// later re-sync, so lookup by name must not be case-sensitive.
		if _, err := st.GroupByDisplayName(ctx, orgID, "agentos-admins"); err != nil {
			t.Errorf("GroupByDisplayName is case-sensitive: %v", err)
		}

		if err := st.SetGroupMembers(ctx, orgID, g.ID, []string{a.ID, b.ID}); err != nil {
			t.Fatalf("SetGroupMembers: %v", err)
		}
		members, err := st.GroupMembers(ctx, orgID, g.ID)
		if err != nil || len(members) != 2 {
			t.Fatalf("GroupMembers = %d members, %v", len(members), err)
		}

		// Wholesale replacement is how every PATCH form lands here, so shrinking
		// the set must actually drop the absent member.
		if err := st.SetGroupMembers(ctx, orgID, g.ID, []string{b.ID}); err != nil {
			t.Fatalf("SetGroupMembers shrink: %v", err)
		}
		members, _ = st.GroupMembers(ctx, orgID, g.ID)
		if len(members) != 1 || members[0].ID != b.ID {
			t.Fatalf("after shrink, members = %+v", members)
		}

		groups, err := st.GroupsForUser(ctx, orgID, b.ID)
		if err != nil || len(groups) != 1 || groups[0].ID != g.ID {
			t.Fatalf("GroupsForUser(b) = %+v, %v", groups, err)
		}
		if groups, _ := st.GroupsForUser(ctx, orgID, a.ID); len(groups) != 0 {
			t.Errorf("GroupsForUser(a) = %+v, want none after removal", groups)
		}

		if err := st.DeleteGroup(ctx, orgID, g.ID); err != nil {
			t.Fatalf("DeleteGroup: %v", err)
		}
		if _, err := st.GroupByID(ctx, orgID, g.ID); !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("after delete, GroupByID = %v, want ErrGroupNotFound", err)
		}
	})
}

func TestGroupDisplayNameIsUniquePerOrg(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, _, _ := groupFixture(t, st)
		if _, err := st.CreateGroup(ctx, orgID, "Engineering", ""); err != nil {
			t.Fatalf("CreateGroup: %v", err)
		}
		// Entra retries a create until it receives a conflict, so this must be
		// ErrGroupExists and not some generic failure.
		for _, dup := range []string{"Engineering", "engineering", "ENGINEERING"} {
			if _, err := st.CreateGroup(ctx, orgID, dup, ""); !errors.Is(err, ErrGroupExists) {
				t.Errorf("CreateGroup(%q) = %v, want ErrGroupExists", dup, err)
			}
		}
		other, err := st.CreateOrg(ctx, "other", 0)
		if err != nil {
			t.Fatalf("CreateOrg: %v", err)
		}
		if _, err := st.CreateGroup(ctx, other.ID, "Engineering", ""); err != nil {
			t.Errorf("the name must be free in a different org: %v", err)
		}
	})
}

func TestRenameGroupRejectsACollision(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, _, _ := groupFixture(t, st)
		g1, _ := st.CreateGroup(ctx, orgID, "Engineering", "")
		g2, _ := st.CreateGroup(ctx, orgID, "Sales", "")

		if err := st.RenameGroup(ctx, orgID, g2.ID, "engineering"); !errors.Is(err, ErrGroupExists) {
			t.Errorf("RenameGroup onto a taken name = %v, want ErrGroupExists", err)
		}
		// Renaming to its own name (differing only in case) is not a collision.
		if err := st.RenameGroup(ctx, orgID, g1.ID, "ENGINEERING"); err != nil {
			t.Errorf("RenameGroup to its own name = %v, want nil", err)
		}
		if err := st.RenameGroup(ctx, orgID, g2.ID, "Revenue"); err != nil {
			t.Errorf("RenameGroup to a free name = %v", err)
		}
	})
}

func TestSetGroupMembersRejectsUnknownUserAtomically(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, a, _ := groupFixture(t, st)
		g, _ := st.CreateGroup(ctx, orgID, "Engineering", "")
		if err := st.SetGroupMembers(ctx, orgID, g.ID, []string{a.ID}); err != nil {
			t.Fatalf("seed members: %v", err)
		}
		// A rejected id must leave the previous membership untouched: a
		// half-applied replacement would silently drop members the IdP still
		// believes are there.
		err := st.SetGroupMembers(ctx, orgID, g.ID, []string{a.ID, "usr_nope"})
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("SetGroupMembers with an unknown id = %v, want ErrUserNotFound", err)
		}
		members, _ := st.GroupMembers(ctx, orgID, g.ID)
		if len(members) != 1 || members[0].ID != a.ID {
			t.Errorf("membership was mutated by a rejected write: %+v", members)
		}
	})
}

func TestGroupMembershipFollowsUserDeletion(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, a, b := groupFixture(t, st)
		g, _ := st.CreateGroup(ctx, orgID, "Engineering", "")
		if err := st.SetGroupMembers(ctx, orgID, g.ID, []string{a.ID, b.ID}); err != nil {
			t.Fatalf("SetGroupMembers: %v", err)
		}
		if err := st.DeleteUser(ctx, orgID, a.ID); err != nil {
			t.Fatalf("DeleteUser: %v", err)
		}
		// Postgres gets this from ON DELETE CASCADE and Memory does it by hand;
		// this is the parity check. A dangling membership would make
		// GroupMembers name a user that no longer exists.
		members, err := st.GroupMembers(ctx, orgID, g.ID)
		if err != nil {
			t.Fatalf("GroupMembers: %v", err)
		}
		if len(members) != 1 || members[0].ID != b.ID {
			t.Errorf("members after deleting a = %+v, want only b", members)
		}
	})
}

func TestGroupsAreOrgScoped(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		orgID, _, _ := groupFixture(t, st)
		other, _ := st.CreateOrg(ctx, "other", 0)
		g, _ := st.CreateGroup(ctx, orgID, "Engineering", "")

		// SCIM is pinned to one org; reading a group through the wrong one must
		// be indistinguishable from it not existing.
		if _, err := st.GroupByID(ctx, other.ID, g.ID); !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("cross-org GroupByID = %v, want ErrGroupNotFound", err)
		}
		if err := st.DeleteGroup(ctx, other.ID, g.ID); !errors.Is(err, ErrGroupNotFound) {
			t.Errorf("cross-org DeleteGroup = %v, want ErrGroupNotFound", err)
		}
		if groups, _ := st.Groups(ctx, other.ID); len(groups) != 0 {
			t.Errorf("Groups(other) = %+v, want none", groups)
		}
	})
}
