package surreal

import (
	"errors"
	"strings"
	"testing"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

func TestDefineUserQueryDatabase(t *testing.T) {
	query, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke-db"}, "k8s_smoke_editor_abcd1234", "secret", []api.SurrealRole{api.RoleEditor})
	if err != nil {
		t.Fatal(err)
	}
	want := "USE NS smoke DB `smoke-db`;\nDEFINE USER OVERWRITE k8s_smoke_editor_abcd1234 ON DATABASE PASSWORD \"secret\" ROLES EDITOR;"
	if query != want {
		t.Fatalf("query mismatch\ngot:  %s\nwant: %s", query, want)
	}
}

func TestDefineUserQueryNamespace(t *testing.T) {
	query, err := DefineUserQuery(UserTarget{Level: api.UserLevelNamespace, Namespace: "select"}, "writer", "secret", []api.SurrealRole{api.RoleViewer, api.RoleEditor})
	if err != nil {
		t.Fatal(err)
	}
	want := "USE NS `select`;\nDEFINE USER OVERWRITE writer ON NAMESPACE PASSWORD \"secret\" ROLES VIEWER, EDITOR;"
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
	_, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke\nREMOVE USER root", Database: "smoke"}, "writer", "secret", []api.SurrealRole{api.RoleViewer})
	if err == nil {
		t.Fatal("expected malicious namespace to fail")
	}
}

func TestDefineUserQueryEscapesPasswordLiteral(t *testing.T) {
	query, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke"}, "writer", "s\"; REMOVE USER root; --", []api.SurrealRole{api.RoleViewer})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, `PASSWORD "s\"; REMOVE USER root; --"`) {
		t.Fatalf("expected escaped password literal, got %s", query)
	}
	if strings.Contains(query, "PASSWORD $password") {
		t.Fatalf("password parameters are not valid in DEFINE USER PASSWORD clauses: %s", query)
	}
}

func TestDefineUserQueryRejectsUnknownRole(t *testing.T) {
	_, err := DefineUserQuery(UserTarget{Level: api.UserLevelDatabase, Namespace: "smoke", Database: "smoke"}, "writer", "secret", []api.SurrealRole{"ADMIN"})
	if err == nil {
		t.Fatal("expected unknown role error")
	}
}

func TestRedactPasswordError(t *testing.T) {
	err := redactPasswordError(errors.New(`parse error near PASSWORD "s\"ecret" with raw s"ecret too`), `s"ecret`)
	if strings.Contains(err.Error(), `s"ecret`) || strings.Contains(err.Error(), `"s\"ecret"`) {
		t.Fatalf("password was not redacted: %s", err)
	}
	if !strings.Contains(err.Error(), "[redacted]") {
		t.Fatalf("expected redacted marker: %s", err)
	}
}
