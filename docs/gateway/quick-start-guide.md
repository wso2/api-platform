# Quick start guide

This guide takes you from a downloaded distribution to a large language model (LLM) request routed through the API Platform AI Gateway.

## Prerequisites

Use one of these Docker-compatible container runtimes:

- [Docker Desktop](https://docs.docker.com/desktop/) (Windows / macOS)
- [Podman Desktop](https://podman-desktop.io/) or [Podman](https://podman.io/docs/installation) (Windows / macOS / Linux)
- [Rancher Desktop](https://rancherdesktop.io/) (Windows / macOS)
- [Colima](https://github.com/abiosoft/colima) (macOS)
- [Docker Engine](https://docs.docker.com/engine/install/) and [Compose plugin](https://docs.docker.com/compose/install/linux/) (Linux)

These examples use `docker compose` and a Linux or macOS shell. If you use another Compose-compatible runtime, use the equivalent commands. On Windows, run them from Git Bash or WSL; the distribution also ships `scripts/setup.ps1` for PowerShell.

Verify the commands for your runtime are available. For Docker:

```bash
docker --version
docker compose version
```

To call an LLM through the gateway, you also need an API key from the LLM service. This guide uses OpenAI.

## Start the gateway

The commands below use version `1.2.0`. Substitute the AI Gateway release version you want to run in the download URL, the archive name, and the directory name.

```bash
# Download distribution.
curl -fL -o wso2apip-ai-gateway-1.2.0.zip https://github.com/wso2/api-platform/releases/download/ai-gateway/v1.2.0/wso2apip-ai-gateway-1.2.0.zip

# Unzip the downloaded distribution.
unzip wso2apip-ai-gateway-1.2.0.zip

cd wso2apip-ai-gateway-1.2.0/

# Run the one-time setup. This provisions the AES-256 at-rest encryption key, the router
# HTTPS listener certificate, api-platform.env, and the gateway-controller admin credentials.
# It prints the admin password once — copy it.
./scripts/setup.sh

# Export the admin credentials so the management API calls below can authenticate.
# The username defaults to "admin"; use the password setup.sh just printed.
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD='<the password scripts/setup.sh printed>'

# Start the complete stack in the background
docker compose up -d

# Verify gateway controller admin endpoint is running.
# The stack takes a few seconds to start; retry if the first call fails.
curl http://localhost:9094/api/admin/v1/health
```

`docker compose up -d` runs the stack detached, so you can run the remaining commands in the same shell. To follow the container logs, run `docker compose logs -f` in another terminal.

> [!TIP]
> **Port 8080, 8443, 9090, or 9094 already taken?**
>
> If the start command fails with a port binding error, find what is already listening, for example with `lsof -nP -iTCP:8080 -sTCP:LISTEN`. Stop the conflicting service, or change the host-side value of the relevant `ports:` mapping in `docker-compose.yaml` and use the remapped port in the commands on this page.

> [!TIP]
> **Customizing configuration**
>
> The setup script writes `api-platform.env`, which Docker Compose loads into the containers through `env_file`. To change the storage backend, connect to a control plane, or tune other settings, edit that file or the `config.toml` interpolation tokens directly.

## Deploy an LLM provider

As a platform administrator, deploy an LLM provider for the vendor whose API key you hold. Replace `<openai-api-key>` with your OpenAI API key, keeping the `Bearer ` prefix.

```bash
curl -X POST http://localhost:9090/api/management/v1/llm-providers \
  -H "Content-Type: application/yaml" \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProvider
metadata:
  name: openai-provider
spec:
  displayName: OpenAI Provider
  version: v1.0
  template: openai
  context: /openai/latest
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: Bearer <openai-api-key>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
EOF
```

Other vendors follow the same shape with a different `template` (see the template list in the [overview](README.md#llm-provider-template)), upstream URL, and auth header. Anthropic, for example, uses `template: anthropic`, `https://api.anthropic.com`, and an `x-api-key` header with no prefix.

Invoke the provider through the gateway:

```bash
curl -X POST https://localhost:8443/openai/latest/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }' -k
```

> [!NOTE]
> **Why these commands pass `-k`**
>
> The `-k` flag tells `curl` to skip TLS certificate verification. The router presents the self-signed listener certificate that `setup.sh` generates, and no certificate authority trusts it. Outside local testing, give the router a certificate from a trusted certificate authority and remove `-k`.

## Deploy an LLM proxy that consumes the provider

As an AI developer, deploy an LLM proxy that consumes the OpenAI provider the platform administrator deployed above:

```bash
curl -X POST http://localhost:9090/api/management/v1/llm-proxies \
  -H "Content-Type: application/yaml" \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1
kind: LlmProxy
metadata:
  name: openai-assistant
spec:
  displayName: OpenAI Assistant
  version: v1.0
  context: /assistant
  provider:
    id: openai-provider
  policies: []
EOF
```

Invoke the proxy:

```bash
curl -X POST "https://localhost:8443/assistant/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }' -k
```

## Stop the gateway

To stop the containers and keep the `controller-data` volume, so your configuration is restored on the next `docker compose up`:

```bash
docker compose down
```

To stop the containers and remove the `controller-data` volume, for a clean start next time:

```bash
docker compose down -v
```

## Next steps

- Put an A2A agent behind the gateway: [A2A agent quick start guide](agent-governance/quick-start-guide.md)
- Learn the artifacts a request passes through: [AI Gateway overview](README.md#how-it-works)
- Add guardrails to a proxy, such as [PII masking](https://wso2.com/api-platform/policy-hub/policies/pii-masking-regex) or a [JSON schema guardrail](https://wso2.com/api-platform/policy-hub/policies/json-schema-guardrail)
- Browse the management API: [Gateway REST APIs](../rest-apis/gateway/README.md)
