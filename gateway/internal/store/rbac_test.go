package store

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/rbac"
)

// storeFactory builds a fresh Store for the RBAC suite. Postgres skips when
// AGENTOS_TEST_DATABASE_URL is unset.
type storeFactory struct {
	name string
	make func(t *testing.T) Store
}

func rbacStores() []storeFactory {
	return []storeFactory{
		{"memory", func(t *testing.T) Store { return NewMemory() }},
		{"postgres", func(t *testing.T) Store { return newTestPostgres(t) }},
	}
}

func TestStoreCreateKeyBackCompatLandsInDefaultOrg(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			// The Phase 1–4 wrapper must still work and attribute to org_default.
			secret, err := st.CreateKey(ctx, "legacy", 10)
			if err != nil {
				t.Fatalf("CreateKey: %v", err)
			}
			key, err := st.Authenticate(ctx, secret)
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if key.OrgID != DefaultOrgID {
				t.Errorf("org = %q, want %q", key.OrgID, DefaultOrgID)
			}
			keys, err := st.Keys(ctx)
			if err != nil {
				t.Fatalf("Keys: %v", err)
			}
			if len(keys) != 1 || keys[0].OrgID != DefaultOrgID {
				t.Errorf("keys = %+v", keys)
			}

			// EnsureKey (bootstrap path) must also land in org_default.
			if err := st.EnsureKey(ctx, "boot", "agos-boot-secret", 5); err != nil {
				t.Fatalf("EnsureKey: %v", err)
			}
			bk, err := st.Authenticate(ctx, "agos-boot-secret")
			if err != nil {
				t.Fatalf("Authenticate boot: %v", err)
			}
			if bk.OrgID != DefaultOrgID {
				t.Errorf("boot org = %q, want %q", bk.OrgID, DefaultOrgID)
			}
		})
	}
}

func TestStoreOrgLifecycleAndSpend(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			org, err := st.CreateOrg(ctx, "acme", 50)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}
			if !strings.HasPrefix(org.ID, "org_") || org.Name != "acme" || org.MonthlyBudgetUSD != 50 {
				t.Errorf("org = %+v", org)
			}
			if org.CreatedAt.IsZero() {
				t.Error("created_at not set")
			}

			got, err := st.Org(ctx, org.ID)
			if err != nil || got.ID != org.ID {
				t.Fatalf("Org = %+v, err %v", got, err)
			}
			if _, err := st.Org(ctx, "org_missing"); !errors.Is(err, ErrOrgNotFound) {
				t.Errorf("Org(missing) err = %v, want ErrOrgNotFound", err)
			}

			// Two keys in the org, spend aggregates across both.
			s1, err := st.CreateKeyIn(ctx, "k1", 100, org.ID, RootCreator)
			if err != nil {
				t.Fatalf("CreateKeyIn k1: %v", err)
			}
			if _, err := st.CreateKeyIn(ctx, "k2", 100, org.ID, "usr_x"); err != nil {
				t.Fatalf("CreateKeyIn k2: %v", err)
			}
			// A key in another org must not count toward this org's spend.
			if _, err := st.CreateKey(ctx, "other", 100); err != nil {
				t.Fatalf("CreateKey other: %v", err)
			}
			if _, err := st.Authenticate(ctx, s1); err != nil {
				t.Fatalf("auth k1: %v", err)
			}
			if err := st.RecordUsage(ctx, Usage{KeyName: "k1", CostUSD: 3, Status: 200}); err != nil {
				t.Fatalf("RecordUsage k1: %v", err)
			}
			if err := st.RecordUsage(ctx, Usage{KeyName: "k2", CostUSD: 4, Status: 200}); err != nil {
				t.Fatalf("RecordUsage k2: %v", err)
			}
			if err := st.RecordUsage(ctx, Usage{KeyName: "other", CostUSD: 9, Status: 200}); err != nil {
				t.Fatalf("RecordUsage other: %v", err)
			}

			spend, err := st.OrgSpend(ctx, org.ID)
			if err != nil {
				t.Fatalf("OrgSpend: %v", err)
			}
			if !almostEqual(spend, 7) {
				t.Errorf("org spend = %v, want 7", spend)
			}

			orgs, err := st.Orgs(ctx)
			if err != nil {
				t.Fatalf("Orgs: %v", err)
			}
			if len(orgs) != 1 || orgs[0].ID != org.ID {
				t.Errorf("orgs = %+v", orgs)
			}
		})
	}
}

