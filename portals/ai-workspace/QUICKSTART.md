# AI Workspace — Quick Start Guide

Get the AI Workspace running locally in under 5 minutes using Docker Compose.

---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) with the Compose plugin (`docker compose version`)
- Port **5380** and **9243** available on your machine

---

## Option A: Docker Compose (recommended)

This is the fastest path. Two containers start together — the AI Workspace UI (nginx) and the Platform API (Go backend).

### 1. Copy the config templates

```bash
cp configs/config-template.toml        configs/config.toml
cp configs/config-platform-api-template.toml  configs/config-platform-api.toml
```

### 2. Hash a password for your user

```bash
# Python (any OS)
python3 -c "import bcrypt; print(bcrypt.hashpw(b'yourpassword', bcrypt.gensalt(12)).decode())"

# Or with htpasswd (Apache utils)
htpasswd -bnBC 12 "" yourpassword | tr -d ':\n'
```

### 3. Generate a JWT secret key

```bash
openssl rand -hex 32
```

Copy the output — you'll paste it as `secret_key` in the next step.

### 4. Edit `configs/config-platform-api.toml`

Set the values marked as required:

```toml
[auth.jwt]
secret_key = "<output from step 3>"

[auth.file_based.organization]
id     = ""             # generate with: uuidgen
name   = "My Organization"
handle = "my-org"

[[auth.file_based.users]]
username      = "admin"
password_hash = "$2a$12$..."   # paste the hash from step 2
```

### 5. Start the stack

```bash
docker compose up -d
```

### 6. Open the workspace

Navigate to **https://localhost:5380** and sign in with the username and password you configured.

> **Browser trust warning?** Both services use a self-signed TLS cert by default. Open `https://localhost:9243/health` in your browser, click **Advanced → Proceed**, then return to the workspace. See [Custom TLS certificates](#custom-tls-certificates) to remove the warning permanently.


## Custom TLS certificates

Mount your own certificate to remove the browser trust warning.

1. Create a `certs/` directory next to `docker-compose.yaml`.
2. Place your certificate files there:
   ```
   certs/
   ├── ai-workspace.crt
   ├── ai-workspace.key
   ├── platform-api.crt
   └── platform-api.key
   ```
3. Uncomment the TLS volume lines in `docker-compose.yaml` under each service.
4. Restart: `docker compose up -d`

---

## Auth modes

| Mode | When to use |
|---|---|
| **Basic auth** (default) | Local or air-gapped setups — no external IDP needed |
| **OIDC** | Production deployments with Asgardeo, Thunder, or any OIDC-compliant provider |

To switch to OIDC, set `auth_mode = "oidc"` in `configs/config.toml` and fill in the `oidc_*` fields. See [`production/README.md`](production/README.md) for the full Asgardeo setup walkthrough.

---

## Next steps

- [Production setup guide](production/README.md) — Asgardeo OIDC, custom certificates, environment variables
- [Developer README](README.md) — frontend architecture, auth context API, Docker distribution internals
