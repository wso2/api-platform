# Gateway Configuration Architecture

## System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                          │
│                                                                     │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                   Gateways                      │  │
│  │                                                              │  │
│  │  ┌────────────────┐  ┌────────────────┐  ┌───────────────┐ │  │
│  │  │ Cluster GW     │  │ Namespace GW   │  │ Label-Based   │ │  │
│  │  │ scope: Cluster │  │ scope: NS      │  │ scope: Labels │ │  │
│  │  │ ↓ ALL APIs     │  │ ↓ Specific NS  │  │ ↓ Match Labels│ │  │
│  │  └────────────────┘  └────────────────┘  └───────────────┘ │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              ↓                                      │
│                    ┌─────────────────────┐                         │
│                    │  API Selector Logic │                         │
│                    └─────────────────────┘                         │
│                              ↓                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    RestApis                         │  │
│  │                                                              │  │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐      │  │
│  │  │ API-1        │  │ API-2        │  │ API-3        │      │  │
│  │  │ gatewayRef:  │  │ labels:      │  │ namespace:   │      │  │
│  │  │ → specific GW│  │ → match GW   │  │ → match NS   │      │  │
│  │  └──────────────┘  └──────────────┘  └──────────────┘      │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                              ↓                                      │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │              Deployed Gateway Infrastructure                 │  │
│  │                                                              │  │
│  │  ┌─────────────────────────────────────────────────────┐    │  │
│  │  │ Gateway Pods (Controllers + Routers)                │    │  │
│  │  │  ┌──────┐ ┌──────┐ ┌──────┐                         │    │  │
│  │  │  │ Pod1 │ │ Pod2 │ │ Pod3 │ ... with APIs deployed  │    │  │
│  │  │  └──────┘ └──────┘ └──────┘                         │    │  │
│  │  └─────────────────────────────────────────────────────┘    │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## Selection Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                  API Selection Decision Tree                    │
└─────────────────────────────────────────────────────────────────┘

                    RestApi Created
                             │
                             ▼
                ┏━━━━━━━━━━━━━━━━━━━━━━━┓
                ┃ Has explicit          ┃
                ┃ gatewayRef?           ┃
                ┗━━━━━━━━━━━━━━━━━━━━━━━┛
                       │         │
                  YES  │         │  NO
                       │         │
        ┌──────────────┘         └──────────────┐
        ▼                                       ▼
┌──────────────────┐                  ┌──────────────────┐
│ Deploy ONLY to   │                  │ Check ALL        │
│ referenced       │                  │ gateways for     │
│ gateway          │                  │ matching selector│
└──────────────────┘                  └──────────────────┘
        │                                       │
        │                              ┌────────┴────────┐
        │                              │                 │
        │                              ▼                 ▼
        │                    ┌──────────────┐  ┌──────────────┐
        │                    │ Cluster      │  │ Namespaced   │
        │                    │ Scope        │  │ Scope        │
        │                    │              │  │              │
        │                    │ ALL APIs     │  │ Check if NS  │
        │                    │ selected     │  │ in list      │
        │                    └──────────────┘  └──────────────┘
        │                              │                 │
        │                              │       ┌─────────┘
        │                              │       │
        │                              ▼       ▼
        │                    ┌────────────────────────┐
        │                    │ LabelSelector Scope    │
        │                    │                        │
        │                    │ Check matchLabels AND  │
        │                    │ matchExpressions       │
        │                    └────────────────────────┘
        │                              │
        │                              ▼
        │                    ┌────────────────────────┐
        │                    │ Collect all matching   │
        │                    │ gateways               │
        │                    └────────────────────────┘
        │                              │
        └──────────────────────────────┘
                       │
                       ▼
        ┌──────────────────────────────────┐
        │ Deploy API to selected gateway(s)│
        └──────────────────────────────────┘
                       │
                       ▼
        ┌──────────────────────────────────┐
        │ Update Status:                   │
        │ - API: deployedGateways[]        │
        │ - Gateway: selectedAPIs count    │
        └──────────────────────────────────┘
```

## Three Selection Modes Visualized

### 1. Cluster Scope
```
┌─────────────────────────────────────────┐
│          Cluster Gateway                │
│   scope: Cluster                        │
│   ↓ Selects from ALL namespaces         │
└─────────────────────────────────────────┘
                  │
      ┌───────────┼───────────┬───────────┐
      │           │           │           │
      ▼           ▼           ▼           ▼
  ┌──────┐   ┌──────┐    ┌──────┐    ┌──────┐
  │ NS-A │   │ NS-B │    │ NS-C │    │ NS-D │
  │ API1 │   │ API2 │    │ API3 │    │ API4 │
  └──────┘   └──────┘    └──────┘    └──────┘
     ✓          ✓           ✓           ✓
  All APIs selected regardless of namespace
```

### 2. Namespace Scope
```
┌─────────────────────────────────────────┐
│       Namespaced Gateway                │
│   scope: Namespaced                     │
│   namespaces: [NS-A, NS-C]              │
│   ↓ Selects from specific namespaces    │
└─────────────────────────────────────────┘
                  │
      ┌───────────┼───────────┬───────────┐
      │           │           │           │
      ▼           ▼           ▼           ▼
  ┌──────┐   ┌──────┐    ┌──────┐    ┌──────┐
  │ NS-A │   │ NS-B │    │ NS-C │    │ NS-D │
  │ API1 │   │ API2 │    │ API3 │    │ API4 │
  └──────┘   └──────┘    └──────┘    └──────┘
     ✓          ✗           ✓           ✗
  Only APIs in NS-A and NS-C selected
