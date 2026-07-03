package surreal

import (
	"context"
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	api "github.com/Seamlezz/surrealdb-credential-operator/api/v1alpha1"
	surrealdb "github.com/surrealdb/surrealdb.go"
)

// Admin manages SurrealDB system users.
type Admin interface {
	DefineUser(ctx context.Context, target UserTarget, username, password string, roles []api.SurrealRole) error
	RemoveUser(ctx context.Context, target UserTarget, username string) error
	Ping(ctx context.Context) error
	Close(ctx context.Context) error
}

// AdminClient is a SurrealDB-backed Admin implementation.
type AdminClient struct {
	db *surrealdb.DB
}

// NewAdminClient connects to SurrealDB and signs in with root/admin credentials.
func NewAdminClient(ctx context.Context, endpoint, username, password string) (*AdminClient, error) {
	return NewAdminClientWithTLS(ctx, endpoint, username, password, nil)
}

// NewAdminClientWithTLS connects to SurrealDB and signs in with root/admin credentials.
func NewAdminClientWithTLS(ctx context.Context, endpoint, username, password string, tlsConfig *tls.Config) (*AdminClient, error) {
	db, err := openDB(ctx, endpoint, tlsConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to surrealdb: %w", err)
	}
	client := &AdminClient{db: db}
	if _, err := db.SignIn(ctx, surrealdb.Auth{Username: username, Password: password}); err != nil {
		_ = client.Close(ctx)
		return nil, fmt.Errorf("sign in to surrealdb: %w", err)
	}
	return client, nil
}

func openDB(ctx context.Context, endpoint string, tlsConfig *tls.Config) (*surrealdb.DB, error) {
	u, err := ParseEndpointURL(endpoint)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		return nil, fmt.Errorf("custom TLS config is not currently supported for %s endpoints", u.Scheme)
	}
	return surrealdb.FromEndpointURLString(ctx, u.String())
}

// DefineUser creates or overwrites a SurrealDB system user.
func (c *AdminClient) DefineUser(ctx context.Context, target UserTarget, username, password string, roles []api.SurrealRole) error {
	query, err := DefineUserQuery(target, username, password, roles)
	if err != nil {
		return err
	}
	if err := execQuery(ctx, c.db, query, nil); err != nil {
		return redactPasswordError(err, password)
	}
	return nil
}

// RemoveUser removes a SurrealDB system user if it exists.
func (c *AdminClient) RemoveUser(ctx context.Context, target UserTarget, username string) error {
	query, err := RemoveUserQuery(target, username)
	if err != nil {
		return err
	}
	return execQuery(ctx, c.db, query, nil)
}

// Ping verifies the connection can run a trivial query.
func (c *AdminClient) Ping(ctx context.Context) error {
	return execQuery(ctx, c.db, "RETURN true;", nil)
}

// Close closes the SurrealDB connection.
func (c *AdminClient) Close(ctx context.Context) error {
	if c == nil || c.db == nil {
		return nil
	}
	return c.db.Close(ctx)
}

func redactPasswordError(err error, password string) error {
	if err == nil || password == "" {
		return err
	}
	message := err.Error()
	message = strings.ReplaceAll(message, strconv.Quote(password), "[redacted]")
	message = strings.ReplaceAll(message, password, "[redacted]")
	return fmt.Errorf("%s", message)
}

func execQuery(ctx context.Context, db *surrealdb.DB, query string, vars map[string]any) error {
	results, err := surrealdb.Query[any](ctx, db, query, vars)
	if err != nil {
		return fmt.Errorf("execute surrealql: %w", err)
	}
	for i, result := range *results {
		if result.Error != nil {
			return fmt.Errorf("surrealql statement %d failed: %w", i, result.Error)
		}
	}
	return nil
}
