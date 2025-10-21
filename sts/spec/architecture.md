# STS Architecture

## Overview

STS is built on [Asgardeo Thunder](https://github.com/asgardeo/thunder) OAuth 2.0 / OIDC server with an integrated Gate App authentication UI packaged in a single Docker container.

## Components

### Thunder (Port 8090)
- OAuth 2.0 / OIDC authorization server
- Token generation and management
- User and organization management
- Application registration

### Gate App (Port 9091)
- Next.js authentication UI
- Login and registration flows
- Communicates with Thunder APIs

## Container Structure

```
┌─────────────────────────────────────────┐
│         Docker Container                │
│                                         │
│  ┌──────────┐         ┌──────────┐      │
│  │ Gate App │◄───────►│ Thunder  │      │
│  │   :9091  │         │   :8090  │      │
│  └──────────┘         └──────────┘      │
│                                         │
└─────────────────────────────────────────┘
```

## Integration

- **Gate App → Thunder**: Authentication requests, user operations
- **External Apps → Thunder**: OAuth 2.0 / OIDC flows for token acquisition