```

### 3. Label Selector Scope
```
┌─────────────────────────────────────────┐
│      Label-Based Gateway                │
│   scope: LabelSelector                  │
│   matchLabels: {env: prod}              │
│   ↓ Selects APIs with matching labels   │
└─────────────────────────────────────────┘
                  │
      ┌───────────┼───────────┬───────────┐
      │           │           │           │
      ▼           ▼           ▼           ▼
  ┌──────┐   ┌──────┐    ┌──────┐    ┌──────┐
  │ API1 │   │ API2 │    │ API3 │    │ API4 │
  │env:  │   │env:  │    │env:  │    │team: │
  │ prod │   │ dev  │    │ prod │    │ ops  │
  └──────┘   └──────┘    └──────┘    └──────┘
     ✓          ✗           ✓           ✗
  Only APIs with env=prod selected (any namespace)
```

## Multi-Gateway Deployment

```
API with Labels (no explicit gatewayRef)
labels: {env: prod, tier: premium}
                │
                │ Evaluated by ALL gateways
                │
    ┌───────────┼───────────┬───────────┐
    │           │           │           │
    ▼           ▼           ▼           ▼
┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
│ GW-1   │ │ GW-2   │ │ GW-3   │ │ GW-4   │
│ labels:│ │ labels:│ │ labels:│ │ labels:│
│ env=   │ │ env=   │ │ tier=  │ │ team=  │
│ prod   │ │ dev    │ │ premium│ │ backend│
└────────┘ └────────┘ └────────┘ └────────┘
    ✓          ✗          ✓          ✗

API deployed to GW-1 and GW-3 (both match)
Status: deployedGateways: ["GW-1", "GW-3"]
```

## Status Updates

```
┌─────────────────────────────────────────────────────────┐
│               Status Synchronization                    │
└─────────────────────────────────────────────────────────┘

Gateway                 RestApi
┌──────────────────┐                ┌──────────────────┐
│ status:          │                │ status:          │
│   phase: Ready   │ ◄────────────► │   phase: Deployed│
│   selectedAPIs:  │                │   deployedGWs:   │
│   - API-1        │                │   - gateway-1    │
│   - API-2        │                │   - gateway-2    │
│   - API-3        │                │                  │
│   conditions:    │                │   conditions:    │
│   - Ready: True  │                │   - Ready: True  │
└──────────────────┘                └──────────────────┘
         │                                    │
         │                                    │
         └──────────┬────────────────────────┘
                    │
                    ▼
         ┌────────────────────┐
         │ Operator watches   │
         │ and reconciles     │
         │ both resources     │
         └────────────────────┘
```

## Real-World Example: Multi-Environment Setup

```
┌───────────────────────────────────────────────────────────────┐
│                    Production Environment                     │
└───────────────────────────────────────────────────────────────┘

├─ prod-gateway (Gateway)
│  └─ scope: Namespaced
│     namespaces: ["production"]
│     replicas: 3
│     resources: high
│
├─ production namespace
│  ├─ api-users (RestApi)
│  ├─ api-orders (RestApi)
│  └─ api-payments (RestApi)
│     └─ All deployed to prod-gateway

┌───────────────────────────────────────────────────────────────┐
│                    Staging Environment                        │
└───────────────────────────────────────────────────────────────┘

├─ staging-gateway (Gateway)
│  └─ scope: Namespaced
│     namespaces: ["staging"]
│     replicas: 2
│     resources: medium
│
├─ staging namespace
│  ├─ api-users (RestApi)
│  ├─ api-orders (RestApi)
│  └─ api-payments (RestApi)
│     └─ All deployed to staging-gateway

┌───────────────────────────────────────────────────────────────┐
│                  Development Environment                      │
└───────────────────────────────────────────────────────────────┘

├─ dev-gateway (Gateway)
│  └─ scope: LabelSelector
│     matchLabels: {env: dev}
│     replicas: 1
│     resources: low
│
├─ Multiple dev namespaces
│  ├─ dev-team-a
│  │  └─ api-feature-x (labels: env=dev) → dev-gateway ✓
│  ├─ dev-team-b
│  │  └─ api-feature-y (labels: env=dev) → dev-gateway ✓
│  └─ dev-team-c
│     └─ api-experiment (labels: env=test) → NOT selected ✗
```

## Component Interaction

```
┌────────────────────────────────────────────────────────────┐
│                     Operator Components                     │
└────────────────────────────────────────────────────────────┘

┌─────────────────────┐         ┌──────────────────────┐
│ Gateway│         │ RestApi     │
│ Controller          │         │ Controller           │
│                     │         │                      │
│ - Watches GW CRDs   │         │ - Watches API CRDs   │
│ - Selects APIs      │◄───────►│ - Finds GWs          │
│ - Deploys infra     │         │ - Triggers deploy    │
│ - Updates status    │         │ - Updates status     │
└─────────────────────┘         └──────────────────────┘
         │                               │
         └───────────┬───────────────────┘
                     │
                     ▼
         ┌────────────────────┐
         │  API Selector      │
         │  (helper package)  │
         │                    │
         │  - Selection logic │
         │  - Label matching  │
         │  - NS filtering    │
         └────────────────────┘
                     │
                     ▼
         ┌────────────────────┐
         │  Manifest Applier  │
         │  (k8sutil package) │
         │                    │
         │  - Apply K8s res.  │
         │  - Set ownership   │
         └────────────────────┘
```

All diagrams show the flexible, multi-mode gateway selection system!
