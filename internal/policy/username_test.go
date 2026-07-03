package policy

import (
	"regexp"
	"strings"
	"testing"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

func TestUsernameDeterministic(t *testing.T) {
	target := UserTarget{ProviderName: "default", SurrealNamespace: "smoke", Database: "smoke", Level: api.UserLevelDatabase}
	first := Username("smoke", "smoke-editor", target)
	second := Username("smoke", "smoke-editor", target)
	if first != second {
		t.Fatalf("expected deterministic username, got %q and %q", first, second)
	}
}

func TestUsernameIsSimpleIdentifier(t *testing.T) {
	target := UserTarget{ProviderName: "default", SurrealNamespace: "team.ns", Database: "db-name", Level: api.UserLevelDatabase}
	got := Username("Team Smoke", "Smoke.Editor/Write", target)
	if !regexp.MustCompile(`^[a-z_][a-z0-9_]*$`).MatchString(got) {
		t.Fatalf("username %q is not a simple identifier", got)
	}
	if !strings.HasPrefix(got, "k8s_team_smoke_smoke_editor_write_") {
		t.Fatalf("username %q does not have expected readable prefix", got)
	}
}

func TestNormalizePartCollapsesUnicodeSeparators(t *testing.T) {
	got := normalizePart("Team-Ångström/東京 42")
	want := "team_ngstr_m_42"
	if got != want {
		t.Fatalf("normalizePart() = %q, want %q", got, want)
	}
}

func TestUsernameHashPreventsNormalizationCollision(t *testing.T) {
	target := UserTarget{ProviderName: "default", SurrealNamespace: "smoke", Database: "smoke", Level: api.UserLevelDatabase}
	first := Username("smoke", "smoke-editor", target)
	second := Username("smoke", "smoke_editor", target)
	if first == second {
		t.Fatalf("expected different usernames for normalized collision, both were %q", first)
	}
}

func TestUsernameTargetChangesHash(t *testing.T) {
	base := UserTarget{ProviderName: "default", SurrealNamespace: "smoke", Database: "smoke", Level: api.UserLevelDatabase}
	otherDB := UserTarget{ProviderName: "default", SurrealNamespace: "smoke", Database: "other", Level: api.UserLevelDatabase}
	if Username("smoke", "editor", base) == Username("smoke", "editor", otherDB) {
		t.Fatal("expected database change to affect deterministic hash")
	}
}

func TestUsernameMaxLength(t *testing.T) {
	target := UserTarget{ProviderName: "default", SurrealNamespace: "smoke", Database: "smoke", Level: api.UserLevelDatabase}
	got := Username(strings.Repeat("namespace-", 20), strings.Repeat("credential-", 20), target)
	if len(got) > usernameMaxLength {
		t.Fatalf("username length = %d, want <= %d: %q", len(got), usernameMaxLength, got)
	}
}
