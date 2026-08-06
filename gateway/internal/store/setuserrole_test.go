package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

// backends runs a test body against both Store implementations. Memory and
// Postgres diverging silently is a live risk here: every HTTP test in this
// package binds Memory, so a method that works in memory and not in Postgres
// would ship green. Postgres is skipped when AGENTOS_TEST_DATABASE_URL is
// unset; CI sets it and fails the build if these skip.
func backends(t *testing.T, body func(t *testing.T, st Store)) {
	t.Helper()
	t.Run("memory", func(t *testing.T) { body(t, NewMemory()) })
	t.Run("postgres", func(t *testing.T) { body(t, newTestPostgres(t)) })
}

func TestSetUserRole(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		org, err := st.CreateOrg(ctx, "acme", 0)
		if err != nil {
			t.Fatalf("CreateOrg: %v", err)
		}
		user, _, err := st.CreateUser(ctx, org.ID, "a@acme.test", rbac.RoleViewer)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}

		// Promotion and demotion both have to work. Until SetUserRole existed a
		// role could only be chosen at creation, so an identity provider could
		// grant authority via group membership and never take it back.
		for _, want := range []string{rbac.RoleAdmin, rbac.RoleOwner, rbac.RoleMember, rbac.RoleViewer} {
			if err := st.SetUserRole(ctx, user.ID, want); err != nil {
				t.Fatalf("SetUserRole(%s): %v", want, err)
			}
			got, err := st.UserByEmail(ctx, org.ID, "a@acme.test")
			if err != nil {
				t.Fatalf("UserByEmail: %v", err)
			}
			if got.Role != want {
				t.Errorf("role = %q, want %q", got.Role, want)
			}
		}
	})
}

func TestSetUserRoleRejectsUnknownRole(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		ctx := context.Background()
		org, _ := st.CreateOrg(ctx, "acme", 0)
		user, _, err := st.CreateUser(ctx, org.ID, "a@acme.test", rbac.RoleMember)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		// Role names reach this from operator config (AGENTOS_SCIM_GROUP_ROLES),
		// so a typo must be refused rather than stored. A stored bogus role is
		// worse than an error: Can() reports it holds nothing, so the user is
		// silently stripped of access with no failure anywhere.
		for _, bogus := range []string{"wizard", "", "Admin", "owner "} {
			err := st.SetUserRole(ctx, user.ID, bogus)
			if !errors.Is(err, ErrInvalidRole) {
				t.Errorf("SetUserRole(%q) = %v, want ErrInvalidRole", bogus, err)
			}
		}
		got, _ := st.UserByEmail(ctx, org.ID, "a@acme.test")
		if got.Role != rbac.RoleMember {
			t.Errorf("role changed to %q despite every write being rejected", got.Role)
		}
	})
}

func TestSetUserRoleUnknownUser(t *testing.T) {
	backends(t, func(t *testing.T, st Store) {
		err := st.SetUserRole(context.Background(), "usr_nope", rbac.RoleAdmin)
		if !errors.Is(err, ErrUserNotFound) {
			t.Errorf("SetUserRole(unknown) = %v, want ErrUserNotFound", err)
		}
	})
}
