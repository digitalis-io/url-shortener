<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis.io/wp-content/uploads/2020/06/digitalis-logo.png" alt="Digitalis.IO" width="300">
  </a>
</p>

# url-shortener

A Go URL shortener backed by Cassandra.

## Current Features

- Public short URL redirects with `302 Found`.
- Authenticated admin-domain create API and browser UI.
- Recent URL list with creator attribution.
- Hourly hit counters per short URL.
- Cassandra schema bootstrap using `github.com/apache/cassandra-gocql-driver/v2`.
- Host-aware public/admin routing.
- Prometheus metrics at `/metrics`.

SAML support uses `github.com/crewjam/saml` and can be configured with Azure Entra ID metadata plus a service-provider certificate/key pair. `AUTH_DEV_BYPASS=true` is available for local development only.

## Local Development

Start Cassandra:

```bash
docker compose up cassandra
```

Run the service with explicit local admin auth bypass:

```bash
AUTH_DEV_BYPASS=true \
SESSION_SECRET=local-dev-secret \
PUBLIC_BASE_URL=http://localhost:8080 \
ADMIN_BASE_URL=http://localhost:8080 \
go run ./cmd/url-shortener
```

Open the admin UI:

```text
http://localhost:8080
```

Create a short URL from the UI, then use the generated public short URL to test redirects.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `HTTP_ADDR` | `:8080` | Listen address |
| `PUBLIC_BASE_URL` | — | **Required.** Base URL for generated short URLs |
| `ADMIN_BASE_URL` | `PUBLIC_BASE_URL` | Base URL for admin UI and SAML routes |
| `CASSANDRA_HOSTS` | `localhost:9042` | Comma-separated `host:port` list |
| `CASSANDRA_KEYSPACE` | `url_shortener` | Cassandra keyspace |
| `CASSANDRA_USERNAME` | `""` | Cassandra username |
| `CASSANDRA_PASSWORD` | `""` | Cassandra password |
| `URL_SHORTENER_CREATE_KEYSPACE` | `false` | When `true`, create the keyspace and tables on startup. Requires `CREATE ON ALL KEYSPACES`. When `false` (default), assumes keyspace and tables are pre-provisioned — only `SELECT`, `MODIFY`, `ALTER` on the keyspace are needed. |
| `CASSANDRA_REPLICATION_STRATEGY` | `SimpleStrategy` | Replication strategy used when `URL_SHORTENER_CREATE_KEYSPACE=true`. Use `NetworkTopologyStrategy` for multi-DC production clusters. |
| `CASSANDRA_REPLICATION_FACTOR` | `1` | Replication factor used when `URL_SHORTENER_CREATE_KEYSPACE=true`. Set to `3` or higher for production. |
| `CASSANDRA_SSL_ENABLED` | `false` | Enable TLS |
| `CASSANDRA_SSL_CA_FILE` | `""` | CA certificate path |
| `CASSANDRA_SSL_SERVER_NAME` | `""` | TLS server name override (SNI) |
| `CASSANDRA_SSL_INSECURE_SKIP_VERIFY` | `false` | Skip server cert verification — dev only |
| `CASSANDRA_SSL_CERT_FILE` | `""` | Client certificate path (mTLS) |
| `CASSANDRA_SSL_KEY_FILE` | `""` | Client private key path (mTLS) |
| `CASSANDRA_CONSISTENCY` | `LOCAL_QUORUM` | Read/write consistency level |
| `CASSANDRA_SERIAL_CONSISTENCY` | `LOCAL_SERIAL` | LWT serial consistency level |
| `CASSANDRA_LOCAL_DC` | `""` | Local DC for token-aware routing (required with `LOCAL_*` levels in multi-DC) |
| `CASSANDRA_PROTO_VERSION` | `4` | CQL native protocol version |
| `CASSANDRA_NUM_CONNS` | `2` | Connections per host |
| `CASSANDRA_PAGE_SIZE` | `5000` | Query page size |
| `CASSANDRA_CONNECT_TIMEOUT` | `10s` | Connection timeout |
| `CASSANDRA_TIMEOUT` | `10s` | Query timeout |
| `CASSANDRA_WRITE_TIMEOUT` | `""` | Write timeout (defaults to `CASSANDRA_TIMEOUT`) |
| `CASSANDRA_RECONNECT_INTERVAL` | `60s` | Reconnect interval after host failure |
| `CASSANDRA_SOCKET_KEEPALIVE` | `15s` | TCP keepalive interval |
| `CASSANDRA_MAX_WAIT_SCHEMA_AGREEMENT` | `60s` | Max wait for schema agreement |
| `CASSANDRA_RETRY_ATTEMPTS` | `3` | Idempotent retry attempts (`0` = disabled) |
| `CASSANDRA_RETRY_MIN_BACKOFF` | `100ms` | Retry backoff minimum |
| `CASSANDRA_RETRY_MAX_BACKOFF` | `2s` | Retry backoff maximum |
| `CASSANDRA_SPECULATIVE_EXECUTION_ENABLED` | `false` | Enable speculative reads |
| `CASSANDRA_SPECULATIVE_ATTEMPTS` | `1` | Speculative attempt count |
| `CASSANDRA_SPECULATIVE_DELAY` | `50ms` | Delay before issuing speculative retry |
| `SESSION_SECRET` | — | **Required.** Session signing secret |
| `AUTH_DEV_BYPASS` | `false` | Skip SAML auth — local dev only |
| `AUTH_HEADER_ENABLED` | `false` | Accept user identity from a trusted header (e.g. Cloudflare Access) instead of SAML |
| `AUTH_USER_EMAIL_HEADER` | `Cf-Access-Authenticated-User-Email` | Header name used when `AUTH_HEADER_ENABLED=true` |
| `SAML_ENTITY_ID` | `ADMIN_BASE_URL/saml/metadata` | SAML entity ID |
| `SAML_ACS_URL` | `ADMIN_BASE_URL/saml/acs` | SAML ACS URL |
| `SAML_IDP_METADATA_URL` | `""` | IdP metadata URL (fetched at startup) |
| `SAML_IDP_METADATA` | `""` | Inline IdP metadata XML |
| `SAML_PRIVATE_KEY` | `""` | SP private key PEM |
| `SAML_CERTIFICATE` | `""` | SP certificate PEM |
| `CODE_LENGTH` | `7` | Short code length |

