# FR2: xDS Server Implementation

## Overview

The Gateway-Controller implements an Envoy xDS v3 server using the State-of-the-World (SotW) protocol. It translates API configurations from in-memory maps into Envoy resources (LDS, RDS, CDS) and pushes complete configuration snapshots to the Router within 5 seconds of any change.

## Requirements

### xDS Protocol
- Implement xDS v3 API (not deprecated v2)
- Use SotW (State-of-the-World) protocol variant
- Support Listener Discovery Service (LDS)
- Support Route Discovery Service (RDS)
- Support Cluster Discovery Service (CDS)
- gRPC server on port 18000

### Snapshot Management
- Generate complete configuration snapshots on each change
- Monotonically increasing snapshot versions (integers)
- Include ALL API configurations in each snapshot (SotW approach)
- Use go-control-plane snapshot cache for distribution

### Configuration Translation
- Read from in-memory maps (not database) for performance
- Translate each API config to Envoy resources:
  - Listener: HTTP connection manager on port 8081
  - Routes: Per-operation method + path matching
  - Clusters: Backend upstream services
- Include access log configuration in all Listeners

### Performance
- Push updates to Router within 5 seconds of configuration change
- Handle 100+ API configurations without degradation
- Support concurrent Router connections (future multi-Router)

### Resilience
- Handle Router disconnections gracefully
- Retain snapshot cache for reconnecting Routers
- Support Router startup before Controller (wait with backoff)

## Success Criteria

- Router receives configuration within 5 seconds of change
- Complete xDS snapshot includes all deployed APIs
- Router successfully applies configuration (verify via admin API)
- In-flight requests complete during configuration updates
- Reconnecting Routers receive latest configuration

## User Scenarios

**Scenario 1**: Initial Router connection
```
1. Router starts and connects to xDS server (port 18000)
2. Controller generates snapshot from in-memory maps
3. Router receives LDS, RDS, CDS resources
4. Router begins serving traffic on port 8081
```

**Scenario 2**: Configuration update
```
1. User deploys new API configuration via REST API
2. Controller updates in-memory maps + database
3. Controller generates new snapshot (version N+1)
4. Controller pushes snapshot to Router via xDS
5. Router applies new configuration within 5 seconds
6. New API routes become active
```

**Scenario 3**: Router reconnection
```
1. Router loses connection to xDS server
2. Router continues serving with last known configuration
3. Network restored, Router reconnects
4. Controller pushes latest snapshot (version N)
5. Router updates to current configuration
```

## xDS Resource Structure

### Listener (LDS)
```yaml
name: listener_0
address: 0.0.0.0:8081
filter_chains:
  - filters:
    - name: envoy.filters.network.http_connection_manager
      typed_config:
        stat_prefix: ingress_http
        route_config_name: route_config_0
        http_filters:
          - name: envoy.filters.http.router
        access_log:
          - name: envoy.access_loggers.file
            typed_config:
              path: /dev/stdout
              json_format: {...}
```

### Route (RDS)
```yaml
name: route_config_0
virtual_hosts:
  - name: petstore_v1
    domains: ["*"]
    routes:
      - match:
          prefix: /petstore/v1
          path: /petstore/v1/pets
          method: GET
        route:
          cluster: cluster_petstore_backend
```

### Cluster (CDS)
```yaml
name: cluster_petstore_backend
type: STRICT_DNS
load_assignment:
  cluster_name: cluster_petstore_backend
  endpoints:
    - lb_endpoints:
      - endpoint:
          address:
            socket_address:
              address: api.example.com
              port_value: 443
```

## Implementation Notes

- Use github.com/envoyproxy/go-control-plane/pkg/cache/v3 for snapshot cache
- Use github.com/envoyproxy/go-control-plane/pkg/server/v3 for xDS gRPC server
- Node ID: "gateway-router" (configured in Router bootstrap)
- Snapshot version: Atomic counter incremented on each change
- Translation logic in `pkg/xds/translator.go`
- Read from in-memory maps in `pkg/storage/memory.go`
