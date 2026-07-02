package surreal

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var simpleIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var reservedIdentifiers = map[string]struct{}{
	"access": {}, "alter": {}, "analyzer": {}, "api": {}, "as": {},
	"database": {}, "db": {}, "define": {}, "delete": {}, "duration": {},
	"editor": {}, "exists": {}, "field": {}, "for": {}, "from": {},
	"if": {}, "index": {}, "namespace": {}, "none": {}, "ns": {},
	"on": {}, "owner": {}, "param": {}, "password": {}, "passhash": {},
	"record": {}, "remove": {}, "roles": {}, "select": {}, "table": {},
	"user": {}, "use": {}, "viewer": {}, "where": {},
}

// EscapeIdent returns input as a SurrealQL identifier.
//
// Simple non-reserved identifiers are returned unchanged. Other identifiers are
// wrapped in backticks with embedded backticks escaped. Empty strings and control
// characters are rejected.
func EscapeIdent(input string) (string, error) {
	if input == "" {
		return "", fmt.Errorf("identifier must not be empty")
	}
	for _, r := range input {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("identifier %q contains control character", input)
		}
	}
	if simpleIdentPattern.MatchString(input) {
		if _, reserved := reservedIdentifiers[strings.ToLower(input)]; !reserved {
			return input, nil
		}
	}
	return "`" + strings.ReplaceAll(input, "`", "\\`") + "`", nil
}
