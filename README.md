# SurrealDB Credential Operator

Kubernetes operator for declaratively managing scoped SurrealDB system-user credentials.

The operator lets platform teams define trusted SurrealDB providers and tenant policies, while application teams request namespace- or database-level credentials without receiving root credentials.

## Status

Early development. The initial API group is `surrealdb.seamlezz.com/v1alpha1`.

## Planned API

- `SurrealDBProvider` — cluster-scoped platform-owned connection/root credential configuration for a SurrealDB instance.
- `SurrealDBTenantPolicy` — namespaced platform-owned policy describing allowed SurrealDB namespaces, databases, and roles.
- `SurrealDBCredential` — namespaced app-owned request that creates one SurrealDB user and one Kubernetes Secret.

## Development

```bash
go test ./...
dagger check
```

## License

Apache-2.0
