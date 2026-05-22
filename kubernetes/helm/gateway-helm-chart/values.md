# Configuration Reference

All configurable parameters for the `gateway` Helm chart.

## Global

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `nameOverride` | string | `""` | Override the chart name portion of resource names. |
| `fullnameOverride` | string | `""` | Override the full resource name prefix. |
| `imagePullSecrets` | list | `[]` | List of image pull secret names for all pods. |
| `commonLabels` | object | `{}` | Labels added to every resource created by this chart. |
| `commonAnnotations` | object | `{}` | Annotations added to every resource created by this chart. |

## Service Account

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `serviceAccount.create` | bool | `true` | Create a dedicated ServiceAccount for the gateway components. |
| `serviceAccount.annotations` | object | `{}` | Annotations to add to the ServiceAccount. |
| `serviceAccount.name` | string | `""` | Override the ServiceAccount name. If empty, a name is generated from the chart release name. |

## Gateway

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.developmentMode` | bool | `false` | Enable development mode. When `false` (production), encryption keys and TLS must be properly configured. |
| `gateway.config_toml` | string | `""` | Raw TOML configuration string. When set, this overrides config generated from the structured `gateway.config.*` values. |
| `gateway.configMap.annotations` | object | `{}` | Annotations applied to the generated ConfigMap. |
| `gateway.configMap.labels` | object | `{}` | Labels applied to the generated ConfigMap. |

## Controller Auth — Basic

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.auth.basic.enabled` | bool | `true` | Enable HTTP Basic authentication for the controller API. |
| `gateway.config.controller.auth.basic.users` | list | `[{username: "admin", password: "admin", password_hashed: false, roles: ["admin"]}]` | List of basic auth user entries. Each entry has `username`, `password`, `password_hashed`, and `roles`. Change credentials before deploying to production. |

## Controller Auth — IdP (JWT)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.auth.idp.enabled` | bool | `false` | Enable JWT-based IdP authentication for the controller API. When enabled, the controller validates bearer tokens against the configured JWKS endpoint. |
| `gateway.config.controller.auth.idp.jwks_url` | string | `""` | URL of the JWKS endpoint used to verify incoming JWT signatures. |
| `gateway.config.controller.auth.idp.issuer` | string | `""` | Expected `iss` claim value in incoming JWTs. |
| `gateway.config.controller.auth.idp.roles_claim` | string | `"scope"` | JWT claim from which roles are extracted. |
| `gateway.config.controller.auth.idp.role_mapping` | object | `{}` | Map of IdP roles to controller roles (e.g., `{editor: admin}`). |

## Controller Server

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.server.gateway_id` | string | `"platform-gateway-id"` | Unique identifier for this gateway instance, used in xDS node metadata and control-plane registration. |


## Controller Storage

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.storage.type` | string | `"sqlite"` | Storage backend type. Supported values: `sqlite`, `postgres`. Use `postgres` for production multi-replica deployments. |
| `gateway.config.controller.storage.sqlite.path` | string | `"./data/gateway.db"` | File path for the SQLite database. Only used when `storage.type` is `sqlite`. Requires a persistent volume via `gateway.controller.persistence`. |

## Controller PostgreSQL

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.storage.postgres.dsn` | string | `""` | Full PostgreSQL DSN (e.g., `postgres://user:pass@host:5432/db`). When set, individual host/port/user fields are ignored. |
| `gateway.config.controller.storage.postgres.host` | string | `""` | PostgreSQL server hostname. |
| `gateway.config.controller.storage.postgres.port` | int | `5432` | PostgreSQL server port. |
| `gateway.config.controller.storage.postgres.database` | string | `""` | Target database name. |
| `gateway.config.controller.storage.postgres.user` | string | `""` | PostgreSQL username. The password is supplied via `gateway.controller.postgres.passwordSecretRef`. |
| `gateway.config.controller.storage.postgres.sslmode` | string | `"require"` | PostgreSQL SSL mode. Recommended `require` or `verify-full` in production. Avoid `disable` outside development. |
| `gateway.config.controller.storage.postgres.connect_timeout` | string | `"5s"` | Timeout for establishing a new PostgreSQL connection. |
| `gateway.config.controller.storage.postgres.max_open_conns` | int | `25` | Maximum number of open connections in the pool. |
| `gateway.config.controller.storage.postgres.max_idle_conns` | int | `5` | Maximum number of idle connections kept in the pool. |
| `gateway.config.controller.storage.postgres.conn_max_lifetime` | string | `"30m"` | Maximum duration a connection may be reused before being closed. |
| `gateway.config.controller.storage.postgres.conn_max_idle_time` | string | `"5m"` | Maximum amount of time a connection may remain idle before being closed. |
| `gateway.config.controller.storage.postgres.application_name` | string | `"gateway-controller"` | Application name reported to PostgreSQL, useful for identifying connections in `pg_stat_activity`. |