func TestStoreUserLifecycle(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			org, err := st.CreateOrg(ctx, "acme", 0)
			if err != nil {
				t.Fatalf("CreateOrg: %v", err)
			}

			user, token, err := st.CreateUser(ctx, org.ID, "a@acme.test", rbac.RoleAdmin)
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			if !strings.HasPrefix(user.ID, "usr_") {
				t.Errorf("user id = %q, want usr_ prefix", user.ID)
			}
			if !strings.HasPrefix(token, "agu-") {
				t.Errorf("token = %q, want agu- prefix", token)
			}
			if user.OrgID != org.ID || user.Role != rbac.RoleAdmin {
				t.Errorf("user = %+v", user)
			}

			// AuthenticateUser round-trips the token.
			au, err := st.AuthenticateUser(ctx, token)
			if err != nil {
				t.Fatalf("AuthenticateUser: %v", err)
			}
			if au.ID != user.ID || au.Role != rbac.RoleAdmin {
				t.Errorf("auth user = %+v", au)
			}
			if _, err := st.AuthenticateUser(ctx, "agu-bogus"); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("AuthenticateUser(bogus) err = %v, want ErrInvalidToken", err)
			}

			// Invalid role rejected.
			if _, _, err := st.CreateUser(ctx, org.ID, "b@acme.test", "wizard"); err == nil {
				t.Error("CreateUser invalid role: want error")
			}
			// Unknown org rejected.
			if _, _, err := st.CreateUser(ctx, "org_missing", "c@acme.test", rbac.RoleMember); !errors.Is(err, ErrOrgNotFound) {
				t.Errorf("CreateUser unknown org err = %v, want ErrOrgNotFound", err)
			}

			users, err := st.Users(ctx, org.ID)
			if err != nil {
				t.Fatalf("Users: %v", err)
			}
			if len(users) != 1 || users[0].ID != user.ID {
				t.Errorf("users = %+v", users)
			}

			// Delete: wrong org is a miss; correct org removes and invalidates token.
			if err := st.DeleteUser(ctx, "org_other", user.ID); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("DeleteUser wrong org err = %v, want ErrUserNotFound", err)
			}
			if err := st.DeleteUser(ctx, org.ID, user.ID); err != nil {
				t.Fatalf("DeleteUser: %v", err)
			}
			if _, err := st.AuthenticateUser(ctx, token); !errors.Is(err, ErrInvalidToken) {
				t.Errorf("AuthenticateUser after delete err = %v, want ErrInvalidToken", err)
			}
			if err := st.DeleteUser(ctx, org.ID, user.ID); !errors.Is(err, ErrUserNotFound) {
				t.Errorf("DeleteUser twice err = %v, want ErrUserNotFound", err)
			}
		})
	}
}

func TestStoreEnsureOrgIdempotent(t *testing.T) {
	for _, sf := range rbacStores() {
		t.Run(sf.name, func(t *testing.T) {
			st := sf.make(t)
			ctx := context.Background()

			if err := st.EnsureOrg(ctx, DefaultOrgID, "default", 0); err != nil {
				t.Fatalf("EnsureOrg: %v", err)
			}
			if err := st.EnsureOrg(ctx, DefaultOrgID, "default", 0); err != nil {
				t.Fatalf("EnsureOrg second: %v", err)
			}
			org, err := st.Org(ctx, DefaultOrgID)
			if err != nil {
				t.Fatalf("Org: %v", err)
			}
			if org.Name != "default" || org.MonthlyBudgetUSD != 0 {
				t.Errorf("org = %+v", org)
			}
			orgs, err := st.Orgs(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(orgs) != 1 {
				t.Errorf("orgs = %d, want 1 (no duplicate)", len(orgs))
			}
		})
	}
}
