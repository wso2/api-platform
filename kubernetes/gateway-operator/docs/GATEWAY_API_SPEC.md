# Gateway and API Configuration Spec Design

## Overview

This document describes the design of `APIGateway` and `RestApi` CRDs, which support flexible API deployment strategies across different scoping mechanisms.

## Design Principles

1. **Flexibility**: Support multiple gateway selection strategies
2. **Kubernetes-native**: Follow standard Kubernetes patterns (similar to Ingress/IngressClass)
3. **Explicit and Implicit**: Support both explicit gateway references and automatic selection
4. **Multi-tenancy**: Enable team-based and namespace-based isolation

## Gateway Spec

### API Selection Strategies

A `APIGateway` can select APIs using three strategies:

#### 1. Cluster-Scoped Gateway

Accepts APIs from **any namespace** in the cluster.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: cluster-gateway
spec:
  apiSelector:
    scope: Cluster
```

**Use cases:**
- Central API gateway for the entire cluster
- Shared infrastructure for all teams
- Public-facing gateway serving all APIs

#### 2. Namespace-Scoped Gateway

Accepts APIs only from **specific namespaces**.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: team-gateway
spec:
  apiSelector:
    scope: Namespaced
    namespaces:
      - "team-a-apis"
      - "team-a-services"
```

**Use cases:**
- Team-specific gateways
- Environment isolation (dev/staging/prod)
- Tenant isolation in multi-tenant clusters

**Note:** If `namespaces` is empty, the gateway only accepts APIs from its own namespace.

#### 3. Label-Based Selection

Accepts APIs matching **specific label criteria**.

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: platform-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      gateway: "platform-gateway"
      environment: "production"
    matchExpressions:
      - key: team
        operator: In
        values: ["platform", "infrastructure"]
      - key: tier
        operator: NotIn
        values: ["experimental"]
```

**Use cases:**
- Fine-grained API selection
- Cross-namespace API grouping with specific characteristics
- Dynamic API routing based on metadata
- Multi-dimensional selection (team, environment, tier, etc.)

### Full Spec Example

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: production-gateway
  namespace: gateway-system
spec:
  # Gateway classification
  gatewayClassName: "production"
  
  # API selection strategy
  apiSelector:
    scope: LabelSelector
    matchLabels:
      environment: "production"
      gateway-class: "production"
  
  # Infrastructure configuration
  infrastructure:
    replicas: 3
    image: "wso2/gateway-controller:latest"
    routerImage: "envoyproxy/envoy:v1.28-latest"
    
    resources:
      requests:
        cpu: "1"
        memory: "2Gi"
      limits:
        cpu: "4"
        memory: "8Gi"
    
    nodeSelector:
      workload: "gateway"
    
    tolerations:
      - key: "dedicated"
        operator: "Equal"
        value: "gateway"
        effect: "NoSchedule"
    
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: gateway
            topologyKey: kubernetes.io/hostname
  
  # Control plane configuration
  controlPlane:
    host: "control-plane.gateway-operator-system:8443"
    tokenSecretRef:
      name: gateway-credentials
      key: token
    tls:
      enabled: true
      certSecretRef:
        name: gateway-tls
        key: tls.crt
  
  # Storage configuration
  storage:
    type: postgres
    connectionSecretRef:
      name: postgres-connection
      key: connection-string
```

### Status Fields

The `APIGateway` status tracks:

```yaml
status:
  phase: "Ready"  # Pending, Ready, Failed
  selectedAPIs: 42
  observedGeneration: 2
  lastUpdateTime: "2025-11-12T10:30:00Z"
  conditions:
    - type: Ready
      status: "True"
      reason: DeploymentReady
      message: "Gateway is ready with 3/3 replicas"
    - type: APIsDeployed
      status: "True"
      reason: AllAPIsDeployed
      message: "42 APIs successfully deployed"
```

## RestApi Spec

### Gateway Selection

An API can specify its gateway(s) in two ways:

#### 1. Explicit Gateway References

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api
  namespace: prod-apis
spec:
  # Deploy to multiple specific gateways
  gatewayRefs:
    - name: production-gateway
      namespace: gateway-system
    - name: backup-gateway
      namespace: gateway-system
  
  apiConfiguration:
    # WSO2 API configuration
```

**Behavior:** The API will **only** be deployed to the explicitly referenced gateways, regardless of gateway selectors.

**Use cases:**
- Critical APIs requiring specific gateway infrastructure
- Multi-region deployments (primary + failover gateways)
- High-availability setups
- Gradual rollout strategies

#### 2. Implicit Selection (Labels)

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: my-api
  namespace: prod-apis
  labels:
    environment: "production"
    gateway-class: "production"
    team: "platform"
spec:
  # No gatewayRef - relies on labels for selection
  apiConfiguration:
    # WSO2 API configuration
```

**Behavior:** The API will be deployed to any gateway whose `apiSelector` matches.

### Status Fields

```yaml
status:
  phase: "Deployed"  # Pending, Deployed, Failed
  deployedGateways:
    - "production-gateway"
    - "backup-gateway"
  observedGeneration: 1
  lastUpdateTime: "2025-11-12T10:30:00Z"
  conditions:
    - type: Deployed
      status: "True"
      reason: DeployedToGateways
      message: "API deployed to 2 gateways"
```

## Selection Logic

### How Gateway Selection Works

