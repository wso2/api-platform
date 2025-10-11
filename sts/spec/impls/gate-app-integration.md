# Feature: Gate App Integration with STS

## Overview

This feature integrates the Gate App (Next.js authentication UI) with the STS Docker container, packaging both Thunder OAuth server and Gate App into a single deployable image.

## Git Commits

- `7a1fc12` - Add STS Docker configuration and build files
- `980477b` - Add Gate App build integration and oxygen-ui package
- `c6e9558` - Add STS component integration

## Motivation

To provide a complete authentication and authorization solution in a single Docker container, combining:
- Thunder OAuth 2.0 / OIDC server (backend)
- Gate App authentication UI (frontend)

This simplifies deployment and reduces infrastructure complexity for users.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│              Docker Container                       │
│                                                     │
│  ┌────────────────┐         ┌──────────────┐        │
│  │   Gate App     │         │   Thunder    │        │
│  │  (Next.js UI)  │◄────────┤   (Core)     │        │
│  │                │         │              │        │
│  │  - Login       │         │ - OAuth 2.0  │        │
│  │  - Register    │         │ - OIDC       │        │
│  │                │         │ - Token Mgmt │        │
│  │  Port: 9090    │         │ Port: 8090   │        │
│  └────────────────┘         └──────────────┘        │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## Implementation Details

### Multi-stage Docker Build

**Stage 1: Gate App Builder** (`node:20-alpine`)
- Installs PNPM 10 for package management
- Installs OpenSSL for certificate generation
- Copies workspace dependencies (oxygen-ui packages)
- Sets up PNPM workspace configuration
- Copies Gate App source code (src, public, scripts, configs)
- Installs dependencies: `pnpm install`
- Generates SSL certificates: `pnpm ensure-certificates`
- Builds Next.js app: `pnpm next_build` (standalone mode)
- Copies static assets and certificates to standalone output

**Stage 2: Thunder Runtime** (`ghcr.io/asgardeo/thunder:latest`)
- Installs Node.js 20 runtime (as root user)
- Switches back to `thunder` user for security
- Copies Gate App build from builder stage with proper ownership
- Copies startup script with execute permissions
- Exposes ports 8090 (Thunder) and 9090 (Gate App)
- Configures health check for Thunder
- Sets startup script as entrypoint

### Startup Process

**File**: `scripts/startup.sh`

The startup script orchestrates both services:

1. **Start Thunder** (background process)
   - Changes to `/opt/thunder` directory
   - Executes `./start.sh &` in background
   - Stores Thunder PID for monitoring
   - Waits 5 seconds for initialization

2. **Start Gate App** (foreground process)
   - Changes to `/opt/gate-app` directory
   - Executes `exec node server.js` in foreground
   - Keeps container alive while both services run

### Key Technical Decisions

1. **Workspace Dependencies**
   - Gate App depends on `@oxygen-ui/react` from Thunder monorepo
   - Source: Thunder repository at `frontend/packages/`
   - Must be COPIED (not symlinked) to STS build context
   - Docker doesn't follow external symlinks

2. **Permission Management**
   - Used `--chown=thunder:thunder` in COPY command
   - Avoids permission errors without root RUN commands
   - Maintains security by running as non-root user

3. **SSL Certificates**
   - Self-signed certificates generated during build
   - OpenSSL installed in builder stage
   - Certificates copied to standalone output

4. **Standalone Output**
   - Next.js standalone mode for minimal runtime
   - Includes only necessary dependencies
   - Reduces final image size

5. **No Environment Variables**
   - Services communicate via exposed ports
   - Simplified configuration
   - Port forwarding handles integration

## Configuration

### Ports

- **8090**: Thunder OAuth 2.0 / OIDC server (HTTPS)
- **9090**: Gate App authentication UI (HTTPS)

### Files

- **Dockerfile**: Multi-stage build definition
- **scripts/startup.sh**: Service orchestration script
- **packages/**: Workspace dependencies from Thunder

## Build & Run

### Build

```bash
cd sts
docker build -t wso2/api-platform-sts:latest .
```

### Run

```bash
docker run --rm -p 8090:8090 -p 9090:9090 wso2/api-platform-sts:latest
```

### Verify

```bash
# Thunder health check
curl -k https://localhost:8090/health/liveness

# Gate App
curl -k https://localhost:9090
```

## Challenges & Solutions

### Challenge 1: Workspace Package Not Found
**Problem**: Gate App couldn't find `@oxygen-ui/react` dependency
**Solution**: Copied packages directory from Thunder source, created pnpm-workspace.yaml

### Challenge 2: Permission Denied on server.key
**Problem**: Gate App couldn't read SSL certificate files
**Initial attempt**: `RUN chown` command failed with "Operation not permitted"
**Solution**: Used `--chown=thunder:thunder` in COPY command

### Challenge 3: Wrong Thunder Startup Path
**Problem**: `/opt/thunder/bin/thunder.sh` doesn't exist
**Solution**: Changed to `/opt/thunder/start.sh` based on Thunder's Dockerfile

### Challenge 4: OpenSSL Not Found
**Problem**: Certificate generation failed in builder
**Solution**: Added `apk add --no-cache openssl` to builder stage

### Challenge 5: Missing server.js
**Problem**: server.js not copied to standalone output
**Solution**: Added server.js to COPY command in builder stage

## Testing

1. Build Docker image successfully
2. Start container with both ports exposed
3. Verify Thunder is running (health check endpoint)
4. Verify Gate App is accessible (login page loads)
5. Test OAuth flow (see kickstart feature)

## Related Features

- [Initial Thunder Setup](./initial-thunder-setup.md) - Foundation for this integration
- [Kickstart Process](./kickstart-process.md) - Automated setup script

## Future Enhancements

- Health check for Gate App (currently only Thunder)
- Custom Thunder configuration mounting
- Environment variable configuration options
- Image size optimization