## Controller Policies

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.policies.definitions_path` | string | `"./default-policies"` | Directory path containing the built-in policy definition files loaded by the controller at startup. |

## Controller Control Plane

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.controlplane.insecure_skip_verify` | bool | `false` | Skip TLS certificate verification when connecting to the control plane. |
| `gateway.config.controller.controlplane.reconnect_initial` | string | `"1s"` | Initial delay before attempting to reconnect to the control plane after a connection failure. |
| `gateway.config.controller.controlplane.reconnect_max` | string | `"5m"` | Maximum backoff delay between control-plane reconnection attempts. |
| `gateway.config.controller.controlplane.polling_interval` | string | `"15m"` | Interval at which the controller polls the control plane for configuration updates. |
| `gateway.config.controller.controlplane.deployment_push_enabled` | bool | `false` | Enable the controller to push deployment state to the control plane. |
| `gateway.config.controller.controlplane.sync_batch_size` | int | `50` | Number of resources to sync per batch during full reconciliation. |
| `gateway.config.controller.controlplane.gateway_name` | string | `""` | Name used to identify this gateway on the control plane. |
| `gateway.config.controller.controlplane.apim_oauth2_client_id` | string | `""` | OAuth2 client ID for authenticating with the APIM control plane. See also `gateway.controller.controlPlane.token`. |
| `gateway.config.controller.controlplane.apim_oauth2_client_secret` | string | `""` | OAuth2 client secret for authenticating with the APIM control plane. |
| `gateway.config.controller.controlplane.apim_oauth2_username` | string | `""` | Username for OAuth2 password-grant flow with the APIM control plane. |
| `gateway.config.controller.controlplane.apim_oauth2_password` | string | `""` | Password for OAuth2 password-grant flow with the APIM control plane. |

## Controller Encryption

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.encryption.providers` | list | `[{type: aesgcm, keys: [{version: aesgcm256-v1, file: /app/data/aesgcm-keys/default-aesgcm256-v1.bin}]}]` | List of encryption provider configurations. Each provider specifies a `type` (e.g., `aesgcm`) and a list of versioned key file references. Required for encrypting sensitive data at rest when `gateway.developmentMode` is `false`. Configure `gateway.controller.encryptionKeys` to mount key material from a Kubernetes Secret. |

## Controller Logging

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.controller.logging.level` | string | `"debug"` | Log verbosity for the controller. Supported: `debug`, `info`, `warn`, `error`. Lower verbosity is recommended in production. |
| `gateway.config.controller.logging.format` | string | `"json"` | Log output format. Supported: `json`, `text`. |

## Router

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.router.gateway_host` | string | `"*"` | Virtual host domain for the router listener. Use `*` to match all incoming hosts, or a specific FQDN. |
gatewayRuntime.service.ports.http`. |
| `gateway.config.router.https_enabled` | bool | `true` | Enable the HTTPS listener. Requires `downstream_tls` configuration. |
| `gateway.config.router.tracing_service_name` | string | `"router"` | Service name reported in distributed tracing spans emitted by the router. |

