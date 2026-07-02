package surreal

import (
	"fmt"
	"strings"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

// UserTarget identifies the SurrealDB scope where a system user is managed.
type UserTarget struct {
	Level     api.UserLevel
	Namespace string
	Database  string
}

// DefineUserQuery builds a SurrealQL query that defines or overwrites a system user.
func DefineUserQuery(target UserTarget, username string, roles []api.SurrealRole) (string, map[string]any, error) {
	prefix, err := usePrefix(target)
	if err != nil {
		return "", nil, err
	}
	userIdent, err := EscapeIdent(username)
	if err != nil {
		return "", nil, fmt.Errorf("escape username: %w", err)
	}
	roleList, err := renderRoles(roles)
	if err != nil {
		return "", nil, err
	}

	on := "DATABASE"
	if target.Level == api.UserLevelNamespace {
		on = "NAMESPACE"
	}
	query := fmt.Sprintf("%s\nDEFINE USER OVERWRITE %s ON %s PASSWORD $password ROLES %s;", prefix, userIdent, on, roleList)
	return query, map[string]any{"password": nil}, nil
}

// RemoveUserQuery builds a SurrealQL query that removes a system user if it exists.
func RemoveUserQuery(target UserTarget, username string) (string, error) {
	prefix, err := usePrefix(target)
	if err != nil {
		return "", err
	}
	userIdent, err := EscapeIdent(username)
	if err != nil {
		return "", fmt.Errorf("escape username: %w", err)
	}
	on := "DATABASE"
	if target.Level == api.UserLevelNamespace {
		on = "NAMESPACE"
	}
	return fmt.Sprintf("%s\nREMOVE USER IF EXISTS %s ON %s;", prefix, userIdent, on), nil
}

func usePrefix(target UserTarget) (string, error) {
	ns, err := EscapeIdent(target.Namespace)
	if err != nil {
		return "", fmt.Errorf("escape namespace: %w", err)
	}
	switch target.Level {
	case api.UserLevelNamespace:
		return fmt.Sprintf("USE NS %s;", ns), nil
	case api.UserLevelDatabase:
		if target.Database == "" {
			return "", fmt.Errorf("database is required for database-level user")
		}
		db, err := EscapeIdent(target.Database)
		if err != nil {
			return "", fmt.Errorf("escape database: %w", err)
		}
		return fmt.Sprintf("USE NS %s DB %s;", ns, db), nil
	default:
		return "", fmt.Errorf("unsupported user level %q", target.Level)
	}
}

func renderRoles(roles []api.SurrealRole) (string, error) {
	if len(roles) == 0 {
		return "", fmt.Errorf("at least one role is required")
	}
	out := make([]string, 0, len(roles))
	seen := map[api.SurrealRole]struct{}{}
	for _, role := range roles {
		if _, ok := seen[role]; ok {
			continue
		}
		switch role {
		case api.RoleViewer, api.RoleEditor, api.RoleOwner:
			out = append(out, string(role))
			seen[role] = struct{}{}
		default:
			return "", fmt.Errorf("unsupported role %q", role)
		}
	}
	return strings.Join(out, ", "), nil
}