`PUBLIC_BASE_URL` is used in generated short URLs. `ADMIN_BASE_URL` is used for the admin UI, SAML routes, and admin API calls.

`CASSANDRA_CONSISTENCY` sets the consistency level for every read and write (default `LOCAL_QUORUM`); `CASSANDRA_SERIAL_CONSISTENCY` controls the serial level used by lightweight transactions such as the `IF NOT EXISTS` guard on short-code creation (default `LOCAL_SERIAL`). Both accept any gocql level (`ONE`, `QUORUM`, `EACH_QUORUM`, `LOCAL_ONE`, …) and an invalid value fails fast at startup. Set `CASSANDRA_LOCAL_DC` to your datacenter name so the token-aware policy keeps coordinator selection DC-local — important whenever you use a `LOCAL_*` consistency level in a multi-DC cluster. Failed idempotent reads and deletes are retried with exponential backoff (`CASSANDRA_RETRY_ATTEMPTS`, `CASSANDRA_RETRY_MIN_BACKOFF`, `CASSANDRA_RETRY_MAX_BACKOFF`; set attempts to `0` to disable). Counter increments for click tracking are never retried because they are not idempotent. Enable `CASSANDRA_SPECULATIVE_EXECUTION_ENABLED=true` to issue redundant idempotent reads after `CASSANDRA_SPECULATIVE_DELAY` to trim tail latency. The remaining knobs (`CASSANDRA_PROTO_VERSION`, `CASSANDRA_NUM_CONNS`, `CASSANDRA_PAGE_SIZE`, `CASSANDRA_CONNECT_TIMEOUT`, `CASSANDRA_TIMEOUT`, `CASSANDRA_WRITE_TIMEOUT`, `CASSANDRA_RECONNECT_INTERVAL`, `CASSANDRA_SOCKET_KEEPALIVE`, `CASSANDRA_MAX_WAIT_SCHEMA_AGREEMENT`) tune the connection pool and timeouts; leave them unset to use the defaults shown above.

For Cassandra clusters that require TLS, set `CASSANDRA_SSL_ENABLED=true`. By default TLS verifies the server certificate and host name. Use `CASSANDRA_SSL_CA_FILE` to trust a private CA, `CASSANDRA_SSL_SERVER_NAME` when the certificate name differs from the dialed host, and `CASSANDRA_SSL_CERT_FILE` plus `CASSANDRA_SSL_KEY_FILE` when client certificate authentication is required. `CASSANDRA_SSL_INSECURE_SKIP_VERIFY=true` disables server certificate verification and should only be used for local development.

For Azure Entra ID, configure the Enterprise Application with:

- Identifier/entity ID: `SAML_ENTITY_ID`
- Reply URL/ACS URL: `SAML_ACS_URL`
- Sign-on URL: `ADMIN_BASE_URL`

The app accepts any user who successfully authenticates through the configured Entra ID tenant.

