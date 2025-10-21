# STS (Security Token Service)

OAuth 2.0 / OIDC server built on [Asgardeo Thunder](https://github.com/asgardeo/thunder) with integrated authentication UI.

## Quick Start

### Build & Run

```bash
# Build
docker build -t wso2/api-platform-sts:latest .

# Run
docker run -d -p 8090:8090 -p 9091:9091 wso2/api-platform-sts:latest

# Verify
curl -k https://localhost:8090/health/liveness  # Thunder
curl -k https://localhost:9091                  # Gate App
```

### Setup (Kickstart)

The kickstart script automates initial STS setup by creating an organization, user, and OAuth application.

```bash
# 1. Run kickstart script
./kickstart.sh

# 2. Review registration.yaml for credentials and OAuth endpoints
cat registration.yaml
```

**What it creates:**
- Organization with name and handle
- User with credentials (username/password)
- OAuth application with client_id and client_secret
- OAuth endpoints (authorize, token, userinfo)
- Example authorization URL for testing

**Configuration (Optional):**

Edit `inputs.yaml` before running kickstart to customize:
- Organization details (name, handle, description)
- User credentials (username, password, email)
- Application settings (name, client_id, redirect_uris)

If not provided, sensible defaults are used.

**Testing OAuth Flow:**

After kickstart, use the `example_auth_url` from `registration.yaml`:
1. Open the URL in a browser
2. Login with the user credentials from `registration.yaml`
3. Exchange the authorization code for tokens using the provided curl command

**Sample Application:**

A Node.js sample app automates the OAuth flow and displays tokens with decoded JWT claims:

```bash
cd sample-app
pnpm install
pnpm start
# Open https://localhost:3000
```

See [sample-app/README.md](sample-app/README.md) for details.

## Ports

- **8090** - Thunder OAuth server (HTTPS)
- **9091** - Gate App UI (HTTPS)

## Documentation

See [spec/](spec/) for detailed documentation:

- [Product Requirements](spec/prd.md)
- [Architecture](spec/architecture.md)
- [Design](spec/design.md)
- [Implementation](spec/impl.md)
