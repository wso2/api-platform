# Gateway Configuration Quick Reference

## Gateway Selection Modes

| Mode | Scope Value | Selects APIs from | Use Case |
|------|------------|-------------------|----------|
| **Cluster** | `Cluster` | All namespaces | Central gateway for entire cluster |
| **Namespaced** | `Namespaced` | Specific namespaces | Team/environment isolation |
| **Label-based** | `LabelSelector` | APIs matching labels | Fine-grained, flexible selection |

## Quick Examples

### Cluster-Scoped Gateway
```yaml
spec:
  apiSelector:
    scope: Cluster
```
✅ Accepts ALL APIs from ANY namespace

### Namespace-Scoped Gateway
```yaml
spec:
  apiSelector:
    scope: Namespaced
    namespaces:
      - "team-a"
      - "team-b"
```
✅ Only accepts APIs from "team-a" and "team-b" namespaces

### Label-Based Gateway
```yaml
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      gateway: "production"
      environment: "prod"
```
✅ Only accepts APIs with matching labels

## API Selection Methods

### Method 1: Explicit References (Recommended for critical APIs)
```yaml
# API spec - deploy to multiple gateways
spec:
  gatewayRefs:
    - name: production-gateway
    - name: backup-gateway
```
🎯 Deploys to **specific gateways only**  
✅ Supports multiple gateways for HA/multi-region

### Method 2: Labels (Recommended for flexibility)
```yaml
# API metadata
metadata:
  labels:
    gateway: "production"
    environment: "prod"
```
🎯 Deploys to **any matching gateway**

### Method 3: Namespace (Simplest)
```yaml
# API metadata
metadata:
  namespace: "production"
```
🎯 Deploys to gateways selecting this namespace

## Common Patterns

### Production Setup
```yaml
# prod-gateway.yaml
spec:
  gatewayClassName: "production"
  apiSelector:
    scope: Namespaced
    namespaces: ["production"]
  infrastructure:
    replicas: 3
```

### Team-Based
```yaml
# team-gateway.yaml
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      team: "platform"
```

### Multi-Tier
```yaml
# premium-gateway.yaml
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels:
      tier: "premium"
  infrastructure:
    replicas: 5
    resources:
      requests: {cpu: "2", memory: "4Gi"}
```

## Label Selector Operators

| Operator | Meaning | Example |
|----------|---------|---------|
| `In` | Value in list | `team In [platform, backend]` |
| `NotIn` | Value not in list | `tier NotIn [experimental]` |
| `Exists` | Label exists | `public-facing Exists` |
| `DoesNotExist` | Label doesn't exist | `internal DoesNotExist` |

## Typical Labels

```yaml
metadata:
  labels:
    # Team ownership
    team: "platform"
    
    # Environment
    environment: "production"
    
    # Gateway routing
    gateway: "production-gateway"
    gateway-class: "production"
    
    # API tier/SLA
    tier: "premium"
    sla: "gold"
    
    # Visibility
    public-facing: "true"
    internal: "false"
    
    # Lifecycle
    stage: "stable"
    version: "v2"
```

## Status Checking

```bash
# Check gateway status
kubectl get gateway <name> -o yaml

# See selected API count
kubectl get gateway <name> -o jsonpath='{.status.selectedAPIs}'

# Check API deployment status
kubectl get apiconfiguration <name> -n <ns> -o jsonpath='{.status.deployedGateways}'

# List all APIs for a gateway
kubectl get restapis --all-namespaces -l gateway=production-gateway
```

## Decision Tree

```
Need to select APIs for a gateway?
│
├─ Want ALL APIs in cluster?
│  └─ Use scope: Cluster
│
├─ Want APIs from specific namespaces?
│  └─ Use scope: Namespaced
│     └─ List namespaces in spec
│
└─ Want APIs matching specific criteria?
   └─ Use scope: LabelSelector
      ├─ Simple match? → Use matchLabels
      └─ Complex match? → Use matchExpressions
```

## Common Commands

```bash
# Create gateway
kubectl apply -f gateway.yaml

# Create API
kubectl apply -f api.yaml

# List gateways
kubectl get gateways

# List APIs
kubectl get restapis -A

# Check gateway-API relationship
kubectl describe gateway <name>
kubectl describe apiconfiguration <name> -n <namespace>

# Label an API for selection
kubectl label apiconfiguration <name> gateway=prod -n <namespace>

# Remove label
kubectl label apiconfiguration <name> gateway- -n <namespace>
```

## Troubleshooting

| Problem | Check | Solution |
|---------|-------|----------|
| API not deployed | Gateway selectors | Add matching labels or gatewayRef |
| Wrong gateway | Label selectors | Fix labels or use explicit gatewayRef |
| Multiple gateways | Expected? | Use gatewayRef for exclusive routing |
| No APIs selected | Gateway scope | Verify namespaces or labels |

## Examples by Use Case

### 1. Dev/Staging/Prod Separation
```yaml
# Prod
spec:
  apiSelector: {scope: Namespaced, namespaces: ["prod"]}

# Staging  
spec:
  apiSelector: {scope: Namespaced, namespaces: ["staging"]}

# Dev
spec:
  apiSelector: {scope: Namespaced, namespaces: ["dev-*"]}
```

### 2. Public vs Internal APIs
```yaml
# Public gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels: {visibility: "public"}

# Internal gateway
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels: {visibility: "internal"}
```

### 3. Team Isolation
```yaml
# Team A
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels: {team: "team-a"}

# Team B
spec:
  apiSelector:
    scope: LabelSelector
    matchLabels: {team: "team-b"}
```

## See Also

- Full spec documentation: `docs/GATEWAY_API_SPEC.md`
- Sample configurations: `config/samples/`
- Operator configuration: `docs/CONFIGURATION.md`
