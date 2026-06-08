# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `cassandra.createKeyspace` Helm value / `URL_SHORTENER_CREATE_KEYSPACE` env var: when `true`, the app creates the keyspace and tables on startup (requires `CREATE ON ALL KEYSPACES`); defaults to `false` — assumes keyspace and tables are pre-provisioned
- `cassandra.replicationStrategy` / `CASSANDRA_REPLICATION_STRATEGY`: replication strategy used when `createKeyspace` is enabled (default `SimpleStrategy`; use `NetworkTopologyStrategy` for multi-DC production clusters)
- `cassandra.replicationFactor` / `CASSANDRA_REPLICATION_FACTOR`: replication factor used when `createKeyspace` is enabled (default `1`; set to `3` or higher for production)
- Helm chart at `charts/url-shortener` for Kubernetes deployment
- Helm chart published to `ghcr.io/digitalis-io/helm-charts` via OCI on every release
- GitHub Actions `helm-lint-test` job: runs `ct lint`, `helm template | kubectl apply --dry-run`, and `helm unittest` on every push and pull request
- GitHub Actions `publish-chart` job: packages and pushes the chart to GHCR OCI on release publish, gated on `test` and `helm-lint-test`
- `values.schema.json` with JSON Schema validation for required fields (`app.publicBaseURL`, `app.adminBaseURL`, `cassandra.hosts`)
- `existingSecret` support for session, Cassandra, and SAML credential groups to integrate with external-secrets or Vault
- Cassandra TLS support: inline PEM content (`cassandra.sslCA/sslCert/sslKey`) rendered into a Secret and volume-mounted at `/etc/cassandra-ssl/`; or `cassandra.existingSSLSecret` to reference an existing Secret
- vals-operator support: `cassandra.valsSecret` and `cassandra.valsSSLSecret` blocks render `ValsSecret` (digitalis.io/v1) resources so vals-operator manages the Cassandra auth and SSL Secrets from external stores (Vault, AWS SSM, etc.)

[Unreleased]: https://github.com/digitalis-io/url-shortener/compare/HEAD...HEAD
