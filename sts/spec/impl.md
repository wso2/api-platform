# STS Implementation

## Overview

STS implementation creates a Docker image packaging Thunder OAuth server and Gate App authentication UI.

## Base

- **Thunder**: `ghcr.io/asgardeo/thunder:latest` - OAuth 2.0 / OIDC server
- **Reference**: [Thunder Documentation](https://github.com/asgardeo/thunder)

## Key Files

- **Dockerfile**: Multi-stage build (node:20-alpine builder + Thunder runtime)
- **scripts/startup.sh**: Launches Thunder (background) and Gate App (foreground)
- **kickstart.sh**: Automated organization/user/application setup script
- **inputs.yaml**: Configuration for kickstart script

## Requirements

- Node.js 20+
- PNPM 10+
- Docker

## Features

See detailed feature documentation:

- [Gate App Integration](impls/gate-app-integration.md)
- [Kickstart Process](impls/kickstart-process.md)
