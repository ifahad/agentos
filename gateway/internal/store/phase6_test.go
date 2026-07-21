package store

import (
	"context"
	"errors"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

func TestStoreUpdateOrg(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			if org.RateLimitRPM != 0 {
				t.Errorf("new org rpm = %d, want 0", org.RateLimitRPM)
			}

			// Patch rpm only; budget unchanged.
			rpm := 60
			got, err := st.UpdateOrg(ctx, org.ID, nil, &rpm)
			if err != nil {
				t.Fatalf("UpdateOrg rpm: %v", err)
			}
			if got.RateLimitRPM != 60 || got.MonthlyBudgetUSD != 0 {
				t.Errorf("after rpm patch = %+v", got)
			}

			// Patch budget only; rpm unchanged.
			budget := 25.0
			got, err = st.UpdateOrg(ctx, org.ID, &budget, nil)
			if err != nil {
				t.Fatalf("UpdateOrg budget: %v", err)
			}
			if got.RateLimitRPM != 60 || got.MonthlyBudgetUSD != 25 {
				t.Errorf("after budget patch = %+v", got)
			}

			// Persisted view reflects both.
			reread, err := st.Org(ctx, org.ID)
			if err != nil {
				t.Fatalf("Org: %v", err)
			}
			if reread.RateLimitRPM != 60 || reread.MonthlyBudgetUSD != 25 {
				t.Errorf("reread = %+v", reread)
			}

			// Unknown org.
			if _, err := st.UpdateOrg(ctx, "org_missing", &budget, &rpm); !errors.Is(err, ErrOrgNotFound) {
				t.Errorf("UpdateOrg missing err = %v, want ErrOrgNotFound", err)
			}
		})
	}
}

func TestStoreUserByEmailAndIssueToken(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			user, token1, err := st.CreateUser(ctx, org.ID, "u@acme.test", rbac.RoleMember)
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}

			// UserByEmail finds the user.
			found, err := st.UserByEmail(ctx, org.ID, "u@acme.test")
			if err != nil {
				t.Fatalf("UserByEmail: %v", err)
			}
			if found.ID != user.ID {
				t.Errorf("UserByEmail id = %q, want %q", found.ID, user.ID)
			}

			// Missing email / wrong org.
			if _, err := st.UserByEmail(ctx, org.ID, "nobody@acme.test"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("UserByEmail missing err = %v, want ErrUserNotFound", err)
			}
			if _, err := st.UserByEmail(ctx, "org_other", "u@acme.test"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("UserByEmail wrong org err = %v, want ErrUserNotFound", err)
			}

			// IssueUserToken mints a working new token and invalidates the old.
			token2, err := st.IssueUserToken(ctx, user.ID)
			if err != nil {
				t.Fatalf("IssueUserToken: %v", err)
			}
			if token2 == token1 || len(token2) < 5 || token2[:4] != "agu-" {
				t.Errorf("issued token = %q, want fresh agu- token", token2)
			}
			if u, err := st.AuthenticateUser(ctx, token2); err != nil || u.ID != user.ID {
				t.Errorf("AuthenticateUser(new) = %+v, %v", u, err)
			}
			if _, err := st.AuthenticateUser(ctx, token1); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("old token still valid; err = %v, want ErrInvalidToken", err)
			}

			// Unknown user id.
			if _, err := st.IssueUserToken(ctx, "usr_missing"); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("IssueUserToken missing err = %v, want ErrUserNotFound", err)
			}
		})
	}
}
