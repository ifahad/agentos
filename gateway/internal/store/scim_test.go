package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

// TestUserActiveDefaultsTrue verifies a freshly created user is active and its
// token authenticates, for both store backends.
func TestUserActiveDefaultsTrue(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()
			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			user, token, err := st.CreateUser(ctx, org.ID, "u@acme.test", rbac.RoleMember)
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			if !user.Active {
				t.Errorf("new user Active = false, want true")
			}
			if user.ExternalID != "" {
				t.Errorf("new user ExternalID = %q, want empty", user.ExternalID)
			}
			au, err := st.AuthenticateUser(ctx, token)
			if err != nil || !au.Active {
				t.Fatalf("AuthenticateUser = %+v, %v", au, err)
			}
		})
	}
}

// TestSetUserActiveInvalidatesToken verifies deactivation makes AuthenticateUser
// fail with ErrUserInactive while the user is retained and reactivatable.
func TestSetUserActiveInvalidatesToken(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()
			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			user, token, err := st.CreateUser(ctx, org.ID, "u@acme.test", rbac.RoleMember)
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}

			// Deactivate → token stops working with ErrUserInactive.
			if err := st.SetUserActive(ctx, user.ID, false); err != nil {
				t.Fatalf("SetUserActive(false): %v", err)
			}
			if _, err := st.AuthenticateUser(ctx, token); !errors.Is(err, ErrUserInactive) {
				t.Errorf("AuthenticateUser(inactive) err = %v, want ErrUserInactive", err)
			}

			// Still listed (retained), with Active=false.
			users, err := st.Users(ctx, org.ID)
			if err != nil || len(users) != 1 || users[0].Active {
				t.Fatalf("Users after deactivate = %+v (err %v)", users, err)
			}

			// Reactivate → token works again.
			if err := st.SetUserActive(ctx, user.ID, true); err != nil {
				t.Fatalf("SetUserActive(true): %v", err)
			}
			if au, err := st.AuthenticateUser(ctx, token); err != nil || au.ID != user.ID {
				t.Errorf("AuthenticateUser(reactivated) = %+v, %v", au, err)
			}

			// Unknown id is a miss.
			if err := st.SetUserActive(ctx, "usr_missing", false); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("SetUserActive(missing) err = %v, want ErrUserNotFound", err)
			}
		})
	}
}

// TestExternalIDLifecycle verifies create-with-external-id, lookup, set, and
// clear across both backends.
func TestExternalIDLifecycle(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()
			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			user, _, err := st.CreateUserWithExternalID(ctx, org.ID, "u@acme.test", rbac.RoleMember, "ext-123")
			if err != nil {
				t.Fatalf("CreateUserWithExternalID: %v", err)
			}
			if user.ExternalID != "ext-123" {
				t.Errorf("ExternalID = %q, want ext-123", user.ExternalID)
			}

			found, err := st.UserByExternalID(ctx, org.ID, "ext-123")
			if err != nil || found.ID != user.ID {
				t.Fatalf("UserByExternalID = %+v, %v", found, err)
			}

			// Empty external id never matches.
			if _, err := st.UserByExternalID(ctx, org.ID, ""); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("UserByExternalID(\"\") err = %v, want ErrUserNotFound", err)
			}
			// Wrong org is a miss.
			if _, err := st.UserByExternalID(ctx, "org_other", "ext-123"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("UserByExternalID(wrong org) err = %v, want ErrUserNotFound", err)
			}

			// Re-point the external id.
			if err := st.SetUserExternalID(ctx, user.ID, "ext-999"); err != nil {
				t.Fatalf("SetUserExternalID: %v", err)
			}
			if found, err := st.UserByExternalID(ctx, org.ID, "ext-999"); err != nil || found.ID != user.ID {
				t.Errorf("UserByExternalID(ext-999) = %+v, %v", found, err)
			}
			if _, err := st.UserByExternalID(ctx, org.ID, "ext-123"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("old external id still resolves; err = %v, want ErrUserNotFound", err)
			}

			// Clear it.
			if err := st.SetUserExternalID(ctx, user.ID, ""); err != nil {
				t.Fatalf("SetUserExternalID(clear): %v", err)
			}
			if _, err := st.UserByExternalID(ctx, org.ID, "ext-999"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("cleared external id still resolves; err = %v", err)
			}

			// Unknown id.
			if err := st.SetUserExternalID(ctx, "usr_missing", "x"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("SetUserExternalID(missing) err = %v, want ErrUserNotFound", err)
			}
		})
	}
}
