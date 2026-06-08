# url-shortener Helm chart

A production-ready Helm chart for the [url-shortener](https://github.com/digitalis-io/url-shortener)
service — a Go URL shortener backed by Cassandra and configured entirely through
environment variables.

The chart is published to the GitHub Container Registry as an OCI artifact at
`oci://ghcr.io/digitalis-io/helm-charts/url-shortener`.

## Prerequisites

- Kubernetes **1.29+**
- Helm **3.8+** (OCI support is required to pull the chart)
- An external Cassandra cluster reachable from the pods

## Quick start

```bash
helm install url-shortener \
  oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version <version> \
  --set app.publicBaseURL=https://short.example \
  --set app.adminBaseURL=https://admin.example \
  --set cassandra.hosts=cassandra:9042 \
  --set session.secret=$(openssl rand -hex 32)
```

`app.publicBaseURL`, `app.adminBaseURL`, and `cassandra.hosts` are required and
validated by `values.schema.json`; install fails fast if any is empty.

## Pulling the chart

```bash
# Inspect without installing
helm show values oci://ghcr.io/digitalis-io/helm-charts/url-shortener --version <version>

# Download the packaged chart
helm pull oci://ghcr.io/digitalis-io/helm-charts/url-shortener --version <version>
```

The `digitalis-io` org's GHCR package must allow anonymous pulls for the commands
above to work without `helm registry login`.

## Authentication

The admin UI supports three mutually compatible modes, selected by what you configure:

- **SAML** — set `saml.idpMetadataURL` (or provide metadata another way) plus
  `saml.certificate` and `saml.privateKey` (PEM). These render into a `Secret`.
- **Trusted header** (e.g. Cloudflare Access) — set `app`/`auth` env so the app reads
  the user from a trusted header. Only safe behind a proxy that strips client-supplied
  copies of that header.
- **Dev bypass** — `auth.devBypass=true` authenticates every request as a development
  user. Never enable in production.

## Secret handling

`session.secret`, `cassandra.password`, `saml.privateKey`, and `saml.certificate`
are rendered into a chart-managed `Secret` and injected into the Deployment via
`secretKeyRef` — they never appear in the ConfigMap or inline in the Deployment.

To supply secrets from an external source (External Secrets Operator, Vault, a
manually-created `Secret`, etc.), set the matching `existingSecret` and omit the
inline value. The external Secret must contain these keys:

| Group       | `existingSecret` value | Required keys                       |
| ----------- | ---------------------- | ----------------------------------- |
| `session`   | `session.existingSecret`   | `SESSION_SECRET`                |
| `cassandra` | `cassandra.existingSecret` | `CASSANDRA_PASSWORD`            |
| `saml`      | `saml.existingSecret`      | `SAML_PRIVATE_KEY`, `SAML_CERTIFICATE` |

When an `existingSecret` is set for a group, the chart renders no `Secret` for it.

## Values reference

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `replicaCount` | int | `1` | Number of replicas (ignored when `autoscaling.enabled`). |
| `image.repository` | string | `ghcr.io/digitalis-io/url-shortner` | Container image repository. |
| `image.tag` | string | `""` | Image tag; defaults to `.Chart.AppVersion` when empty. |
| `image.pullPolicy` | string | `IfNotPresent` | Image pull policy. |
| `imagePullSecrets` | list | `[]` | Image pull secrets. |
| `nameOverride` | string | `""` | Override the chart name portion of resource names. |
| `fullnameOverride` | string | `""` | Override the full resource name. |
| `serviceAccount.create` | bool | `true` | Create a ServiceAccount. |
| `serviceAccount.name` | string | `""` | Name of the ServiceAccount (defaults to the fullname / `default`). |
| `serviceAccount.annotations` | map | `{}` | ServiceAccount annotations. |
| `podAnnotations` | map | `{}` | Extra pod annotations. |
| `podSecurityContext` | map | `runAsNonRoot: true`, UID/GID/fsGroup `65532` | Pod-level security context. |
| `securityContext` | map | `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true`, drop `ALL` | Container security context. |
| `service.type` | string | `ClusterIP` | Service type. |
| `service.port` | int | `80` | Service port (targets container port `8080`). |
| `service.targetPort` | int | `8080` | Container port (named `http`). |
| `ingress.enabled` | bool | `false` | Render an Ingress. |
| `ingress.className` | string | `""` | IngressClass name. |
| `ingress.annotations` | map | `{}` | Ingress annotations. |
| `ingress.hosts` | list | `[]` | Ingress hosts/paths. |
| `ingress.tls` | list | `[]` | Ingress TLS configuration. |
| `resources` | map | `{}` | Container resource requests/limits. |
| `autoscaling.enabled` | bool | `false` | Render a HorizontalPodAutoscaler. |
| `autoscaling.minReplicas` | int | `1` | Minimum replicas. |
| `autoscaling.maxReplicas` | int | `5` | Maximum replicas. |
| `autoscaling.targetCPUUtilizationPercentage` | int | `80` | Target CPU utilization. |
| `app.httpAddr` | string | `:8080` | `HTTP_ADDR` listen address. |
| `app.publicBaseURL` | string | `http://localhost:8080` | **Required.** `PUBLIC_BASE_URL` for generated short links. |
| `app.adminBaseURL` | string | `http://localhost:8080` | **Required.** `ADMIN_BASE_URL` for the admin UI/SAML. |
| `app.codeLength` | int | `7` | `CODE_LENGTH` short-code length. |
| `cassandra.hosts` | string | `cassandra:9042` | **Required.** `CASSANDRA_HOSTS`, comma-separated. |
| `cassandra.keyspace` | string | `url_shortener` | `CASSANDRA_KEYSPACE`. |
| `cassandra.username` | string | `""` | `CASSANDRA_USERNAME`. |
| `cassandra.password` | string | `""` | `CASSANDRA_PASSWORD` (rendered into a Secret). |
| `cassandra.existingSecret` | string | `""` | External Secret with `CASSANDRA_PASSWORD`. |
| `cassandra.sslEnabled` | bool | `false` | `CASSANDRA_SSL_ENABLED`. |
| `cassandra.localDC` | string | `""` | `CASSANDRA_LOCAL_DC` for DC-aware routing. |
| `cassandra.consistency` | string | `LOCAL_QUORUM` | `CASSANDRA_CONSISTENCY`. |
| `session.secret` | string | `""` | `SESSION_SECRET` (rendered into a Secret). |
| `session.existingSecret` | string | `""` | External Secret with `SESSION_SECRET`. |
| `saml.entityID` | string | `""` | `SAML_ENTITY_ID`. |
| `saml.acsURL` | string | `""` | `SAML_ACS_URL`. |
| `saml.idpMetadataURL` | string | `""` | `SAML_IDP_METADATA_URL`. |
| `saml.privateKey` | string | `""` | `SAML_PRIVATE_KEY` PEM (rendered into a Secret). |
| `saml.certificate` | string | `""` | `SAML_CERTIFICATE` PEM (rendered into a Secret). |
| `saml.existingSecret` | string | `""` | External Secret with `SAML_PRIVATE_KEY` and `SAML_CERTIFICATE`. |
| `auth.devBypass` | bool | `false` | `AUTH_DEV_BYPASS`. Never enable in production. |
| `livenessProbe` | map | `GET /healthz:8080` | Liveness probe. |
| `readinessProbe` | map | `GET /readyz:8080` | Readiness probe (checks Cassandra). |
| `nodeSelector` | map | `{}` | Node selector. |
| `tolerations` | list | `[]` | Tolerations. |
| `affinity` | map | `{}` | Affinity rules. |

## Upgrade notes

```bash
helm upgrade url-shortener \
  oci://ghcr.io/digitalis-io/helm-charts/url-shortener \
  --version <new-version> \
  --reuse-values
```

The Deployment carries a `checksum/config` annotation, so changing non-sensitive
config triggers a rolling restart automatically. The default `RollingUpdate`
strategy keeps the service available during upgrades.

## Development

Validate the chart locally:

```bash
helm lint charts/url-shortener
helm template url-shortener charts/url-shortener \
  --set app.publicBaseURL=https://short.example \
  --set app.adminBaseURL=https://admin.example \
  --set cassandra.hosts=cassandra:9042
helm unittest charts/url-shortener
```
