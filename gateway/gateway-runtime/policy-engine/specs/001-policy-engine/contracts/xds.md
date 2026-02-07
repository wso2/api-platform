# xDS Policy Discovery Service Contract

**Feature**: 001-policy-engine | **Date**: 2025-11-18

## Overview

The Policy Engine exposes an xDS-based configuration API for dynamic policy chain updates without service restart.

## Service Definition

Custom xDS service for policy configuration discovery:

```protobuf
service PolicyDiscoveryService {
  // Streaming xDS protocol for policy configuration
  rpc StreamPolicyMappings(stream DiscoveryRequest) returns (stream DiscoveryResponse);

  // Delta xDS for incremental updates
  rpc DeltaPolicyMappings(stream DeltaDiscoveryRequest) returns (stream DeltaDiscoveryResponse);
}
```

## Resource Type

```
type.googleapis.com/envoy.policy.v1.PolicyChainConfig
```

## Configuration Format

```yaml
resources:
  - "@type": type.googleapis.com/envoy.policy.v1.PolicyChainConfig
    metadata_key: "api-v1-private"
    request_policies:
      - name: "jwtValidation"
        version: "v1.0.0"
        enabled: true
        parameters:
          jwksUrl: "https://auth.example.com/.well-known/jwks.json"
          issuer: "https://auth.example.com"
          audiences: ["https://api.example.com"]
      - name: "rateLimiting"
        version: "v1.0.0"
        enabled: true
        execution_condition: 'request.Method in ["POST", "PUT", "DELETE"]'
        parameters:
          requestsPerSecond: 100
          burstSize: 20
    response_policies:
      - name: "securityHeaders"
        version: "v1.0.0"
        enabled: true
```

## Version Tracking

xDS uses resource versions for atomic updates:
- Initial request: version_info = ""
- Response: version_info = "v1"
- Client ACK/NACK with version
- Server sends updates only when version changes

## Implementation Location

`src/kernel/xds.go` implements the xDS server.

## File-Based Fallback

For development/testing, Policy Engine supports file-based configuration:

```yaml
# configs/xds/routes.yaml
policy_chains:
  - metadata_key: "api-v1-public"
    request_policies: [...]
    response_policies: [...]
```

Loaded via `--config-file` flag instead of xDS streaming.

## Contract Tests

Located in `tests/contract/xds_test.go`

## References

- xDS protocol: https://www.envoyproxy.io/docs/envoy/v1.36.2/api-docs/xds_protocol
- go-control-plane: https://github.com/envoyproxy/go-control-plane
