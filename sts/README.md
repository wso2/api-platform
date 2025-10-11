# STS (Security Token Service)

A containerized OAuth 2.0 / OIDC authentication and authorization service built on [Asgardeo Thunder](https://github.com/asgardeo/thunder).

## Overview

The STS provides OAuth 2.0 / OIDC capabilities with an integrated authentication UI (Gate App), packaged as a single Docker image for easy deployment.

## Current Status

**Phase 1: Basic Thunder Integration** ✅
**Phase 2: Gate App Build Integration** ✅
**Phase 3: Component Integration & Networking** ✅
**Phase 4: Pre-configuration & Testing** ✅

- Thunder OAuth 2.0 / OIDC server running on port 8090
- Gate App authentication UI running on port 3000
- Both services start automatically in single container
- Health checks configured for Thunder
- Kickstart script for easy organization/user/application setup

## Quick Start

### Prerequisites

- Docker installed on your system

### Build the Image

```bash
cd sts
docker build -t wso2/api-platform-sts:latest .
```

### Run the Container

```bash
docker run --rm -p 8090:8090 -p 9090:9090 wso2/api-platform-sts:latest
```

### Verify Installation

Check if both services are running:

**Thunder (OAuth Server):**
```bash
curl -k https://localhost:8090/health/liveness
```

**Gate App (Auth UI):**
```bash
curl -k https://localhost:9090
```

## Kickstart Script

The kickstart script automates the creation of an organization, user, and application in Thunder.

### Usage

1. **Start the STS container:**
   ```bash
   docker run -d --name sts-container -p 8090:8090 -p 9090:9090 wso2/api-platform-sts:latest
   ```

2. **Configure your setup** (optional):

   Edit `inputs.yaml` with your organization and user details, or use the defaults.

3. **Run the kickstart script:**
   ```bash
   cd sts
   ./kickstart.sh
   ```

4. **Review the output:**

   The script generates `registration.yaml` containing:
   - Organization ID and details
   - User credentials
   - Application client ID and secret
   - OAuth endpoints
   - Example authorization URL
   - Test commands

### Example Output

```yaml
organization:
  id: "d713e47e-0a92-4608-b8c6-f069fbd37805"
  name: "Acme Corporation"
  handle: "acme"

user:
  id: "1e7e977f-e956-41a8-b8d4-9b6b886ce49c"
  username: "admin"
  password: "Admin@123"
  email: "admin@acme.com"

application:
  id: "1f78eac6-e8d2-48f2-b037-e42f2d114e84"
  client_id: "management-portal-client"
  client_secret: "c531589f187567e2c8..."
  redirect_uris:
    - "https://localhost:3000/callback"

oauth_endpoints:
  authorize: "https://localhost:8090/oauth2/authorize"
  token: "https://localhost:8090/oauth2/token"
```

### Testing OAuth Flow

After running kickstart, test the OAuth flow:

1. **Open the authorization URL** (from `registration.yaml`):
   ```
   https://localhost:8090/oauth2/authorize?response_type=code&client_id=<client_id>&redirect_uri=https://localhost:3000/callback&scope=openid&state=random_state_123
   ```

2. **Login with user credentials** from `registration.yaml`

3. **Exchange code for token:**
   ```bash
   curl -k -X POST https://localhost:8090/oauth2/token \
     -u <client_id>:<client_secret> \
     -d "grant_type=authorization_code" \
     -d "code=<authorization_code>" \
     -d "redirect_uri=https://localhost:3000/callback"
   ```

## Configuration

### Ports

- **8090** - Thunder OAuth 2.0 / OIDC server (HTTPS)
- **9090** - Gate App authentication UI (HTTPS)

### Custom Configuration (Optional)

Mount a custom Thunder configuration file:

```bash
docker run --rm \
  -p 8090:8090 \
  -p 9090:9090 \
  -v $(pwd)/deployment.yaml:/opt/thunder/repository/conf/deployment.yaml \
  wso2/api-platform-sts:latest
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│              Docker Container                       │
│                                                     │
│  ┌────────────────┐         ┌──────────────┐       │
│  │   Gate App     │         │   Thunder    │       │
│  │  (Next.js UI)  │◄────────┤   (Core)     │       │
│  │                │         │              │       │
│  │  - Login       │         │ - OAuth 2.0  │       │
│  │  - Register    │         │ - OIDC       │       │
│  │                │         │ - Token Mgmt │       │
│  │  Port: 9090    │         │ Port: 8090   │       │
│  └────────────────┘         └──────────────┘       │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## Roadmap

- [x] Phase 1: Basic Thunder Integration
- [x] Phase 2: Gate App Build Integration
- [x] Phase 3: Component Integration & Networking
- [x] Phase 4: Pre-configuration & Testing
- [ ] Phase 5: Optimization & Documentation (Optional)

## Documentation

See the `spec/` directory for detailed documentation:

- [Product Requirements](spec/product/prd.md)
- [Architecture](spec/architecture/architecture.md)
- [Design](spec/design/design.md)
- [Implementation Guide](spec/impl/impl.md)

## Reference

Based on [Asgardeo Thunder](https://github.com/asgardeo/thunder)
