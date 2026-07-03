package surreal

import (
	"strings"
	"testing"
)

func TestValidateEndpointScheme(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		wantErr   bool
		wantError string
	}{
		{name: "ws", endpoint: "ws://surrealdb:8000"},
		{name: "wss", endpoint: "wss://surrealdb.example.com/rpc"},
		{name: "http", endpoint: "http://surrealdb:8000", wantErr: true, wantError: "use ws:// or wss:// endpoints"},
		{name: "https", endpoint: "https://surrealdb:8000", wantErr: true, wantError: "use ws:// or wss:// endpoints"},
		{name: "unsupported scheme", endpoint: "tcp://surrealdb:8000", wantErr: true, wantError: "use ws:// or wss:// endpoints"},
		{name: "missing scheme", endpoint: "surrealdb:8000", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpointScheme(tt.endpoint)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantError != "" && !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error to contain %q, got %q", tt.wantError, err.Error())
			}
		})
	}
}
