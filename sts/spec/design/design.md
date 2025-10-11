# STS Design

## Overview

STS packages [Asgardeo Thunder](https://github.com/asgardeo/thunder) OAuth server and Gate App authentication UI into a single Docker container.

## Components

- **Thunder**: OAuth 2.0 / OIDC authorization server (port 8090)
- **Gate App**: Next.js authentication UI (port 9090)

## Key Decisions

- **Single container**: Simplifies deployment and ensures version compatibility
- **Multi-stage build**: Gate App built in builder stage, then copied to Thunder runtime
- **Workspace dependencies**: Gate App requires oxygen-ui packages from Thunder monorepo
