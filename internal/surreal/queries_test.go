package surreal

import (
	"strings"
	"testing"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

func TestDefineUserQueryDatabase(t *testing.T) {
	query, vars, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke-db"}, "k8s_smoke_editor_abcd1234", []api.SurrealRole{api.RoleEditor})
	if err != nil {
		t.Fatal(err)
	}
	want := "USE NS smoke DB `smoke-db`;\nDEFINE USER OVERWRITE k8s_smoke_editor_abcd1234 ON DATABASE PASSWORD $password ROLES EDITOR;"
	if query != want {
		t.Fatalf("query mismatch\ngot:  %s\nwant: %s", query, want)
	}
	if _, ok := vars["password"]; !ok {
		t.Fatal("expected password variable")
	}
}

func TestDefineUserQueryNamespace(t *testing.T) {
	query, _, err := DefineUserQuery(UserTarget{Level: api.UserLevelNamespace, Namespace: "select"}, "writer", []api.SurrealRole{api.RoleViewer, api.RoleEditor})
	if err != nil {
		t.Fatal(err)
	}
	want := "USE NS `select`;\nDEFINE USER OVERWRITE writer ON NAMESPACE PASSWORD $password ROLES VIEWER, EDITOR;"
	if query != want {
		t.Fatalf("query mismatch\ngot:  %s\nwant: %s", query, want)
	}
}

func TestRemoveUserQuery(t *testing.T) {
	query, err := RemoveUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke"}, "writer")
	if err != nil {
		t.Fatal(err)
	}
	want := "USE NS smoke DB smoke;\nREMOVE USER IF EXISTS writer ON DATABASE;"
	if query != want {
		t.Fatalf("got %q, want %q", query, want)
	}
}

func TestDefineUserQueryRejectsMaliciousControlInput(t *testing.T) {
	_, _, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke\nREMOVE USER root", Database: "smoke"}, "writer", []api.SurrealRole{api.RoleViewer})
	if err == nil {
		t.Fatal("expected malicious namespace to fail")
	}
}

func TestDefineUserQueryDoesNotInterpolatePassword(t *testing.T) {
	query, _, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke"}, "writer", []api.SurrealRole{api.RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "PASSWORD $password") {
		t.Fatalf("expected password parameter, got %s", query)
	}
}

func TestDefineUserQueryRejectsUnknownRole(t *testing.T) {
	_, _, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke"}, "writer", []api.SurrealRole{"ADMIN"})
	if err == nil {
		t.Fatal("expected unknown role error")
	}
}