## Router Access Logs

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.router.access_logs.enabled` | bool | `true` | Enable Envoy access logging. |
| `gateway.config.router.access_logs.format` | string | `"json"` | Access log format. Supported: `json`, `text`. When `json`, the fields in `json_fields` are used. |
| `gateway.config.router.access_logs.json_fields` | object | `{t, meth, path, proto, respCd, respFlg, bytesRx, bytesTx, dur, uSvcT, xff, ua, reqId, host, uHost, uProto, uPath, respCdDtl, connTrmDtl}` | Map of field names to Envoy access log format strings used when `format` is `json`. See Envoy documentation for supported substitution commands. |
| `gateway.config.router.access_logs.text_format` | string | See `values.yaml` | Access log line template used when `format` is `text`. Default includes method, path, protocol, response code, flags, bytes, duration, forwarded-for, user-agent, request-id, authority, and upstream host. |

## Router Downstream TLS

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.router.downstream_tls.minimum_protocol_version` | string | `"TLS1_2"` | Minimum TLS protocol version accepted from downstream clients. |
| `gateway.config.router.downstream_tls.maximum_protocol_version` | string | `"TLS1_3"` | Maximum TLS protocol version accepted from downstream clients. |
| `gateway.config.router.downstream_tls.ciphers` | string | See `values.yaml` | Comma-separated TLS 1.2 cipher suites for downstream connections. Defaults to a set of ECDHE/AES cipher suites. TLS 1.3 ciphers are not configurable. |

## Router Upstream TLS

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.router.upstream.tls.minimum_protocol_version` | string | `"TLS1_2"` | Minimum TLS protocol version for upstream (backend) connections. |
| `gateway.config.router.upstream.tls.maximum_protocol_version` | string | `"TLS1_3"` | Maximum TLS protocol version for upstream connections. |
| `gateway.config.router.upstream.tls.ciphers` | string | See `values.yaml` | Comma-separated TLS 1.2 cipher suites for upstream connections. Defaults to the same ECDHE/AES suite set as downstream. |
| `gateway.config.router.upstream.tls.trusted_cert_path` | string | `"/etc/ssl/certs/ca-certificates.crt"` | Path to the CA bundle used to verify upstream TLS certificates. |
| `gateway.config.router.upstream.tls.custom_certs_path` | string | `"./certificates"` | Directory containing additional custom CA certificates to trust for upstream connections. Populated via `gateway.controller.upstreamCerts`. |
| `gateway.config.router.upstream.tls.verify_host_name` | bool | `true` | Verify the upstream server's hostname against the certificate's SAN/CN. Disable only for debugging. |
| `gateway.config.router.upstream.tls.disable_ssl_verification` | bool | `false` | Disable all TLS verification for upstream connections. **Never enable in production.** |

## Router Upstream Timeouts

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.router.upstream.timeouts.route_timeout_ms` | int | `60000` | Maximum duration (ms) for a complete upstream request/response cycle. Setting `0` disables the timeout. |
| `gateway.config.router.upstream.timeouts.route_idle_timeout_ms` | int | `300000` | Duration (ms) with no activity on an upstream stream before it is torn down. Relevant for streaming and long-lived connections. |
| `gateway.config.router.upstream.timeouts.connect_timeout_ms` | int | `5000` | Timeout (ms) for establishing a TCP connection to an upstream host. |


## Policy Engine

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.policy_engine.admin.enabled` | bool | `true` | Enable the policy engine admin HTTP API. |
| `gateway.config.policy_engine.admin.port` | int | `9002` | Port for the policy engine admin API. Must match `gateway.gatewayRuntime.service.ports.policyEngineAdmin`. |
| `gateway.config.policy_engine.admin.allowed_ips` | list | `["*", "127.0.0.1"]` | List of IP addresses allowed to access the admin API. Restrict to specific CIDRs in production. |
| `gateway.config.policy_engine.file_config.path` | string | `""` | Path to the static policy configuration file. Only used when `config_mode.mode` is `file`. |

## Policy Engine xDS Client

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.policy_engine.xds.connect_timeout` | string | `"10s"` | Timeout for establishing the xDS gRPC stream to the controller policy server. |
| `gateway.config.policy_engine.xds.request_timeout` | string | `"5s"` | Timeout for individual xDS requests. |
| `gateway.config.policy_engine.xds.initial_reconnect_delay` | string | `"1s"` | Initial backoff delay before the first xDS reconnection attempt. |
| `gateway.config.policy_engine.xds.max_reconnect_delay` | string | `"60s"` | Maximum backoff delay between xDS reconnection attempts. |
| `gateway.config.policy_engine.xds.tls.enabled` | bool | `false` | Enable TLS on the xDS gRPC connection from the policy engine to the controller. Must align with `gateway.config.controller.policy_server.tls.enabled`. |