SAML credentials are provided as inline values rather than file paths, so they can be sourced directly from a Kubernetes Secret. `SAML_CERTIFICATE` and `SAML_PRIVATE_KEY` hold the service-provider certificate and key as PEM content, and the IdP metadata is supplied either as inline XML via `SAML_IDP_METADATA` or fetched from `SAML_IDP_METADATA_URL`. In Kubernetes, store these in a `Secret` and inject them with `valueFrom.secretKeyRef` so nothing is mounted as a file:

```yaml
env:
  - name: SAML_CERTIFICATE
    valueFrom:
      secretKeyRef:
        name: url-shortener-saml
        key: certificate.pem
  - name: SAML_PRIVATE_KEY
    valueFrom:
      secretKeyRef:
        name: url-shortener-saml
        key: private-key.pem
  - name: SAML_IDP_METADATA
    valueFrom:
      secretKeyRef:
        name: url-shortener-saml
        key: idp-metadata.xml
```

### Cloudflare Access (header authentication)

As an alternative to SAML, you can place the admin host behind Cloudflare Access (Zero Trust) and let the app trust the identity Cloudflare injects. Leave the `SAML_*` variables unset and set:

```text
AUTH_HEADER_ENABLED=true
AUTH_USER_EMAIL_HEADER=Cf-Access-Authenticated-User-Email
```

When enabled, the app authenticates each admin request from the email in `AUTH_USER_EMAIL_HEADER` (Cloudflare Access sends `Cf-Access-Authenticated-User-Email`) and uses that email as both the user ID and email. The header is trusted **without signature verification**, so this mode is only safe when the origin is reachable exclusively through Cloudflare — for example via a `cloudflared` tunnel with no public ingress. If the origin can be reached directly, a client could spoof the header and impersonate any user. Keep `AUTH_HEADER_ENABLED=false` (the default) in any deployment where that guarantee does not hold. Header authentication and SAML are independent; if both are configured, the trusted header is checked first.

## Tests

The sandbox may not allow writing to the default Go cache. Use workspace-local caches:

```bash
GOCACHE="$PWD/.gocache" GOMODCACHE="$PWD/.gomodcache" go test ./...
```

Run the Cassandra-backed integration test with local Cassandra listening on `localhost:9042`:

```bash
CASSANDRA_INTEGRATION=1 \
CASSANDRA_HOSTS=localhost:9042 \
CASSANDRA_KEYSPACE=url_shortener_integration \
go test ./internal/storage/cassandra -run TestIntegrationURLLifecycleAndHourlyHits -count=1 -v
```

Run the admin UI smoke test against a local app using `AUTH_DEV_BYPASS=true`:

```bash
UI_BASE_URL=http://localhost:18080 node test/ui/admin_smoke.mjs
```

GitHub Actions runs the unit tests, binary build, Cassandra integration test, admin UI smoke test, and Docker image build in `.github/workflows/ci.yml`. The workflow starts a `cassandra:5.0` service container for the Cassandra-backed checks.

When a GitHub release is published, the same workflow waits for the test job to pass, then builds and pushes the Docker image to GitHub Container Registry:

```text
ghcr.io/digitalis-io/url-shortener
```

## Container

Build the application image:

```bash
docker build -t url-shortener:local .
```

Run it against local Cassandra:

```bash
docker run --rm -p 8080:8080 \
  -e AUTH_DEV_BYPASS=true \
  -e SESSION_SECRET=local-dev-secret \
  -e PUBLIC_BASE_URL=http://localhost:8080 \
  -e ADMIN_BASE_URL=http://localhost:8080 \
  -e CASSANDRA_HOSTS=host.docker.internal:9042 \
  url-shortener:local
```

## Helm Chart

The chart is published to the GitHub Container Registry OCI endpoint and works with any Kubernetes 1.26+ cluster.

**Install:**

```bash
helm install url-shortener oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version 0.1.0 \
  --set app.publicBaseURL=https://short.example.com \
  --set app.adminBaseURL=https://admin.example.com \
  --set cassandra.hosts=cassandra:9042 \
  --set session.secret=your-session-secret
```

**Upgrade:**

```bash
helm upgrade url-shortener oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version 0.2.0 \
  -f my-values.yaml
```

See [`charts/url-shortener/README.md`](charts/url-shortener/README.md) for the full values reference.

## Cassandra Tables

- `urls_by_code`: redirect and metadata source of truth.
- `urls_by_created_day`: recent admin list without scanning.
- `hits_by_short_url_hour`: hourly hit counters for admin charts.

URL metadata tables use a Cassandra `default_time_to_live` of `7776000` seconds, which is 90 days. The hourly hit counter table does not use this URL metadata TTL.

---

Maintained by [Digitalis.io](https://digitalis.io). For support, visit [digitalis.io/contact](https://digitalis.io/contact).
