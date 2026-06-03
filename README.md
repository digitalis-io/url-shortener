# url-shortner

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

Important environment variables:

```text
HTTP_ADDR=:8080
PUBLIC_BASE_URL=https://short.example
ADMIN_BASE_URL=https://admin-short.example
CASSANDRA_HOSTS=localhost:9042
CASSANDRA_KEYSPACE=url_shortener
CASSANDRA_USERNAME=
CASSANDRA_PASSWORD=
SESSION_SECRET=
AUTH_DEV_BYPASS=false
SAML_ENTITY_ID=https://admin-short.example/saml/metadata
SAML_ACS_URL=https://admin-short.example/saml/acs
SAML_IDP_METADATA_URL=
SAML_IDP_METADATA_FILE=
SAML_PRIVATE_KEY_FILE=
SAML_CERTIFICATE_FILE=
CODE_LENGTH=7
```

`PUBLIC_BASE_URL` is used in generated short URLs. `ADMIN_BASE_URL` is used for the admin UI, SAML routes, and admin API calls.

For Azure Entra ID, configure the Enterprise Application with:

- Identifier/entity ID: `SAML_ENTITY_ID`
- Reply URL/ACS URL: `SAML_ACS_URL`
- Sign-on URL: `ADMIN_BASE_URL`

The app accepts any user who successfully authenticates through the configured Entra ID tenant.

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

## Cassandra Tables

- `urls_by_code`: redirect and metadata source of truth.
- `urls_by_created_day`: recent admin list without scanning.
- `hits_by_short_url_hour`: hourly hit counters for admin charts.