## Policy Engine Logging

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.policy_engine.logging.level` | string | `"info"` | Log verbosity for the policy engine. Supported: `debug`, `info`, `warn`, `error`. |
| `gateway.config.policy_engine.logging.format` | string | `"json"` | Log output format for the policy engine. Supported: `json`, `text`. |

## Immutable Gateway

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.config.immutable_gateway.enabled` | bool | `false` | Enable immutable gateway mode, where API configurations are loaded from static artifact files rather than from the controller's dynamic API. |
| `gateway.config.immutable_gateway.artifacts_dir` | string | `"/etc/api-platform-gateway/immutable_gateway/artifacts"` | Directory path containing the static gateway artifact files. Only used when `immutable_gateway.enabled` is `true`. |

## Gateway Controller — Image & Service

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.image.repository` | string | `"ghcr.io/wso2/api-platform/gateway-controller"` | Container image repository for the gateway controller. |
| `gateway.controller.image.tag` | string | `"1.1.0"` | Container image tag for the gateway controller. |
| `gateway.controller.image.pullPolicy` | string | `"Always"` | Image pull policy for the gateway controller. |
| `gateway.controller.imagePullSecrets` | list | `[]` | Image pull secrets specific to the controller (merged with `imagePullSecrets` at chart root). |
| `gateway.controller.service.type` | string | `"ClusterIP"` | Kubernetes Service type for the controller. `ClusterIP` is recommended; the controller is an internal component. |
| `gateway.controller.service.annotations` | object | `{}` | Annotations applied to the controller Service. |
| `gateway.controller.service.labels` | object | `{}` | Labels applied to the controller Service. |
| `gateway.controller.service.ports.rest` | int | `9090` | Service port for the controller REST API. |
| `gateway.controller.service.ports.admin` | int | `9092` | Service port for the controller admin API (health, ready). |
| `gateway.controller.service.ports.metrics` | int | `9091` | Service port for the controller Prometheus metrics endpoint. |

## Gateway Controller — Control Plane Connection

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.controlPlane.host` | string | `"host.docker.internal"` | Hostname of the upstream control plane. Change to the actual control plane address in production. |
| `gateway.controller.controlPlane.port` | int | `8443` | Port of the upstream control plane. |
| `gateway.controller.controlPlane.token.value` | string | `""` | Inline bearer token value for authenticating with the control plane. Not recommended for production — use `secretName` instead. |
| `gateway.controller.controlPlane.token.secretName` | string | `""` | Name of the Kubernetes Secret containing the control plane token. Takes precedence over `token.value` when set. |
| `gateway.controller.controlPlane.token.key` | string | `"token"` | Key within the Secret referenced by `token.secretName` that holds the token value. |

