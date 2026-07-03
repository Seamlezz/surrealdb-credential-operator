package surreal

import (
	"fmt"
	"net/url"
)

// ParseEndpointURL parses and validates a SurrealDB provider endpoint URL.
func ParseEndpointURL(endpoint string) (*url.URL, error) {
	u, err := url.ParseRequestURI(endpoint)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "ws", "wss":
		return u, nil
	case "http", "https":
		return nil, unsupportedEndpointSchemeError(u.Scheme)
	default:
		return nil, unsupportedEndpointSchemeError(u.Scheme)
	}
}

// ValidateEndpointScheme validates that a SurrealDB provider endpoint uses a supported scheme.
func ValidateEndpointScheme(endpoint string) error {
	_, err := ParseEndpointURL(endpoint)
	return err
}

func unsupportedEndpointSchemeError(scheme string) error {
	return fmt.Errorf("unsupported SurrealDB provider endpoint scheme %q: use ws:// or wss:// endpoints", scheme)
}
