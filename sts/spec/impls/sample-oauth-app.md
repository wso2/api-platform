# Feature: Sample OAuth Application

## Overview

Node.js Express application demonstrating automated OAuth2 authorization code flow with token exchange and JWT display.

## Git Commits

- `5966ef7` - Add sample OAuth application for testing STS integration

## Motivation

Manual OAuth testing requires copy/paste of authorization codes and curl commands for token exchange.

## Implementation Details

### Structure

**Directory**: `sts/sample-app/`

- `server.js` - HTTPS Express server
- `package.json` - Dependencies (express, axios, js-yaml)
- `server.key` / `server.cert` - SSL certificates from gate-app
- `README.md` - Quick start

### Flow

1. **Home (`GET /`)** - Redirects to STS authorization endpoint
2. **Callback (`GET /callback`)** - Exchanges code for token, displays results

### Key Technical Decisions

1. **HTTPS on port 3000** - Uses gate-app certificates
2. **Auto-redirect** - Direct OAuth initiation, no landing page
3. **Client-side JWT decode** - Demo purposes, no verification
4. **Loads configuration from registration.yaml** - No hardcoded values

## Build & Run

```bash
cd sts/sample-app
pnpm install
pnpm start
# Open https://localhost:3000
```

## Related Features

- [Kickstart Process](./kickstart-process.md) - Generates configuration
- [Gate App Integration](./gate-app-integration.md) - Provides certificates
