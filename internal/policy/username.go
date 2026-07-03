package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
)

const (
	usernamePrefix    = "k8s"
	usernameHashChars = 8
	usernameMaxLength = 63
)

// UserTarget identifies the SurrealDB scope for a generated user.
type UserTarget struct {
	ProviderName     string
	SurrealNamespace string
	Database         string
	Level            api.UserLevel
}

// Username returns a deterministic SurrealDB-safe username for a Kubernetes credential CR.
//
// The visible prefix is human-readable and normalized. The suffix is a deterministic
// hash of stable identity and target inputs, not randomness. That suffix prevents
// collisions caused by normalization and keeps delete/recreate behavior stable.
func Username(kubernetesNamespace, credentialName string, target UserTarget) string {
	parts := []string{
		usernamePrefix,
		normalizePart(kubernetesNamespace),
		normalizePart(credentialName),
	}

	base := strings.Join(nonEmpty(parts), "_")
	if base == "" || base == usernamePrefix {
		base = usernamePrefix + "_credential"
	}

	hash := stableHash(kubernetesNamespace, credentialName, target.ProviderName, target.SurrealNamespace, target.Database, string(target.Level))
	suffix := "_" + hash[:usernameHashChars]
	maxBase := usernameMaxLength - len(suffix)
	if len(base) > maxBase {
		base = strings.TrimRight(base[:maxBase], "_")
		if base == "" {
			base = usernamePrefix
		}
	}

	return base + suffix
}

func stableHash(values ...string) string {
	h := sha256.New()
	for _, value := range values {
		_, _ = h.Write([]byte(value))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func normalizePart(input string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToLower(input) {
		if isSimpleUsernamePart(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func isSimpleUsernamePart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
