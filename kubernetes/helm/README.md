# WSO2 API Platform Helm charts

Use the Helm charts in this folder to install the WSO2 API Platform product offerings on Kubernetes.

- This page introduces each offering and shows which charts to install for it.
- Each chart's own README has the full installation and configuration steps.

## Product offerings

The WSO2 API Platform has the following offerings.

| Offering | What you get | Charts to install |
| --- | --- | --- |
| Standalone AI Gateway | A gateway for AI traffic, such as requests to LLM providers | `gateway` |
| AI Gateway and AI Workspace | An AI Gateway that you manage from the AI Workspace console | `ai-workspace`, `gateway` |
| Standalone API Gateway | A gateway for API traffic | `gateway` |
| API Gateway and API Portal | An API Gateway, and a developer portal where consumers find and use your APIs | `api-portal`, `gateway` |
| Standalone API Portal | A developer portal and MCP Hub | `api-portal` |
| AI or API Gateway, AI Workspace, and API Portal | An AI or API Gateway, the AI Workspace, and the API Portal, sharing one Platform API | `ai-workspace` or `api-portal` with the other console added, and `gateway` |

## Components

Each offering is built from the following components.

- **Gateway.** Receives AI and API traffic and applies policies. The AI Gateway and the API Gateway use the same [`gateway`](gateway-helm-chart/README.md) chart and the same settings.
- **Platform API.** The shared control plane. The AI Workspace, the API Portal, and connected gateways use it.
- **AI Workspace.** Web console for managing AI resources such as LLM providers, MCP proxies, and AI gateways.
- **API Portal.** Developer portal and MCP Hub where consumers find and use APIs.

The [`ai-workspace`](ai-workspace-helm-chart/README.md) and [`api-portal`](api-portal-helm-chart/README.md) charts are umbrella charts.

- Each one installs the Platform API together with its console, in one step.
- They use the `platform-api`, `ai-workspace-ui`, and `api-portal-ui` charts in this folder, so you don't install those yourself.

The components work together in the following way.

- AI and API clients send their requests to the gateway.
- The gateway connects to the Platform API with a registration token and receives the APIs and proxies deployed to it.
- The AI Workspace connects to the Platform API to manage AI resources.
- The Platform API publishes APIs to the API Portal. The API Portal sends API key and subscription events back to the Platform API.

## Before you start

Make sure you have the following.

