## Quick Start

### Using Docker Compose (Recommended)

### Prerequisites

A Docker-compatible container runtime such as:

- Docker Desktop (Windows / macOS)
- Rancher Desktop (Windows / macOS)
- Colima (macOS)
- Docker Engine + Compose plugin (Linux)

Ensure `docker` and `docker compose` commands are available.

```bash
docker --version
docker compose version
```

Replace `${version}` with the API Platform Gateway release version you want to run.

```bash
# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/gateway/v1.2.0-beta/wso2apip-api-gateway-1.2.0-beta.zip

# Unzip the downloaded distribution.
unzip wso2apip-api-gateway-1.2.0-beta.zip


cd wso2apip-api-gateway-v1.2.0-beta/

# One-time setup: provision the HTTPS listener certificate, the encryption key,
# api-platform.env, and the gateway-controller admin credentials. setup.sh prints
# the admin password once — copy it now.
./scripts/setup.sh

# Export the admin credentials so the management-API calls below can authenticate.
# The username defaults to "admin"; use the password setup.sh just printed.
export ADMIN_USERNAME=admin
export ADMIN_PASSWORD='<the password scripts/setup.sh printed>'

# Start the complete stack
docker compose up -d

# Verify gateway controller admin endpoint is running
curl http://localhost:9094/api/admin/v0.9/health

# Deploy an API configuration
curl -X POST http://localhost:9090/api/management/v0.9/rest-apis \
  -u "$ADMIN_USERNAME:$ADMIN_PASSWORD" \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1
kind: RestApi
metadata:
  name: reading-list-api-v1.0
spec:
  displayName: Reading-List-API
  version: v1.0
  context: /reading-list/$version
  upstream:
    main:
      url: https://apis.bijira.dev/samples/reading-list-api-service/v1.0
  policies:
    - name: set-headers
      version: v1
      params:
        request:
          headers:
            - name: x-wso2-apip-gateway-version
              value: v1.0.0
        response:
          headers:
            - name: x-environment
              value: development
  operations:
    - method: GET
      path: /books
    - method: POST
      path: /books
    - method: GET
      path: /books/{id}
    - method: PUT
      path: /books/{id}
    - method: DELETE
      path: /books/{id}
EOF


# Test routing through the gateway
curl -i http://localhost:8080/reading-list/v1.0/books
curl -ik https://localhost:8443/reading-list/v1.0/books
```

### Setup Script (`scripts/setup.sh`)

The gateway **never auto-generates keys or certificates**.
Instead, `scripts/setup.sh` provisions everything the gateway needs before first start, and the server fails
closed with a descriptive error if a required key or certificate is missing. Run it once from the
distribution (or repo) root:

```bash
./scripts/setup.sh
```

It provisions, idempotently (existing files are kept unless `--force`):

| Artifact | Purpose |
|---|---|
| `listener-certs/default-listener.crt` / `.key` | Self-signed certificate for the router's HTTPS ingress listener (`:8443`). |
| `aesgcm-keys/default-aesgcm256-v1.bin` | AES-256 key for at-rest encryption of stored secrets (bind-mounted into the controller). |
| `api-platform.env` | Required runtime settings, loaded into both containers via docker-compose `env_file:` — `GATEWAY_CONTROLLER_HOST`, `LOG_LEVEL`, and the gateway-controller admin credentials (`APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME` and the bcrypt `APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH`). |

**Admin credentials.** The gateway-controller REST/management API is protected by basic auth.
Two sets of variables are involved, and `setup.sh` bridges them:

- **What you provide** — the plaintext inputs to `setup.sh` (also what you pass to `curl -u`):
  - `ADMIN_USERNAME` — defaults to `admin` (override via the environment or the interactive prompt).
  - `ADMIN_PASSWORD` — used if set; otherwise you are prompted; otherwise a strong random one is generated.
- **What `setup.sh` writes** into `api-platform.env` — the variables `config.toml` actually reads via its
  `{{ env }}` tokens (you normally never set these by hand):
  - `APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME` ← your `ADMIN_USERNAME`.
  - `APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_PASSWORD_HASH` ← the **bcrypt hash** of your `ADMIN_PASSWORD`.

The plaintext password is printed to the console **once** and never stored — copy it, and use it with the
username for `curl -u "$ADMIN_USERNAME:$ADMIN_PASSWORD"` against the management API.

For non-interactive use (CI), set the plaintext inputs up front:

```bash
ADMIN_USERNAME=admin ADMIN_PASSWORD='choose-a-strong-password' ./scripts/setup.sh
```

If the controller starts with the shipped `config.toml` while `APIP_GW_CONTROLLER_AUTH_BASIC_ADMIN_USERNAME`
/ `..._PASSWORD_HASH` are unset (i.e. you never ran `setup.sh`), it **refuses to start** rather than coming
up on an empty credential — so always run `setup.sh` first. To populate those two config variables by hand
instead, set the username and a **bcrypt hash** of the password (never the plaintext).

**Options:**

| Flag | Effect |
|---|---|
| `--force` | Regenerate the certificate and encryption key, and rewrite `api-platform.env` — this **rotates the admin password** too (a new one is generated/prompted and printed). |
| `--certs-only` | Generate only the listener TLS certificate (skip the encryption key, admin credentials, and `api-platform.env`). |
| `--help` | Print usage. |

Then start the stack:

```bash
docker compose up -d
```

#### How configuration is delivered

`config.toml` pulls values in only through explicit `{{ env "NAME" "default" }}` interpolation tokens, resolved at startup.
`setup.sh` writes those values into `api-platform.env`, which docker-compose loads into the containers
via `env_file:` (`format: raw`, `required: true` — so `docker compose up` fails fast if
`api-platform.env` is missing; run `./scripts/setup.sh` first). To change a setting, edit
`config.toml` directly or set the variable its token reads in `api-platform.env`.

#### Connecting to a WSO2 API Platform control plane (optional)

The gateway runs standalone by default. To register it with a control plane, add the following to
`api-platform.env` (both default to empty; `config.toml` reads them via `{{ env }}` tokens):

```bash
# api-platform.env
APIP_GW_CONTROLLER_CONTROLPLANE_HOST=your-platform-host:9243
APIP_GW_CONTROLLER_CONTROLPLANE_TOKEN=<registration-token-from-the-control-plane>
```

The registration token is issued by the control plane; `setup.sh` never generates it.

#### At-rest encryption

At-rest encryption of stored secrets is **enabled by default**. `setup.sh` generates the AES-256 key
(`aesgcm-keys/default-aesgcm256-v1.bin`) and the compose bind-mounts it into the controller. The key
is **required** at startup — the server never generates one and exits with a descriptive error if it
is missing. Rotate it with `./scripts/setup.sh --force`.

#### Moesif analytics (optional)

Set your Moesif application id in `api-platform.env` and enable the publisher in `config.toml`:

```bash
# api-platform.env
MOESIF_KEY=<your-moesif-application-id>
```

### Stopping the Gateway

When stopping the gateway, you have two options:

**Option 1: Stop runtime, keep data (persisted APIs and configuration)**
```bash
docker compose down
```
This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose up`, all your API configurations will be restored.

**Option 2: Complete shutdown with data cleanup (fresh start)**
```bash
docker compose down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted APIs or configuration.
