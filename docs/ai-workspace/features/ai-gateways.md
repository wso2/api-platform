# AI Gateways

An AI Gateway is a deployed runtime instance (Envoy + policy engine) that proxies traffic between AI consumers and upstream LLM services. The AI Workspace acts as the control plane: you register gateways here, and the Platform API pushes configuration to them.

## Concepts

- **Gateway runtime** — the actual proxy process, deployed separately (typically as a Kubernetes workload or Docker container).
- **Registration token** — a one-time-use secret the gateway runtime uses to connect to the Platform API on first start. Generated in the AI Workspace UI.
- **Connectivity status** — the UI shows whether each gateway is currently connected to the Platform API (Active / Inactive).

## Creating a Gateway

1. Navigate to **AI Gateways** in the left sidebar.
2. Click **Create Gateway**.
3. Enter a name and optional description.
4. Click **Create** — the gateway is registered and a registration token is generated.

## Obtaining Setup Instructions

After creating a gateway, the UI provides deployment artifacts tailored to your `controlplane_host` configuration:

- **`keys.env`** — environment file containing the registration token and control-plane endpoint, for use with Docker Compose.
- **Helm values** — equivalent values for Kubernetes / Helm-based deployments.

Download the appropriate artifact and pass it to your gateway runtime deployment.

## Deploying Resources to a Gateway

LLM Providers, LLM Proxies, and MCP Proxies are all deployed to gateways explicitly. After creating one of these resources, use the **Deploy** flow to select which gateways should serve it.

A resource can be deployed to multiple gateways simultaneously, allowing you to run e.g. the same LLM provider configuration across a production and staging gateway.

## Gateway Status

The **AI Gateways** list shows the current connectivity status of each registered gateway:

| Status | Meaning |
|--------|---------|
| Active | Gateway is connected and receiving configuration from the Platform API |
| Inactive | Gateway has not connected recently or the registration token has not been used |

## Editing and Deleting Gateways

- **Edit** — update the gateway name and description.
- **Delete** — removes the gateway registration. Any deployed resources are no longer pushed to this gateway. The running gateway runtime will lose its control-plane connection and stop receiving updates.