- A Kubernetes cluster, version 1.24 or later
- Helm 3.12 or later
- `kubectl`
- `openssl`, to generate encryption keys
- `htpasswd` or Docker, used by the AI Workspace and API Portal secret scripts
- cert-manager installed in your cluster, which issues the TLS certificates by default. See [Installing cert-manager](gateway-helm-chart/README.md#installing-cert-manager). To use your own TLS secrets instead, see each chart's README.

## Install an offering

Each section lists the steps for one offering. Install the charts in the order shown.

### Standalone AI Gateway or API Gateway

1. Install the `gateway` chart. Follow the steps in the [`gateway` README](gateway-helm-chart/README.md).

### AI Gateway and AI Workspace

1. Install the `ai-workspace` chart. Follow the steps in the [`ai-workspace` README](ai-workspace-helm-chart/README.md).
2. In the AI Workspace, go to **AI Gateways** and add a gateway. The AI Workspace copies the **Gateway Registration Token** for you.
3. Install the `gateway` chart with the following values. Follow the steps in the [`gateway` README](gateway-helm-chart/README.md).
   - Set `gateway.controller.controlPlane.host` to `ai-workspace-platform-api.ai-workspace.svc:9243`. This is the Platform API address when you use the default release and namespace names.
   - Set `gateway.controller.controlPlane.token.value` to the registration token.
   - For testing only, if the Platform API uses a self-signed certificate, also set `gateway.config.controller.controlplane.insecure_skip_verify` to `true`.
4. In the AI Workspace, check that the gateway status is **Active**.

> **Tip:** The **Kubernetes** tab on the gateway page shows a `helm install` command you can copy.

> **Note:** The AI Workspace shows the registration token only once. To get a new token, click **Reconfigure** on the gateway page.

### API Gateway and API Portal

1. Install the `api-portal` chart. Follow the steps in the [`api-portal` README](api-portal-helm-chart/README.md).
2. Register the gateway and get its registration token from the Platform API REST API.
   - Register the gateway with `POST /api/v0.9/gateways`. See [Register a new gateway](../../docs/rest-apis/platform-api/gateways.md#register-a-new-gateway).
   - Create the registration token with `POST /api/v0.9/gateways/{gatewayId}/tokens`. See the [Gateway Tokens API](../../docs/rest-apis/platform-api/gateway-tokens.md#rotate-gateway-token).
3. Install the `gateway` chart with the following values. Follow the steps in the [`gateway` README](gateway-helm-chart/README.md).
   - Set `gateway.controller.controlPlane.host` to `api-portal-platform-api.api-portal.svc:9243`. This is the Platform API address when you use the default release and namespace names.
   - Set `gateway.controller.controlPlane.token.value` to the registration token.
   - For testing only, if the Platform API uses a self-signed certificate, also set `gateway.config.controller.controlplane.insecure_skip_verify` to `true`.

### Standalone API Portal

1. Install the `api-portal` chart. Follow the steps in the [`api-portal` README](api-portal-helm-chart/README.md).

### AI or API Gateway, AI Workspace, and API Portal

To install these components as one product, use an umbrella chart and add the components you need to it as subcharts.

- You can use either umbrella chart, `ai-workspace` or `api-portal`. Start from one, and add the other console to it as a subchart.
- One release then installs the shared Platform API, the AI Workspace, and the API Portal.
- The steps below use the `ai-workspace` chart as an example.

1. Add the API Portal as a subchart of the `ai-workspace` chart. In `ai-workspace-helm-chart/Chart.yaml`, add this entry at the end of `dependencies`.

   ```yaml
     - name: api-portal-ui
       version: "1.0.0"
       repository: "oci://ghcr.io/wso2/api-platform/helm-charts"
       condition: api-portal-ui.enabled
   ```
   Then run `helm dependency update ./ai-workspace-helm-chart` to download the subchart.

2. Install the `ai-workspace` chart. Follow the steps in the [`ai-workspace` README](ai-workspace-helm-chart/README.md), with these changes.
   - When you generate the secrets by running the script, set `API_PORTAL=true`, so that the script also creates the API Portal secret.
   - When you install the chart, create a values file for each subchart in the `kubernetes/helm` folder. In each file, put the settings under the subchart name, such as `platform-api`, `ai-workspace-ui`, or `api-portal-ui`. Put settings shared by all subcharts under `global`.
     - In `values-api-portal-ui.yaml`, turn on the API Portal.

       ```yaml
       api-portal-ui:
         enabled: true
         config:
           platformApi:
             insecure: true   # For testing only, with a self-signed certificate
       ```

     - In `values-ai-workspace-ui.yaml`, set the Platform API address that gateways use. The AI Workspace shows this address in its gateway setup commands.

       ```yaml
       ai-workspace-ui:
         config:
           gateway:
             controlplaneHost: <platform-api-host>:9243
           controlPlane:
             tlsSkipVerify: true   # For testing only, with a self-signed certificate
       ```

     - In `values-global.yaml`, add any settings shared by all subcharts, such as image pull secrets.

       ```yaml
       global:
         imagePullSecrets:
           - <secret-name>
       ```

     Add each values file to the install command with `-f`, as in the following example. Leave out any file that you didn't create.

     ```bash
     helm upgrade --install ai-workspace ./ai-workspace-helm-chart -n ai-workspace \
       -f values-secrets.yaml \
       -f values-global.yaml \
       -f values-ai-workspace-ui.yaml \
       -f values-api-portal-ui.yaml
     ```

3. Open the AI Workspace and the API Portal.
   - Run `helm status ai-workspace -n ai-workspace` and note the `EXTERNAL-IP` of each console. On a local cluster, use `localhost`.
   - Open the AI Workspace at `https://<EXTERNAL-IP>:9643/ai-workspace/`.
   - Open the API Portal at `https://<EXTERNAL-IP>:9543/api-portal/default/views/default`.
   - Log in to each with the admin username and password that the secret script printed.
4. In the AI Workspace, go to **AI Gateways** and add a gateway. The AI Workspace copies the **Gateway Registration Token** for you.
5. Install the `gateway` chart with the following values. Follow the steps in the [`gateway` README](gateway-helm-chart/README.md).
   - Set `gateway.controller.controlPlane.host` to `ai-workspace-platform-api.ai-workspace.svc:9243`.
   - Set `gateway.controller.controlPlane.token.value` to the registration token.
   - For testing only, if the Platform API uses a self-signed certificate, also set `gateway.config.controller.controlplane.insecure_skip_verify` to `true`.
6. In the AI Workspace, check that the gateway status is **Active**.