```
┌─────────────────────────────────────────────────────────────┐
│                    API Created/Updated                       │
└────────────────────────────┬────────────────────────────────┘
                             │
                             ▼
              ┌──────────────────────────┐
              │ Has explicit gatewayRef? │
              └──────────┬───────────────┘
                         │
         ┌───────────────┴───────────────┐
         │ YES                            │ NO
         ▼                                ▼
┌─────────────────────┐      ┌───────────────────────────┐
│ Deploy to that      │      │ Find all gateways whose   │
│ gateway only        │      │ apiSelector matches       │
└─────────────────────┘      └────────────┬──────────────┘
                                           │
                              ┌────────────┴────────────┐
                              │                         │
                              ▼                         ▼
                    ┌──────────────────┐    ┌──────────────────┐
                    │ Cluster scope:   │    │ Namespaced:      │
                    │ ALL gateways     │    │ Check namespace  │
                    └──────────────────┘    └──────────────────┘
                              │                         │
                              └────────────┬────────────┘
                                           │
                                           ▼
                              ┌────────────────────────┐
                              │ LabelSelector:         │
                              │ Check labels match     │
                              └────────────┬───────────┘
                                           │
                                           ▼
                              ┌────────────────────────┐
                              │ Deploy to all matching │
                              │ gateways               │
                              └────────────────────────┘
```

### Precedence Rules

1. **Explicit reference wins**: If `gatewayRef` is set, only that gateway is used
2. **Multiple matches allowed**: Without explicit reference, API can deploy to multiple gateways
3. **No match = no deployment**: If no gateway selects the API, it remains undeployed

## Use Case Examples

### Use Case 1: Multi-Environment Setup

```yaml
---
# Production gateway - cluster-scoped for prod namespace
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: prod-gateway
spec:
  gatewayClassName: "production"
  apiSelector:
    scope: Namespaced
    namespaces: ["production"]
  infrastructure:
    replicas: 3
---
# Staging gateway - cluster-scoped for staging
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: staging-gateway
spec:
  gatewayClassName: "staging"
  apiSelector:
    scope: Namespaced
    namespaces: ["staging"]
  infrastructure:
    replicas: 2
---
# Dev gateway - cluster-scoped for all dev namespaces
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: dev-gateway
spec:
  gatewayClassName: "development"
  apiSelector:
    scope: Namespaced
    namespaces: ["dev-team-a", "dev-team-b", "dev-team-c"]
  infrastructure:
    replicas: 1
```

### Use Case 2: Team-Based Isolation

```yaml
---
# Team Platform gateway
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: platform-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      team: "platform"
---
# Team Backend gateway
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: backend-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      team: "backend"
```

### Use Case 3: Tiered Gateway Architecture

```yaml
---
# Premium tier gateway (high resources)
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: premium-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      tier: "premium"
  infrastructure:
    replicas: 5
    resources:
      requests:
        cpu: "2"
        memory: "4Gi"
---
# Standard tier gateway
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: standard-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      tier: "standard"
  infrastructure:
    replicas: 2
```

### Use Case 4: Hybrid Selection

```yaml
---
# Gateway using complex label matching
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: APIGateway
metadata:
  name: hybrid-gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      environment: "production"
    matchExpressions:
      - key: team
        operator: In
        values: ["platform", "infrastructure", "core"]
      - key: tier
        operator: NotIn
        values: ["experimental", "deprecated"]
      - key: public-facing
        operator: Exists
```

## Migration Guide

### From Hardcoded to Configurable

**Before:**
- Gateway manifest was hardcoded
- No API selection logic
- Single gateway per cluster

**After:**
- Multiple gateways with different selectors
- Flexible API routing
- Support for multi-tenancy

### Migration Steps

1. **Create gateway configurations:**
   ```bash
   kubectl apply -f config/samples/api_v1_gateway.yaml
   ```

2. **Label existing APIs** (for label-based selection):
   ```bash
   kubectl label apiconfiguration my-api gateway=production-gateway -n prod-apis
   ```

3. **Or add explicit gateway references** to API specs

4. **Verify selection:**
   ```bash
   kubectl get apigateway production-gateway -o jsonpath='{.status.selectedAPIs}'
   ```

## Best Practices

1. **Use label-based selection for flexibility**
   - Allows dynamic API routing
   - Easier to reorganize APIs

2. **Use namespace-scoped for isolation**
   - Team-based separation
   - Environment separation

3. **Use cluster-scoped sparingly**
   - Only for truly global gateways
   - Consider security implications

4. **Use explicit references for critical APIs**
   - Ensures predictable deployment
   - Prevents accidental routing changes

5. **Monitor gateway status**
   - Check `selectedAPIs` count
   - Verify conditions in status

6. **Use meaningful labels**
   - `team`, `environment`, `tier`, `criticality`
   - Document your labeling strategy

## Troubleshooting

### API not deployed to any gateway

**Check:**
1. Does any gateway selector match the API?
2. If using `gatewayRef`, does the referenced gateway exist?
3. Check API and gateway namespaces
4. Verify labels if using label-based selection

**Debug:**
```bash
# List all gateways
kubectl get apigateways

# Check gateway status
kubectl describe apigateway <name>

# Check API status
kubectl describe apiconfiguration <name> -n <namespace>
```

### API deployed to wrong gateway

**Check:**
1. Verify `gatewayRef` if using explicit reference
2. Check label selector logic
3. Review namespace list for namespace-scoped gateways

### Multiple gateways selecting same API

This is **expected behavior** when using implicit selection. If you want exclusive deployment, use explicit `gatewayRef`.

## Future Enhancements

Potential future additions:

1. **Priority-based selection**: Gateway weight/priority when multiple match
2. **Gateway groups**: Logical grouping of gateways for load distribution
3. **Dynamic scaling**: Auto-scale gateways based on API load
4. **Gateway health checks**: Automatic failover to backup gateways
5. **Cross-cluster gateways**: Multi-cluster API deployment
