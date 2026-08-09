package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/ifahad/agentos/gateway/internal/provider"
	"github.com/ifahad/agentos/gateway/internal/rbac"
	"github.com/ifahad/agentos/gateway/internal/store"
)

// newSCIMGatewayWithGroupRoles is newSCIMGateway plus an operator allowlist.
func newSCIMGatewayWithGroupRoles(t *testing.T, mapping map[string]string) (string, *store.Memory) {
	t.Helper()
	fake := newFakeProvider(t)
	mem := store.NewMemory()
	if err := mem.EnsureOrg(context.Background(), store.DefaultOrgID, "default", 0); err != nil {
		t.Fatalf("EnsureOrg: %v", err)
	}
	router := &provider.Router{
		AnthropicBaseURL: fake.server.URL, OpenAIBaseURL: fake.server.URL, OllamaBaseURL: fake.server.URL,
		AnthropicAPIKey: "k", OpenAIAPIKey: "k",
	}
	ts := newHTTPServer(t, New(mem, router, testAdminKey,
		WithSCIM(testSCIMToken, "", ""),
		WithSCIMGroupRoles(mapping)).Handler())
	return ts, mem
}

func roleOf(t *testing.T, mem *store.Memory, id string) string {
	t.Helper()
	users, err := mem.Users(context.Background(), store.DefaultOrgID)
	if err != nil {
		t.Fatalf("Users: %v", err)
	}
	for i := range users {
		if users[i].ID == id {
			return users[i].Role
		}
	}
	t.Fatalf("user %s not found", id)
	return ""
}

func TestSCIMGroupRolesGrantAndRevoke(t *testing.T) {
	ts, mem := newSCIMGatewayWithGroupRoles(t, map[string]string{"AgentOS Admins": rbac.RoleAdmin})
	alice := createSCIMUser(t, ts, "alice@corp.test")
	if got := roleOf(t, mem, alice); got != rbac.RoleMember {
		t.Fatalf("new user role = %q, want the SCIM default %q", got, rbac.RoleMember)
	}
	gid := createSCIMGroup(t, ts, "AgentOS Admins")

	// Grant.
	resp, body := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+alice+`"}]}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("add status = %d (%s)", resp.StatusCode, body)
	}
	if got := roleOf(t, mem, alice); got != rbac.RoleAdmin {
		t.Errorf("after joining the mapped group, role = %q, want admin", got)
	}

	// Revoke. This is the half a grant-only implementation forgets: without it
	// an identity provider can hand out authority and never take it back.
	resp, body = doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"remove","path":"members[value eq \"`+alice+`\"]"}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remove status = %d (%s)", resp.StatusCode, body)
	}
	if got := roleOf(t, mem, alice); got != rbac.RoleMember {
		t.Errorf("after leaving her only mapped group, role = %q, want the default %q", got, rbac.RoleMember)
	}
}

func TestSCIMGroupRolesStrongestWins(t *testing.T) {
	ts, mem := newSCIMGatewayWithGroupRoles(t, map[string]string{
		"Admins":  rbac.RoleAdmin,
		"Viewers": rbac.RoleViewer,
	})
	alice := createSCIMUser(t, ts, "alice@corp.test")
	admins := createSCIMGroup(t, ts, "Admins")
	viewers := createSCIMGroup(t, ts, "Viewers")

	for _, gid := range []string{viewers, admins} {
		doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
			`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+alice+`"}]}]}`)
	}
	// In both groups: the strongest role wins, deterministically, regardless of
	// the order the identity provider happened to sync them in.
	if got := roleOf(t, mem, alice); got != rbac.RoleAdmin {
		t.Errorf("in Admins and Viewers, role = %q, want admin", got)
	}

	// Losing the stronger group falls back to the weaker one, not to the default.
	doRawBytes(t, http.MethodDelete, ts+"/scim/v2/Groups/"+admins, testSCIMToken, "")
	if got := roleOf(t, mem, alice); got != rbac.RoleViewer {
		t.Errorf("after the admin group is deleted, role = %q, want viewer", got)
	}
}

func TestSCIMGroupRolesIgnoreUnmappedGroups(t *testing.T) {
	ts, mem := newSCIMGatewayWithGroupRoles(t, map[string]string{"Admins": rbac.RoleAdmin})
	alice := createSCIMUser(t, ts, "alice@corp.test")
	// Identity providers push every group an admin scopes. A group nobody
	// mapped must grant nothing rather than being rejected or guessed at.
	gid := createSCIMGroup(t, ts, "All Company")
	resp, _ := doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+alice+`"}]}]}`)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("an unmapped group must still accept members: %d", resp.StatusCode)
	}
	if got := roleOf(t, mem, alice); got != rbac.RoleMember {
		t.Errorf("unmapped group changed the role to %q", got)
	}
}

func TestSCIMGroupRolesOffByDefault(t *testing.T) {
	// No allowlist configured: this is every existing deployment, and the
	// security assessment's "SCIM cannot escalate role" must stay true of it.
	ts, mem := newSCIMGateway(t)
	alice := createSCIMUser(t, ts, "alice@corp.test")
	gid := createSCIMGroup(t, ts, "Admins")
	doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+alice+`"}]}]}`)
	if got := roleOf(t, mem, alice); got != rbac.RoleMember {
		t.Errorf("with no mapping configured, role changed to %q", got)
	}
}

func TestSCIMGroupRolesLeaveHandCreatedUsersAlone(t *testing.T) {
	ts, mem := newSCIMGatewayWithGroupRoles(t, map[string]string{"Admins": rbac.RoleAdmin})
	ctx := context.Background()
	// Created through the admin path, so no ExternalID: not SCIM-provisioned.
	owner, _, err := mem.CreateUser(ctx, store.DefaultOrgID, "owner@corp.test", rbac.RoleOwner)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	gid := createSCIMGroup(t, ts, "Admins")

	doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+owner.ID+`"}]}]}`)
	if got := roleOf(t, mem, owner.ID); got != rbac.RoleOwner {
		t.Errorf("a hand-created owner was changed to %q by joining a group", got)
	}
	// And removal must not demote them either — that is the case the ExternalID
	// guard exists for.
	doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"remove","path":"members"}]}`)
	if got := roleOf(t, mem, owner.ID); got != rbac.RoleOwner {
		t.Errorf("a hand-created owner was demoted to %q by leaving a group", got)
	}
}

func TestSCIMGroupRoleMappingIsCaseInsensitive(t *testing.T) {
	ts, mem := newSCIMGatewayWithGroupRoles(t, map[string]string{"agentos-admins": rbac.RoleAdmin})
	alice := createSCIMUser(t, ts, "alice@corp.test")
	// The IdP's casing need not match what the operator typed; if it had to,
	// a re-sync that changed case would silently drop everyone's role.
	gid := createSCIMGroup(t, ts, "AgentOS-Admins")
	doRawBytes(t, http.MethodPatch, ts+"/scim/v2/Groups/"+gid, testSCIMToken,
		`{"Operations":[{"op":"add","path":"members","value":[{"value":"`+alice+`"}]}]}`)
	if got := roleOf(t, mem, alice); got != rbac.RoleAdmin {
		t.Errorf("case-differing group name granted %q, want admin", got)
	}
}
