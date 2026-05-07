# API Platform Kubernetes Gateway (V2)

API Platform Kubernetes Gateway (V2) is the latest evolution of the WSO2 APK Kubernetes gateway experience.  
If you are coming from WSO2 APK `1.3.0`, treat this as the successor track with a broader API Platform integration model and improved Kubernetes-native workflows.

- Previous generation (reference): [WSO2 APK 1.3.0 docs](https://apk.docs.wso2.com/en/latest/)
- Current generation: API Platform Kubernetes Gateway V2 (this documentation set)

## What Is New in V2

- Unified API Platform gateway model for both standalone and operator-driven deployments.
- Native support for Kubernetes Operator workflows and GitOps-style resource management.
- Flexible API exposure model:
  - API Platform CRDs (`APIGateway`, `RestApi`, and related CRDs)
  - Kubernetes Gateway API (`GatewayClass`, `Gateway`, `HTTPRoute`, optional `APIPolicy`)
- Improved extensibility for policies and secret/config-based parameter references.

## Deployment Modes

Choose one of the following modes based on your operational model.

### 1) Kubernetes Standalone Mode

Use this mode when you only need the gateway runtime and manage it directly via the gateway Helm chart.

- Installs gateway components without the gateway operator.
- Best for simpler setups or environments where operator lifecycle management is not required.
- Guide: [`kubernetes-standalone.md`](./kubernetes-standalone.md)

### 2) Kubernetes Operator Mode

Use this mode when you want operator-managed lifecycle, declarative resources, and automation.

- Installs the Gateway Operator via Helm.
- Supports:
  - API Platform CRD path (`APIGateway` + `RestApi`)
  - Kubernetes Gateway API path (`Gateway` + `HTTPRoute`)
- Guide: [`gateway-operator.md`](./gateway-operator.md)

## How to Choose a Mode

- Choose **Standalone Mode** if your priority is a lightweight gateway runtime install with Helm-managed values.
- Choose **Operator Mode** if your priority is Kubernetes-native reconciliation, GitOps workflows, multi-resource lifecycle automation, and CRD-based API management.

## Recommended Reading Order

1. Read this overview (`kubernetes-gateway-v2.md`)
2. Pick your mode:
   - Standalone: [`kubernetes-standalone.md`](./kubernetes-standalone.md)
   - Operator: [`gateway-operator.md`](./gateway-operator.md)