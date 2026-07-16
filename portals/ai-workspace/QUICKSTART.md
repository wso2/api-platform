# AI Workspace — Quick Start Guide

Get the AI Workspace running locally in under 5 minutes using Docker Compose.

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin (`docker compose version`)
- Ports **5380** and **9243** available on your machine
- `curl` and `unzip` installed

---

## Get Started

### 1. Download AI Workspace

```bash
curl -sLO https://github.com/wso2/api-platform/releases/download/ai-workspace/v1.0.0-alpha/wso2apip-ai-workspace-1.0.0-alpha.zip && \
unzip wso2apip-ai-workspace-1.0.0-alpha.zip
```

### 2. Run the setup script

```bash
cd wso2apip-ai-workspace-1.0.0-alpha
./scripts/setup.sh
```

The script generates everything the stack requires: the encryption and JWT
signing keys and the admin login credentials (written to `api-platform.env`), and the
TLS certificate shared by both services (written to `resources/certificates/`).

> **Save the printed admin username and password** — the password is shown only
> once and stored nowhere. Rerun `./scripts/setup.sh --force` to rotate it.

### 3. Start the stack

```bash
docker compose up -d
```

### 4. Open the workspace

Navigate to **https://localhost:5380** and sign in with the admin credentials
printed by `setup.sh`.

> **Browser trust warning?** The generated TLS certificates are self-signed. Click **Advanced → Proceed** to continue, then return to the workspace. See [Custom TLS certificates](README.md#custom-tls-certificates) to remove the warning permanently.

---

## Try it out

### Step 1: Create an AI Gateway

An AI gateway is the runtime that processes and routes requests between your applications and LLM providers. You need at least one gateway before configuring providers or proxies.

1. Navigate to **AI Gateways** in the left navigation menu.
2. Click **+ Add AI Gateway**.
3. Fill in the **Name**, **URL**, and **Associated Environment**, then click **Add Gateway**.
4. Copy the **Gateway Registration Token** and follow the setup instructions to start the gateway runtime.
5. Once connected, the gateway status changes from **Inactive** to **Active**.

### Step 2: Configure an LLM Provider

An LLM provider connects the AI Workspace to an AI service platform such as OpenAI, Anthropic, or Azure OpenAI.

1. Navigate to **LLM > Service Provider**.
2. Click **+ Add New Provider** and select your provider type.
3. Fill in the **Name**, **Version**, and **API Key**, then click **Add Provider**.
4. Configure how applications authenticate when accessing this provider through the gateway.
5. Click **Deploy to Gateway** and select your active gateway.

---

## Next steps

- [Production setup guide](production/README.md) — Asgardeo OIDC, custom certificates, environment variables
- [Developer README](README.md) — frontend architecture, auth context API, Docker distribution internals