## Gateway Controller — TLS

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.tls.enabled` | bool | `true` | Enable TLS for the controller's xDS and policy servers. Strongly recommended in production. |
| `gateway.controller.tls.certificateProvider` | string | `"cert-manager"` | Certificate provisioning method. Supported: `cert-manager`, `secret`, `none`. |

## Gateway Controller — TLS (cert-manager)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.tls.certManager.create` | bool | `true` | Create a cert-manager `Certificate` resource. Requires cert-manager to be installed in the cluster. |
| `gateway.controller.tls.certManager.createIssuer` | bool | `true` | Create a cert-manager `Issuer` resource alongside the Certificate. |
| `gateway.controller.tls.certManager.issuerRef.name` | string | `"selfsigned-issuer"` | Name of the cert-manager Issuer or ClusterIssuer to use for signing. |
| `gateway.controller.tls.certManager.issuerRef.kind` | string | `"Issuer"` | Kind of the cert-manager issuer: `Issuer` or `ClusterIssuer`. |
| `gateway.controller.tls.certManager.commonName` | string | `"localhost"` | Common name (CN) for the generated certificate. |
| `gateway.controller.tls.certManager.dnsNames` | list | `["localhost", "*.localhost"]` | List of DNS SANs to include in the generated certificate. |
| `gateway.controller.tls.certManager.duration` | string | `"2160h"` | Certificate validity duration (90 days). |
| `gateway.controller.tls.certManager.renewBefore` | string | `"720h"` | How long before expiry cert-manager should renew the certificate (30 days). |

## Gateway Controller — TLS (existing secret)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.tls.secret.name` | string | `"gateway-tls"` | Name of the Kubernetes Secret containing the TLS certificate and key. Used when `certificateProvider` is `existing-secret`, or as the target secret name when using cert-manager. |
| `gateway.controller.tls.secret.certKey` | string | `"tls.crt"` | Key within the TLS secret that holds the certificate data. |
| `gateway.controller.tls.secret.keyKey` | string | `"tls.key"` | Key within the TLS secret that holds the private key data. |

## Gateway Controller — Upstream Certificates

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.upstreamCerts.enabled` | bool | `false` | Mount additional CA certificates for verifying upstream (backend) TLS connections. |
| `gateway.controller.upstreamCerts.secretName` | string | `""` | Name of a Kubernetes Secret containing custom CA certificates to mount. |
| `gateway.controller.upstreamCerts.configMapName` | string | `""` | Name of a Kubernetes ConfigMap containing custom CA certificates to mount. Either `secretName` or `configMapName` may be specified. |

## Gateway Controller — Encryption Keys

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.encryptionKeys.enabled` | bool | `false` | Mount encryption key material from a Kubernetes Secret. Required when `gateway.developmentMode` is `false` to supply AES-GCM keys for at-rest data encryption. |
| `gateway.controller.encryptionKeys.secretName` | string | `""` | Name of the Kubernetes Secret containing the encryption key files. |
| `gateway.controller.encryptionKeys.mountPath` | string | `"/app/data/aesgcm-keys"` | Path inside the controller container where encryption key files are mounted. Must match the `file` paths in `gateway.config.controller.encryption.providers`. |

