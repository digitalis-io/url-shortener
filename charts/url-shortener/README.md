<p align="center">
  <a href="https://digitalis.io">
    <img src="https://digitalis.io/wp-content/uploads/2020/06/digitalis-logo.png" alt="Digitalis.IO" width="300">
  </a>
</p>

# url-shortener Helm Chart

Helm chart for the [url-shortener](https://github.com/digitalis-io/url-shortener) service — a self-hosted URL shortener backed by Apache Cassandra.

## Prerequisites

- Kubernetes 1.26+
- Helm 3.10+
- An external Cassandra cluster (5.x recommended) with the `url_shortener` keyspace and tables created, **or** set `cassandra.createKeyspace: true` to have the app create them on first start (requires `CREATE ON ALL KEYSPACES`)

## Quick Start

```bash
helm install url-shortener oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version 0.1.0 \
  --set app.publicBaseURL=https://short.example.com \
  --set app.adminBaseURL=https://admin.example.com \
  --set cassandra.hosts=cassandra:9042 \
  --set session.secret=change-me
```

```bash
# Upgrade to a new version
helm upgrade url-shortener oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version 0.2.0 \
  -f values-prod.yaml

# Uninstall
helm uninstall url-shortener
```

## Examples

### Minimal — dev/test with auth bypass

Useful for local testing. **Never use `auth.devBypass: true` in production.**

```yaml
# values-dev.yaml
app:
  publicBaseURL: http://localhost:8080
  adminBaseURL: http://localhost:8080

cassandra:
  hosts: localhost:9042

session:
  secret: dev-only-secret

auth:
  devBypass: true
```

```bash
helm install url-shortener oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version 0.1.0 \
  -f values-dev.yaml
```

---

### Production — SAML auth, Ingress, resource limits, HPA

```yaml
# values-prod.yaml
replicaCount: 2

image:
  tag: "0.4.1"

app:
  publicBaseURL: https://short.example.com
  adminBaseURL: https://admin.example.com
  codeLength: 8
  createRateLimitPerMinute: 30

cassandra:
  hosts: "cass-1.example.com:9042,cass-2.example.com:9042,cass-3.example.com:9042"
  keyspace: url_shortener_prod
  username: url_shortener_app
  password: ""                 # set via --set or existingSecret
  localDC: dc1
  consistency: LOCAL_QUORUM
  numConns: 4

session:
  secret: ""                   # set via --set or existingSecret

saml:
  entityID: https://admin.example.com/saml/metadata
  acsURL: https://admin.example.com/saml/acs
  idpMetadataURL: https://login.microsoftonline.com/<tenant>/federationmetadata/2007-06/federationmetadata.xml
  privateKey: |
    -----BEGIN RSA PRIVATE KEY-----
    ...
    -----END RSA PRIVATE KEY-----
  certificate: |
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----

service:
  type: ClusterIP
  port: 80

ingress:
  enabled: true
  className: nginx
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "1m"
    cert-manager.io/cluster-issuer: letsencrypt-prod
  hosts:
    - host: short.example.com
      paths:
        - path: /
          pathType: Prefix
    - host: admin.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: url-shortener-tls
      hosts:
        - short.example.com
        - admin.example.com

resources:
  requests:
    cpu: 50m
    memory: 64Mi
  limits:
    cpu: 500m
    memory: 256Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70
```

---

### Cassandra TLS — inline PEM

The chart stores the PEM content in a Secret and volume-mounts it at `/etc/cassandra-ssl/`.

```yaml
cassandra:
  hosts: "cass-1.example.com:9042"
  sslEnabled: true
  sslServerName: cass-1.example.com   # if cert CN differs from dialed host

  # CA only (server-auth TLS)
  sslCA: |
    -----BEGIN CERTIFICATE-----
    MIIB...
    -----END CERTIFICATE-----

  # Add sslCert + sslKey for mutual TLS (mTLS)
  sslCert: |
    -----BEGIN CERTIFICATE-----
    MIIB...
    -----END CERTIFICATE-----
  sslKey: |
    -----BEGIN RSA PRIVATE KEY-----
    MIIB...
    -----END RSA PRIVATE KEY-----
```

---

### Cassandra TLS — cert-manager managed Secret

Use `existingSSLSecret` when cert-manager (or any operator) already creates the Secret.
The chart mounts it at `/etc/cassandra-ssl/` and sets the file path env vars automatically.
Expected Secret keys: `ca.crt`, `tls.crt`, `tls.key`.

```yaml
cassandra:
  hosts: "cass-1.example.com:9042"
  sslEnabled: true
  existingSSLSecret: cassandra-tls   # created by cert-manager Certificate resource
```

```yaml
# cert-manager Certificate that produces the Secret above
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: cassandra-tls
spec:
  secretName: cassandra-tls
  issuerRef:
    name: internal-ca
    kind: ClusterIssuer
  commonName: cass-1.example.com
  dnsNames:
    - cass-1.example.com
```

---

### Cassandra auth — pre-existing Secret

Skip the chart-rendered Secret entirely and reference one you manage yourself
(external-secrets, Sealed Secrets, manual).

```yaml
cassandra:
  hosts: cassandra:9042
  existingSecret: cassandra-credentials   # must contain key: CASSANDRA_PASSWORD
```

```yaml
# example Secret (managed outside this chart)
apiVersion: v1
kind: Secret
metadata:
  name: cassandra-credentials
type: Opaque
stringData:
  CASSANDRA_PASSWORD: "hunter2"
```

---

### vals-operator — Cassandra auth from Vault

Requires [vals-operator](https://github.com/digitalis-io/vals-operator) installed in the cluster.
The chart renders a `ValsSecret` resource; vals-operator creates and rotates the K8s Secret.

```yaml
cassandra:
  hosts: cassandra:9042
  valsSecret:
    enabled: true
    ttl: 3600   # vals-operator re-syncs the Secret every 3600 seconds
    data:
      CASSANDRA_PASSWORD:
        ref: "ref+vault://secret/data/cassandra#password"
        encoding: text
```

The Secret name vals-operator creates defaults to `<release>-cassandra`. Override with `valsSecret.name`:

```yaml
cassandra:
  valsSecret:
    enabled: true
    name: my-cassandra-secret   # vals-operator will create a Secret with this name
    ttl: 3600
    data:
      CASSANDRA_PASSWORD:
        ref: "ref+vault://secret/data/cassandra#password"
        encoding: text
```

---

### vals-operator — Cassandra SSL certificates from Vault

```yaml
cassandra:
  hosts: cassandra:9042
  sslEnabled: true
  valsSSLSecret:
    enabled: true
    ttl: 86400
    data:
      ca.crt:
        ref: "ref+vault://secret/data/cassandra/ssl#ca_cert"
        encoding: text
      tls.crt:
        ref: "ref+vault://secret/data/cassandra/ssl#client_cert"
        encoding: text
      tls.key:
        ref: "ref+vault://secret/data/cassandra/ssl#client_key"
        encoding: text
```

Only keys with a non-empty `ref` are included in the `ValsSecret` spec. Omit `tls.crt` / `tls.key` entirely for server-auth-only TLS.

---

### vals-operator — AWS SSM / Secrets Manager

vals supports many backends via the `ref+` URI scheme.

```yaml
cassandra:
  valsSecret:
    enabled: true
    ttl: 3600
    data:
      CASSANDRA_PASSWORD:
        ref: "ref+awssecrets://us-east-1/cassandra/password"
        encoding: text
```

---

### vals-operator — both auth and SSL from Vault

```yaml
cassandra:
  hosts: "cass-1.example.com:9042"
  username: app_user
  sslEnabled: true

  valsSecret:
    enabled: true
    ttl: 3600
    data:
      CASSANDRA_PASSWORD:
        ref: "ref+vault://secret/data/cassandra#password"
        encoding: text

  valsSSLSecret:
    enabled: true
    ttl: 86400
    data:
      ca.crt:
        ref: "ref+vault://secret/data/cassandra/ssl#ca"
        encoding: text
      tls.crt:
        ref: "ref+vault://secret/data/cassandra/ssl#cert"
        encoding: text
      tls.key:
        ref: "ref+vault://secret/data/cassandra/ssl#key"
        encoding: text
```

---

### Session and SAML from existing Secrets

```yaml
session:
  existingSecret: session-credentials   # key: SESSION_SECRET

saml:
  existingSecret: saml-credentials      # keys: SAML_PRIVATE_KEY, SAML_CERTIFICATE, SAML_IDP_METADATA
```

---

### Multi-DC Cassandra with speculative execution

```yaml
cassandra:
  hosts: "cass-dc1-1:9042,cass-dc1-2:9042,cass-dc1-3:9042"
  localDC: dc1
  consistency: LOCAL_QUORUM
  serialConsistency: LOCAL_SERIAL
  numConns: 4
  speculativeExecutionEnabled: true
  speculativeAttempts: 2
  speculativeDelay: 100ms
  retryAttempts: 5
  retryMinBackoff: 200ms
  retryMaxBackoff: 5s
```

---

## Values Reference

### Core

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `replicaCount` | int | `1` | Number of pod replicas |
| `image.repository` | string | `ghcr.io/digitalis-io/url-shortener` | Container image repository |
| `image.tag` | string | `""` | Image tag; defaults to `Chart.appVersion` |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | list | `[]` | Image pull secrets |
| `nameOverride` | string | `""` | Override chart name |
| `fullnameOverride` | string | `""` | Override full release name |
| `nodeSelector` | object | `{}` | Node selector |
| `tolerations` | list | `[]` | Tolerations |
| `affinity` | object | `{}` | Affinity rules |

### Service Account

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `serviceAccount.create` | bool | `true` | Create a ServiceAccount |
| `serviceAccount.name` | string | `""` | Name override (auto-generated when empty) |
| `serviceAccount.annotations` | object | `{}` | Annotations added to the ServiceAccount |

### Pod

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `podAnnotations` | object | `{}` | Extra pod annotations |
| `podLabels` | object | `{}` | Extra pod labels |
| `podSecurityContext.runAsNonRoot` | bool | `true` | Run as non-root |
| `podSecurityContext.runAsUser` | int | `65532` | UID |
| `podSecurityContext.runAsGroup` | int | `65532` | GID |
| `podSecurityContext.fsGroup` | int | `65532` | fsGroup |
| `securityContext.allowPrivilegeEscalation` | bool | `false` | Disable privilege escalation |
| `securityContext.readOnlyRootFilesystem` | bool | `true` | Read-only root filesystem |
| `resources` | object | `{}` | CPU/memory requests and limits |

### Service & Ingress

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `service.type` | string | `ClusterIP` | Service type |
| `service.port` | int | `80` | Service port |
| `service.targetPort` | int | `8080` | Container port |
| `ingress.enabled` | bool | `false` | Enable Ingress |
| `ingress.className` | string | `""` | Ingress class name |
| `ingress.annotations` | object | `{}` | Ingress annotations |
| `ingress.hosts` | list | `[]` | Host rules (`host`, `paths[].path`, `paths[].pathType`) |
| `ingress.tls` | list | `[]` | TLS blocks (`secretName`, `hosts`) |

### Autoscaling

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `autoscaling.enabled` | bool | `false` | Enable HPA |
| `autoscaling.minReplicas` | int | `1` | Minimum replicas |
| `autoscaling.maxReplicas` | int | `5` | Maximum replicas |
| `autoscaling.targetCPUUtilizationPercentage` | int | `80` | CPU utilisation target % |

### Application

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `app.env` | string | `production` | `APP_ENV` |
| `app.httpAddr` | string | `:8080` | `HTTP_ADDR` |
| `app.publicBaseURL` | string | **required** | Public-facing base URL used in generated short URLs |
| `app.adminBaseURL` | string | **required** | Admin UI and SAML base URL |
| `app.codeLength` | int | `7` | Short code length |
| `app.createRateLimitPerMinute` | int | `60` | Short URL creation rate limit per minute |
| `app.clickEventBufferSize` | int | `1000` | In-memory click event buffer |
| `app.clickEventFlushInterval` | string | `1s` | How often buffered click events are flushed to Cassandra |

### Cassandra

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cassandra.hosts` | string | **required** | Comma-separated `host:port` list |
| `cassandra.keyspace` | string | `url_shortener` | Keyspace name |
| `cassandra.createKeyspace` | bool | `false` | When `true`, the app creates the keyspace and tables on startup. Requires the Cassandra user to hold `CREATE` on `<all keyspaces>`. Leave `false` (default) when the keyspace is pre-provisioned — only per-keyspace `SELECT`/`MODIFY`/`ALTER` permissions are needed. |
| `cassandra.replicationStrategy` | string | `SimpleStrategy` | Replication strategy used when `createKeyspace: true`. Use `NetworkTopologyStrategy` for multi-DC clusters and set `replicationFactor` to match your DC configuration. |
| `cassandra.replicationFactor` | int | `1` | Replication factor used when `createKeyspace: true`. Set to `3` (or higher) for production. |
| `cassandra.username` | string | `""` | Username |
| `cassandra.password` | string | `""` | Password — stored in a Secret |
| `cassandra.existingSecret` | string | `""` | Existing Secret name with key `CASSANDRA_PASSWORD`; skips chart-rendered Secret |
| `cassandra.localDC` | string | `""` | Local datacenter for token-aware routing (required with `LOCAL_*` consistency) |
| `cassandra.consistency` | string | `LOCAL_QUORUM` | Read/write consistency level |
| `cassandra.serialConsistency` | string | `LOCAL_SERIAL` | LWT serial consistency level |
| `cassandra.protoVersion` | int | `4` | CQL native protocol version |
| `cassandra.numConns` | int | `2` | Connections per host |
| `cassandra.pageSize` | int | `5000` | Query page size |
| `cassandra.connectTimeout` | string | `10s` | Connection establishment timeout |
| `cassandra.timeout` | string | `10s` | Query timeout |
| `cassandra.writeTimeout` | string | `""` | Write timeout; defaults to `timeout` |
| `cassandra.reconnectInterval` | string | `60s` | Reconnect interval after host failure |
| `cassandra.socketKeepalive` | string | `15s` | TCP keepalive interval |
| `cassandra.maxWaitSchemaAgreement` | string | `60s` | Max wait for schema agreement |
| `cassandra.retryAttempts` | int | `3` | Idempotent retry attempts (0 = disabled) |
| `cassandra.retryMinBackoff` | string | `100ms` | Retry backoff minimum |
| `cassandra.retryMaxBackoff` | string | `2s` | Retry backoff maximum |
| `cassandra.speculativeExecutionEnabled` | bool | `false` | Enable speculative execution on reads |
| `cassandra.speculativeAttempts` | int | `1` | Speculative attempt count |
| `cassandra.speculativeDelay` | string | `50ms` | Delay before issuing a speculative retry |

### Cassandra TLS

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cassandra.sslEnabled` | bool | `false` | Enable TLS for Cassandra connections |
| `cassandra.sslServerName` | string | `""` | Override TLS server name (SNI) |
| `cassandra.sslInsecureSkipVerify` | bool | `false` | Skip server certificate verification — dev only |
| `cassandra.sslCA` | string | `""` | CA certificate PEM — chart stores in a Secret, mounts at `/etc/cassandra-ssl/ca.crt` |
| `cassandra.sslCert` | string | `""` | Client certificate PEM — mounted at `/etc/cassandra-ssl/tls.crt` (mTLS) |
| `cassandra.sslKey` | string | `""` | Client private key PEM — mounted at `/etc/cassandra-ssl/tls.key` (mTLS) |
| `cassandra.existingSSLSecret` | string | `""` | Existing Secret with keys `ca.crt`, `tls.crt`, `tls.key`; skips chart-rendered SSL Secret |

### vals-operator — Cassandra auth

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cassandra.valsSecret.enabled` | bool | `false` | Render a `ValsSecret` for Cassandra auth; skips plain Secret |
| `cassandra.valsSecret.name` | string | `""` | Name of the Secret vals-operator will create; defaults to `<release>-cassandra` |
| `cassandra.valsSecret.ttl` | int | `3600` | Sync interval in seconds |
| `cassandra.valsSecret.labels` | object | `{}` | Extra labels on the `ValsSecret` resource |
| `cassandra.valsSecret.data` | object | see values | Map of `key → {ref, encoding}` entries; keys with empty `ref` are omitted |

### vals-operator — Cassandra SSL

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `cassandra.valsSSLSecret.enabled` | bool | `false` | Render a `ValsSecret` for Cassandra SSL; skips plain SSL Secret |
| `cassandra.valsSSLSecret.name` | string | `""` | Name of the Secret vals-operator will create; defaults to `<release>-cassandra-ssl` |
| `cassandra.valsSSLSecret.ttl` | int | `3600` | Sync interval in seconds |
| `cassandra.valsSSLSecret.labels` | object | `{}` | Extra labels on the `ValsSecret` resource |
| `cassandra.valsSSLSecret.data` | object | see values | Map of `key → {ref, encoding}`; scaffold keys are `ca.crt`, `tls.crt`, `tls.key` |

### Session

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `session.secret` | string | `""` | Session signing secret — stored in a Secret |
| `session.existingSecret` | string | `""` | Existing Secret name with key `SESSION_SECRET` |

### SAML

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `saml.entityID` | string | `""` | SP entity ID (defaults to `<adminBaseURL>/saml/metadata`) |
| `saml.acsURL` | string | `""` | Assertion Consumer Service URL (defaults to `<adminBaseURL>/saml/acs`) |
| `saml.idpMetadataURL` | string | `""` | URL to fetch IdP federation metadata |
| `saml.idpMetadata` | string | `""` | Inline IdP metadata XML — stored in a Secret |
| `saml.privateKey` | string | `""` | SP private key PEM — stored in a Secret |
| `saml.certificate` | string | `""` | SP certificate PEM — stored in a Secret |
| `saml.existingSecret` | string | `""` | Existing Secret name with keys `SAML_PRIVATE_KEY`, `SAML_CERTIFICATE`, `SAML_IDP_METADATA` |

### Auth

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `auth.devBypass` | bool | `false` | Skip SAML — injects a hardcoded dev user. **Never `true` in production.** |
| `auth.devUserID` | string | `local-dev-user` | Dev bypass user ID |
| `auth.devUserEmail` | string | `dev@example.com` | Dev bypass user email |

## Secret priority

For each credential group, the chart resolves the Secret name in this order:

```
existingSecret  (highest)
  └── valsSecret.name / valsSSLSecret.name
        └── <release>-<group>  (chart-rendered default)
```

`existingSecret` always wins. `valsSecret` / `valsSSLSecret` suppresses the chart-rendered plain Secret but the Deployment still references the Secret vals-operator creates under `spec.name`.

## OCI Pull

```bash
# Pull a specific version
helm pull oci://ghcr.io/digitalis-io/helm-charts/url-shortener --version 0.1.0

# Inspect default values before install
helm show values oci://ghcr.io/digitalis-io/helm-charts/url-shortener --version 0.1.0
```

No authentication required — the chart is published publicly.

## Maintainers

Maintained by [Digitalis.io](https://digitalis.io) — [contact us](https://digitalis.io/contact)
