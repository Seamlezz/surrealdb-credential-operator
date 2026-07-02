package policy

import (
	"fmt"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

// CredentialRequest is the policy-relevant part of a SurrealDBCredential.
type CredentialRequest struct {
	Level    api.UserLevel
	Database string
	Roles    []api.SurrealRole
}

// Evaluation describes the accepted SurrealDB target for a credential request.
type Evaluation struct {
	Allowed bool
	Reason  string
	Message string
	Target  UserTarget
}

// Evaluate checks whether a credential request is allowed by a tenant policy spec.
func Evaluate(policy api.SurrealDBTenantPolicySpec, request CredentialRequest) Evaluation {
	target := UserTarget{
		ProviderName:     policy.ProviderRef.Name,
		SurrealNamespace: policy.SurrealNamespace,
		Database:         request.Database,
		Level:            request.Level,
	}

	if len(request.Roles) == 0 {
		return denied("NoRoles", "at least one role is required", target)
	}

	switch request.Level {
	case api.UserLevelDatabase:
		if request.Database == "" {
			return denied("DatabaseRequired", "database is required for database-level users", target)
		}
		if !containsString(policy.DatabaseUsers.AllowedDatabases, request.Database) {
			return denied("DatabaseDenied", fmt.Sprintf("database %q is not allowed by policy", request.Database), target)
		}
		if missing := missingRoles(policy.DatabaseUsers.AllowedRoles, request.Roles); len(missing) > 0 {
			return denied("RoleDenied", fmt.Sprintf("roles %v are not allowed for database-level users", missing), target)
		}
		return Evaluation{Allowed: true, Reason: "Allowed", Message: "database user request is allowed", Target: target}
	case api.UserLevelNamespace:
		if !policy.NamespaceUsers.Allowed {
			return denied("NamespaceUsersDenied", "namespace-level users are not allowed by policy", target)
		}
		if request.Database != "" {
			return denied("DatabaseForbidden", "database must be empty for namespace-level users", target)
		}
		if missing := missingRoles(policy.NamespaceUsers.AllowedRoles, request.Roles); len(missing) > 0 {
			return denied("RoleDenied", fmt.Sprintf("roles %v are not allowed for namespace-level users", missing), target)
		}
		return Evaluation{Allowed: true, Reason: "Allowed", Message: "namespace user request is allowed", Target: target}
	default:
		return denied("InvalidLevel", fmt.Sprintf("unsupported user level %q", request.Level), target)
	}
}

func denied(reason, message string, target UserTarget) Evaluation {
	return Evaluation{Allowed: false, Reason: reason, Message: message, Target: target}
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func missingRoles(allowed, requested []api.SurrealRole) []api.SurrealRole {
	allowedSet := make(map[api.SurrealRole]struct{}, len(allowed))
	for _, role := range allowed {
		allowedSet[role] = struct{}{}
	}
	var missing []api.SurrealRole
	for _, role := range requested {
		if _, ok := allowedSet[role]; !ok {
			missing = append(missing, role)
		}
	}
	return missing
}