## Gateway Controller — Storage & Persistence

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.logging.level` | string | `"info"` | Log level for the controller Kubernetes deployment (separate from `gateway.config.controller.logging.level` which controls the runtime config). |
| `gateway.controller.storage.type` | string | `"sqlite"` | Storage backend for the controller deployment. Mirrors `gateway.config.controller.storage.type`. Set to `postgres` for HA deployments — SQLite does not support multiple replicas. |
| `gateway.controller.storage.sqlitePath` | string | `"./data/gateway.db"` | SQLite database file path used by the controller deployment. |
| `gateway.controller.postgres.passwordSecretRef.name` | string | `""` | Name of the Kubernetes Secret that contains the PostgreSQL password. |
| `gateway.controller.postgres.passwordSecretRef.key` | string | `"password"` | Key within the Secret referenced by `postgres.passwordSecretRef.name`. |
| `gateway.controller.metrics.port` | int | `9091` | Port exposed for Prometheus metrics scraping. Must match `gateway.controller.service.ports.metrics`. |
| `gateway.controller.persistence.enabled` | bool | `true` | Create a PersistentVolumeClaim for the controller's data directory (required for SQLite). |
| `gateway.controller.persistence.existingClaim` | string | `""` | Name of an existing PVC to use. When set, no new PVC is created. |
| `gateway.controller.persistence.accessModes` | list | `["ReadWriteOnce"]` | Access modes for the PVC. `ReadWriteOnce` is appropriate for single-replica SQLite. For PostgreSQL with multiple replicas, the PVC can be omitted. |
| `gateway.controller.persistence.size` | string | `"100Mi"` | Storage capacity requested for the PVC. |
| `gateway.controller.persistence.storageClass` | string | `""` | StorageClass for the PVC. Empty string uses the cluster default. |

## Gateway Controller — Deployment

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.deployment.enabled` | bool | `true` | Deploy the gateway controller. Set `false` to skip the controller Deployment (e.g., when running the controller externally). |
| `gateway.controller.deployment.replicaCount` | int | `1` | Number of controller replicas. When using SQLite storage, keep at `1`. Increase only with a PostgreSQL backend. |
| `gateway.controller.deployment.volumeMountPath` | string | `"/app/data"` | Path in the controller container where the persistence volume is mounted. |
| `gateway.controller.deployment.extraEnv` | list | `[]` | Additional environment variables to inject into the controller container as `env` entries. |
| `gateway.controller.deployment.extraEnvFrom` | list | `[]` | Additional environment variable sources (`configMapRef`, `secretRef`) for the controller container. |
| `gateway.controller.deployment.env.xdsServerAddress` | string | `""` | Override the xDS server address the controller advertises to the policy engine. |
| `gateway.controller.deployment.extraVolumeMounts` | list | `[]` | Additional volume mounts for the controller container. |
| `gateway.controller.deployment.extraVolumes` | list | `[]` | Additional volumes to attach to the controller Pod. |
| `gateway.controller.deployment.labels` | object | `{}` | Labels added to the controller Deployment. |
| `gateway.controller.deployment.annotations` | object | `{}` | Annotations added to the controller Deployment. |
| `gateway.controller.deployment.podAnnotations` | object | `{}` | Annotations added to controller Pods. |
| `gateway.controller.deployment.podLabels` | object | `{}` | Labels added to controller Pods. |
| `gateway.controller.deployment.priorityClassName` | string | `""` | PriorityClass name assigned to controller Pods. |
| `gateway.controller.deployment.livenessProbe` | object | `httpGet /api/admin/v0.9/health` | Liveness probe configuration. Defaults to an HTTP GET against the admin health endpoint. |
| `gateway.controller.deployment.readinessProbe` | object | `httpGet /api/admin/v0.9/health` | Readiness probe configuration. Defaults to an HTTP GET against the admin health endpoint. |
| `gateway.controller.deployment.resources` | object | `{}` | CPU/memory resource requests and limits for the controller container. Intentionally empty — recommended values are provided as comments in `values.yaml`. |
| `gateway.controller.deployment.podSecurityContext` | object | `{}` | Pod-level security context for the controller Pod. |
| `gateway.controller.deployment.securityContext` | object | `{}` | Container-level security context for the controller container. |
| `gateway.controller.deployment.nodeSelector` | object | `{}` | Node selector constraints for scheduling controller Pods. |
| `gateway.controller.deployment.tolerations` | list | `[]` | Tolerations for scheduling controller Pods on tainted nodes. |
| `gateway.controller.deployment.affinity` | object | `{}` | Affinity and anti-affinity rules for the controller Pods. |

## Gateway Controller — HPA

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.hpa.enabled` | bool | `false` | Enable a HorizontalPodAutoscaler for the controller. Requires a PostgreSQL backend (`gateway.controller.storage.type: postgres`) when scaling beyond 1 replica. |
| `gateway.controller.hpa.minReplicas` | int | `1` | Minimum number of controller replicas maintained by the HPA. |
| `gateway.controller.hpa.maxReplicas` | int | `3` | Maximum number of controller replicas the HPA may scale to. |
| `gateway.controller.hpa.targetCPUUtilizationPercentage` | int | `80` | Target average CPU utilization (%) that triggers controller scale-out. |
| `gateway.controller.hpa.targetMemoryUtilizationPercentage` | string | `""` | Target average memory utilization (%) for scale-out. Empty disables memory-based scaling. |
| `gateway.controller.hpa.customMetrics` | list | `[]` | Additional custom or external HPA metrics. |
| `gateway.controller.hpa.behavior` | object | `{}` | HPA scale-up/scale-down behavior configuration. |

## Gateway Controller — Pod Disruption Budget

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.controller.podDisruptionBudget.enabled` | bool | `false` | Enable a PodDisruptionBudget for the controller to limit voluntary disruptions during cluster operations. |
| `gateway.controller.podDisruptionBudget.minAvailable` | int | `1` | Minimum number of controller Pods that must remain available. Mutually exclusive with `maxUnavailable`. |
| `gateway.controller.podDisruptionBudget.maxUnavailable` | string | `""` | Maximum number (or percentage) of controller Pods that may be unavailable simultaneously. Overrides `minAvailable` when set. |

