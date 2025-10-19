# Policy Engine Integration

## Overview

Policy-first architecture where all functionality beyond basic HTTP routing (authentication, authorization, rate limiting, transformations) is implemented as pluggable policies applied at the operation level.

## Requirements

### Policy Architecture
- Policy Engine deployed as separate component (Standard tier only)
- Policies defined in API configuration at operation level (per HTTP method + path)
- Policy execution order: request policies → backend call → response policies
- Policy failure results in HTTP error response without calling backend service

### Authentication Policies
- **API Key Policy**: Extract API key from header/query parameter, validate against stored credentials
- **OAuth Policy**: Validate OAuth 2.0 bearer tokens, extract scopes and claims
- **JWT Policy**: Validate JWT signatures, verify expiry, extract claims for authorization
- **Basic Auth Policy**: Validate HTTP Basic Authentication credentials

### Authorization Policies
- **RBAC Policy**: Role-based access control using extracted user roles
- **Scope Validation**: Verify OAuth scopes match required operation scopes
- **Attribute-Based Policy**: Evaluate custom attributes for access decisions

### Rate Limiting Policies
- **Basic Rate Limiting**: Per-API rate limits enforced at Router (included in Basic tier)
- **Distributed Rate Limiting**: Redis-backed counters for quota management across multiple Router instances (Standard tier only)
- **Spike Arrest**: Smoothing traffic spikes by limiting requests per time window

### Policy Configuration Format
```yaml
operations:
  - method: GET
    path: /users/{id}
    requestPolicies:
      - name: apiKey
        params:
          headerName: X-API-Key
      - name: rateLimit
        params:
          limit: 100
          window: 60s
    responsePolicies:
      - name: headerModify
        params:
          add:
            X-Response-Time: "${duration}"
```

### Policy Engine Communication
- Router calls Policy Engine via HTTP/gRPC for policy evaluation (Standard tier)
- Policy decisions returned with allow/deny status and optional error messages
- Policy evaluation latency budget: p95 < 100ms to maintain overall request performance

## Success Criteria

- Policies correctly enforce authentication with 100% accuracy (valid credentials pass, invalid fail)
- Rate limiting policies prevent requests exceeding configured limits with <1% false positives
- Policy evaluation adds <100ms p95 latency to total request processing time
- Policy failures return appropriate HTTP status codes (401, 403, 429) with descriptive error messages
- Policy configuration changes apply via xDS without Policy Engine restarts
