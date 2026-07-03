package policy

import (
	"testing"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

func testPolicy() api.SurrealDBTenantPolicySpec {
	return api.SurrealDBTenantPolicySpec{
		ProviderRef:      api.LocalProviderReference{Name: "default"},
		SurrealNamespace: "smoke",
		DatabaseUsers: api.DatabaseUserPolicy{
			AllowedDatabases: []string{"smoke", "smoke-readonly"},
			AllowedRoles:     []api.SurrealRole{api.RoleViewer, api.RoleEditor},
		},
		NamespaceUsers: api.NamespaceUserPolicy{
			Allowed:      true,
			AllowedRoles: []api.SurrealRole{api.RoleViewer},
		},
	}
}

func TestEvaluateAllowsDatabaseRequest(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelDatabase, Database: "smoke", Roles: []api.SurrealRole{api.RoleEditor}})
	if !got.Allowed {
		t.Fatalf("expected allowed, got %s: %s", got.Reason, got.Message)
	}
	if got.Target.SurrealNamespace != "smoke" || got.Target.Database != "smoke" || got.Target.ProviderName != "default" {
		t.Fatalf("unexpected target: %#v", got.Target)
	}
}

func TestEvaluateDeniesDatabaseNotInList(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelDatabase, Database: "other", Roles: []api.SurrealRole{api.RoleViewer}})
	if got.Allowed || got.Reason != "DatabaseDenied" {
		t.Fatalf("expected DatabaseDenied, got %#v", got)
	}
}

func TestEvaluateDeniesDatabaseOwnerUnlessAllowed(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelDatabase, Database: "smoke", Roles: []api.SurrealRole{api.RoleOwner}})
	if got.Allowed || got.Reason != "RoleDenied" {
		t.Fatalf("expected RoleDenied, got %#v", got)
	}
}

func TestEvaluateAllowsOwnerWhenPolicyAllowsIt(t *testing.T) {
	p := testPolicy()
	p.DatabaseUsers.AllowedRoles = append(p.DatabaseUsers.AllowedRoles, api.RoleOwner)
	got := Evaluate(p, CredentialRequest{Level: api.UserLevelDatabase, Database: "smoke", Roles: []api.SurrealRole{api.RoleOwner}})
	if !got.Allowed {
		t.Fatalf("expected owner allowed by explicit policy, got %#v", got)
	}
}

func TestEvaluateAllowsNamespaceRequest(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelNamespace, Roles: []api.SurrealRole{api.RoleViewer}})
	if !got.Allowed {
		t.Fatalf("expected namespace request allowed, got %#v", got)
	}
}

func TestEvaluateDeniesNamespaceWhenDisabled(t *testing.T) {
	p := testPolicy()
	p.NamespaceUsers.Allowed = false
	got := Evaluate(p, CredentialRequest{Level: api.UserLevelNamespace, Roles: []api.SurrealRole{api.RoleViewer}})
	if got.Allowed || got.Reason != "NamespaceUsersDenied" {
		t.Fatalf("expected NamespaceUsersDenied, got %#v", got)
	}
}

func TestEvaluateDeniesNamespaceEditorWhenOnlyViewerAllowed(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelNamespace, Roles: []api.SurrealRole{api.RoleEditor}})
	if got.Allowed || got.Reason != "RoleDenied" {
		t.Fatalf("expected RoleDenied, got %#v", got)
	}
}

func TestEvaluateDeniesNoRoles(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelDatabase, Database: "smoke"})
	if got.Allowed || got.Reason != "NoRoles" {
		t.Fatalf("expected NoRoles, got %#v", got)
	}
}

func TestEvaluateDeniesDatabaseRequired(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelDatabase, Roles: []api.SurrealRole{api.RoleViewer}})
	if got.Allowed || got.Reason != "DatabaseRequired" {
		t.Fatalf("expected DatabaseRequired, got %#v", got)
	}
}

func TestEvaluateDeniesDatabaseForbiddenForNamespaceUser(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevelNamespace, Database: "smoke", Roles: []api.SurrealRole{api.RoleViewer}})
	if got.Allowed || got.Reason != "DatabaseForbidden" {
		t.Fatalf("expected DatabaseForbidden, got %#v", got)
	}
}

func TestEvaluateDeniesInvalidLevel(t *testing.T) {
	got := Evaluate(testPolicy(), CredentialRequest{Level: api.UserLevel("Project"), Roles: []api.SurrealRole{api.RoleViewer}})
	if got.Allowed || got.Reason != "InvalidLevel" {
		t.Fatalf("expected InvalidLevel, got %#v", got)
	}
}
