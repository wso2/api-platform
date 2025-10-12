# Feature: Initial Thunder Setup

## Overview

Basic Docker container setup using Thunder OAuth 2.0 / OIDC server as the foundation for STS.

## Git Commits

- `7a1fc12` - Add STS Docker configuration and build files
- `7a0fb01` - Add Thunder deployment configuration

## Motivation

Establish the foundation for STS by packaging Thunder as a standalone Docker container before adding additional components like Gate App.

## Implementation Details

### Dockerfile

Simple single-stage build using Thunder as the base image with custom deployment configuration:

```dockerfile
FROM ghcr.io/asgardeo/thunder:latest

EXPOSE 8090
COPY --chown=thunder:thunder conf/deployment.yaml /opt/thunder/repository/conf/deployment.yaml
COPY --chmod=755 scripts/startup.sh /opt/sts/startup.sh
WORKDIR /opt/sts
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD curl -k -f https://localhost:8090/health/liveness || exit 1
CMD ["/opt/sts/startup.sh"]
```

### Configuration

**File**: `conf/deployment.yaml`

Custom Thunder configuration file copied into the image at `/opt/thunder/repository/conf/deployment.yaml`. 

**Purpose**: Fixes CORS issue between Gate App and Thunder by setting `gate_client.hostname: "localhost"` instead of Thunder's default `"0.0.0.0"`. Without this, when users are redirected to Gate App for login, the Gate App's API calls to Thunder fail because Thunder only allows CORS from localhost by default.

### Startup Script

**File**: `scripts/startup.sh`

Simple script that launches Thunder:

```bash
#!/bin/bash
set -e
cd /opt/thunder
exec ./start.sh
```

**Key Points**:
- Changes to Thunder directory (`/opt/thunder`)
- Executes Thunder's startup script (`./start.sh`)
- Uses `exec` to replace shell process with Thunder process

### Health Check

- **Endpoint**: `https://localhost:8090/health/liveness`
- **Interval**: 30 seconds
- **Timeout**: 10 seconds
- **Start period**: 60 seconds (allows Thunder to initialize)
- **Retries**: 3
- **Flag**: `-k` to skip SSL certificate verification (self-signed cert)

## Configuration

### Ports

- **8090**: Thunder OAuth 2.0 / OIDC server (HTTPS)

### Files

- **Dockerfile**: Single-stage build with Thunder base and custom configuration
- **conf/deployment.yaml**: Thunder configuration file (baked into image)
- **scripts/startup.sh**: Thunder launcher
- **.dockerignore**: Build optimization

## Build & Run

### Build

```bash
docker build -t wso2/api-platform-sts:latest .
```

### Run

Basic run with default Thunder configuration:
```bash
docker run --rm -p 8090:8090 wso2/api-platform-sts:latest
```

Run with custom Thunder configuration:
```bash
docker run --rm \
  -p 8090:8090 \
  -v $(pwd)/deployment.yaml:/opt/thunder/repository/conf/deployment.yaml \
  wso2/api-platform-sts:latest
```

### Verify

```bash
curl -k https://localhost:8090/health/liveness
```

Expected response: `{"status": "UP"}`

## Key Technical Decisions

1. **Thunder Base Image**: Use official `ghcr.io/asgardeo/thunder:latest` for OAuth 2.0 / OIDC capabilities
2. **Custom Configuration**: Include `deployment.yaml` to fix CORS issue (sets `gate_client.hostname: "localhost"`)
3. **Simple Startup**: Single service (Thunder only) with straightforward startup process
4. **Health Check**: Monitor Thunder liveness to ensure container health
5. **Port 8090**: Standard Thunder HTTPS port

## Challenges & Solutions

### Challenge 1: Thunder Startup Path
**Problem**: Initial assumption that Thunder startup was at `/opt/thunder/bin/thunder.sh`
**Solution**: Verified actual path is `/opt/thunder/start.sh` from Thunder's Dockerfile
**Resolution**: Updated startup script to use correct path

### Challenge 2: Health Check Endpoint
**Problem**: Initial health check used wrong endpoint causing TLS errors
**Solution**: Changed to `/health/liveness` endpoint and added `-k` flag for self-signed certificate
**Resolution**: Health check now works reliably

## Testing

1. Build Docker image successfully
2. Start container and verify Thunder is running
3. Access health check endpoint
4. Verify Thunder APIs are accessible

## Related Features

- [Gate App Integration](./gate-app-integration.md) - Built on top of this initial setup
- [Kickstart Process](./kickstart-process.md) - Uses Thunder APIs established here

## Future Enhancements

- Thunder logging configuration
- Thunder performance tuning
- Runtime configuration override via volume mounts