## Gateway Runtime — Image & Service

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.gatewayRuntime.image.repository` | string | `"ghcr.io/wso2/api-platform/gateway-runtime"` | Container image repository for the gateway runtime (Envoy + policy engine). |
| `gateway.gatewayRuntime.image.tag` | string | `"1.1.0"` | Container image tag for the gateway runtime. |
| `gateway.gatewayRuntime.image.pullPolicy` | string | `"Always"` | Image pull policy for the gateway runtime. |
| `gateway.gatewayRuntime.imagePullSecrets` | list | `[]` | Image pull secrets specific to the runtime (merged with `imagePullSecrets` at chart root). |
| `gateway.gatewayRuntime.service.type` | string | `"LoadBalancer"` | Kubernetes Service type for the runtime. `LoadBalancer` exposes HTTP/HTTPS ports externally. Use `NodePort` if a cloud load balancer is unavailable. |
| `gateway.gatewayRuntime.service.annotations` | object | `{}` | Annotations applied to the runtime Service. Useful for cloud-provider load balancer configuration. |
| `gateway.gatewayRuntime.service.labels` | object | `{}` | Labels applied to the runtime Service. |
| `gateway.gatewayRuntime.service.ports.http` | int | `8080` | Service port for plaintext HTTP traffic. |
| `gateway.gatewayRuntime.service.ports.https` | int | `8443` | Service port for HTTPS traffic. |
| `gateway.gatewayRuntime.service.ports.envoyAdmin` | int | `9901` | Service port for the Envoy admin interface. Restrict access in production. |
| `gateway.gatewayRuntime.service.ports.policyEngineAdmin` | int | `9002` | Service port for the policy engine admin API. Must match `gateway.config.policy_engine.admin.port`. |
| `gateway.gatewayRuntime.service.ports.policyEngineMetrics` | int | `9003` | Service port for the policy engine Prometheus metrics endpoint. |

## Gateway Runtime — Policies

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.gatewayRuntime.policies.llmPricing.enabled` | bool | `true` | Mount the LLM pricing configuration into the runtime. |
| `gateway.gatewayRuntime.policies.llmPricing.configMapName` | string | `""` | Name of an existing ConfigMap containing the LLM pricing data. When empty, the chart uses its bundled default pricing ConfigMap. |

## Gateway Runtime — Deployment

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.gatewayRuntime.deployment.enabled` | bool | `true` | Deploy the gateway runtime. Set `false` to skip the runtime Deployment. |
| `gateway.gatewayRuntime.deployment.replicaCount` | int | `1` | Number of gateway runtime replicas. The runtime is stateless and safe to scale horizontally. |
| `gateway.gatewayRuntime.deployment.env.gatewayControllerHost` | string | `""` | Override the gateway controller hostname used by the runtime. Defaults to the in-cluster controller Service address when empty. |
| `gateway.gatewayRuntime.deployment.env.logLevel` | string | `"info"` | Log level for the runtime container entrypoint. |
| `gateway.gatewayRuntime.deployment.env.moesifKey` | string | `""` | API key for Moesif analytics integration. Leave empty to disable. |
| `gateway.gatewayRuntime.deployment.extraEnv` | list | `[]` | Additional environment variables for the runtime container. |
| `gateway.gatewayRuntime.deployment.extraEnvFrom` | list | `[]` | Additional environment variable sources for the runtime container. |
| `gateway.gatewayRuntime.deployment.extraVolumeMounts` | list | `[]` | Additional volume mounts for the runtime container. |
| `gateway.gatewayRuntime.deployment.extraVolumes` | list | `[]` | Additional volumes to attach to the runtime Pod. |
| `gateway.gatewayRuntime.deployment.labels` | object | `{}` | Labels added to the runtime Deployment. |
| `gateway.gatewayRuntime.deployment.annotations` | object | `{}` | Annotations added to the runtime Deployment. |
| `gateway.gatewayRuntime.deployment.podAnnotations` | object | `{}` | Annotations added to runtime Pods. |
| `gateway.gatewayRuntime.deployment.podLabels` | object | `{}` | Labels added to runtime Pods. |
| `gateway.gatewayRuntime.deployment.priorityClassName` | string | `""` | PriorityClass name assigned to runtime Pods. |
| `gateway.gatewayRuntime.deployment.livenessProbe` | object | `exec: ["health-check.sh"]` | Liveness probe for the runtime container. Defaults to executing the bundled `health-check.sh` script. |
| `gateway.gatewayRuntime.deployment.readinessProbe` | object | `exec: ["health-check.sh"]` | Readiness probe for the runtime container. Defaults to executing the bundled `health-check.sh` script. |
| `gateway.gatewayRuntime.deployment.resources` | object | `{}` | CPU/memory resource requests and limits for the runtime container. Intentionally empty — recommended values are provided as comments in `values.yaml`. |
| `gateway.gatewayRuntime.deployment.podSecurityContext` | object | `{}` | Pod-level security context for runtime Pods. |
| `gateway.gatewayRuntime.deployment.securityContext` | object | `{}` | Container-level security context for the runtime container. |
| `gateway.gatewayRuntime.deployment.nodeSelector` | object | `{}` | Node selector constraints for scheduling runtime Pods. |
| `gateway.gatewayRuntime.deployment.tolerations` | list | `[]` | Tolerations for scheduling runtime Pods on tainted nodes. |
| `gateway.gatewayRuntime.deployment.affinity` | object | `{}` | Affinity and anti-affinity rules for runtime Pods. |

## Gateway Runtime — HPA

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.gatewayRuntime.hpa.enabled` | bool | `false` | Enable a HorizontalPodAutoscaler for the gateway runtime. The runtime is stateless and well-suited for autoscaling. |
| `gateway.gatewayRuntime.hpa.minReplicas` | int | `1` | Minimum number of runtime replicas maintained by the HPA. |
| `gateway.gatewayRuntime.hpa.maxReplicas` | int | `5` | Maximum number of runtime replicas the HPA may scale to. |
| `gateway.gatewayRuntime.hpa.targetCPUUtilizationPercentage` | int | `70` | Target average CPU utilization (%) that triggers runtime scale-out. |
| `gateway.gatewayRuntime.hpa.targetMemoryUtilizationPercentage` | string | `""` | Target average memory utilization (%) for scale-out. Empty disables memory-based scaling. |
| `gateway.gatewayRuntime.hpa.customMetrics` | list | `[]` | Additional custom or external HPA metrics. |
| `gateway.gatewayRuntime.hpa.behavior` | object | `{}` | HPA scale-up/scale-down behavior configuration. |

## Gateway Runtime — Pod Disruption Budget

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `gateway.gatewayRuntime.podDisruptionBudget.enabled` | bool | `false` | Enable a PodDisruptionBudget for the runtime to limit voluntary disruptions during cluster maintenance. |
| `gateway.gatewayRuntime.podDisruptionBudget.minAvailable` | int | `1` | Minimum number of runtime Pods that must remain available. Mutually exclusive with `maxUnavailable`. |
| `gateway.gatewayRuntime.podDisruptionBudget.maxUnavailable` | string | `""` | Maximum number (or percentage) of runtime Pods that may be unavailable simultaneously. Overrides `minAvailable` when set. |
